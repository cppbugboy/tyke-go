package ipc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"hash"
	"sync/atomic"

	"github.com/tyke/tyke/tyke/common"
)

const (
	MsgHandshakeInit byte = 0x01
	MsgHandshakeResp byte = 0x02
	MsgData          byte = 0x03
)

func encodeU32(val uint32, out *[]byte) {
	*out = append(*out, byte(val&0xFF), byte((val>>8)&0xFF), byte((val>>16)&0xFF), byte((val>>24)&0xFF))
}

func decodeU32(data []byte) uint32 {
	return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
}

func BuildFrame(frameType byte, payload []byte) []byte {
	var frame []byte
	totalLen := uint32(1 + len(payload))
	encodeU32(totalLen, &frame)
	frame = append(frame, frameType)
	frame = append(frame, payload...)
	return frame
}

func ExtractFrame(buffer *[]byte) (byte, []byte, error) {
	if len(*buffer) < 5 {
		return 0, nil, fmt.Errorf("buffer too small for frame header")
	}
	totalLen := decodeU32(*buffer)
	if uint32(len(*buffer)) < 4+totalLen {
		return 0, nil, fmt.Errorf("buffer incomplete: expected %d bytes, got %d", 4+totalLen, len(*buffer))
	}
	frameType := (*buffer)[4]
	payload := make([]byte, totalLen-1)
	copy(payload, (*buffer)[5:4+totalLen])
	*buffer = (*buffer)[4+totalLen:]
	return frameType, payload, nil
}

// ECDHKeyExchange 实现 ECDH 密钥交换算法，用于建立共享密钥。
type ECDHKeyExchange struct {
	privateKey *ecdh.PrivateKey
}

// NewECDHKeyExchange 创建一个新的 ECDHKeyExchange 实例。
func NewECDHKeyExchange() *ECDHKeyExchange {
	return &ECDHKeyExchange{}
}

func (e *ECDHKeyExchange) GenerateKey() common.BoolResult {
	privateKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		common.LogError("ECDH key generation failed", "error", err)
		return common.ErrBool("ECDH key generation failed")
	}
	e.privateKey = privateKey
	common.LogDebug("ECDH key generated successfully")
	return common.OkBool(true)
}

func (e *ECDHKeyExchange) GetPublicKeyDer() common.ByteVecResult {
	if e.privateKey == nil {
		return common.ErrByteVec("no ECDH key available")
	}
	derBytes, err := x509.MarshalPKIXPublicKey(e.privateKey.PublicKey())
	if err != nil {
		common.LogError("Failed to marshal public key to SPKI DER", "error", err)
		return common.ErrByteVec("failed to marshal public key: " + err.Error())
	}
	common.LogDebug("Public key exported as SPKI DER", "length", len(derBytes))
	return common.OkByteVec(derBytes)
}

func (e *ECDHKeyExchange) ComputeSharedSecret(peerPubDer []byte) common.ByteVecResult {
	if e.privateKey == nil {
		return common.ErrByteVec("no ECDH key available")
	}
	pub, err := x509.ParsePKIXPublicKey(peerPubDer)
	if err != nil {
		common.LogError("Failed to parse peer public key SPKI DER", "error", err)
		return common.ErrByteVec("failed to parse peer public key DER: " + err.Error())
	}
	var ecdhPub *ecdh.PublicKey
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		ecdhPub, err = k.ECDH()
		if err != nil {
			common.LogError("Failed to convert ECDSA public key to ECDH", "error", err)
			return common.ErrByteVec("failed to convert ECDSA public key to ECDH: " + err.Error())
		}
	case *ecdh.PublicKey:
		ecdhPub = k
	default:
		common.LogError("Peer public key is not ECDSA/ECDH", "type", fmt.Sprintf("%T", pub))
		return common.ErrByteVec("peer public key is not ECDSA/ECDH")
	}
	secret, err := e.privateKey.ECDH(ecdhPub)
	if err != nil {
		common.LogError("Failed to compute shared secret", "error", err)
		return common.ErrByteVec("failed to compute shared secret")
	}
	common.LogDebug("Shared secret computed successfully", "length", len(secret))
	return common.OkByteVec(secret)
}

// AESGCMCipher 实现 AES-GCM 加密算法，用于数据加密和解密。
type AESGCMCipher struct {
	aesKey      []byte
	initialized atomic.Bool
	ivCounter   atomic.Uint64
}

func NewAESGCMCipher() *AESGCMCipher {
	return &AESGCMCipher{}
}

func hkdfExpand(hashFunc func() hash.Hash, prk []byte, info []byte, length int) ([]byte, error) {
	output := make([]byte, 0, length)
	var counter byte = 1
	var prev []byte

	for len(output) < length {
		h := hmac.New(hashFunc, prk)
		h.Write(prev)
		h.Write(info)
		h.Write([]byte{counter})
		t := h.Sum(nil)
		remaining := length - len(output)
		if remaining > len(t) {
			remaining = len(t)
		}
		output = append(output, t[:remaining]...)
		prev = t
		counter++
		if counter == 0 {
			return nil, fmt.Errorf("HKDF expand counter overflow")
		}
	}
	return output, nil
}

func hkdfExtract(hashFunc func() hash.Hash, salt []byte, ikm []byte) []byte {
	if salt == nil || len(salt) == 0 {
		salt = make([]byte, hashFunc().Size())
	}
	h := hmac.New(hashFunc, salt)
	h.Write(ikm)
	return h.Sum(nil)
}

func hkdfDeriveKey(hashFunc func() hash.Hash, salt []byte, ikm []byte, info []byte, length int) ([]byte, error) {
	prk := hkdfExtract(hashFunc, salt, ikm)
	return hkdfExpand(hashFunc, prk, info, length)
}

func (c *AESGCMCipher) Init(sharedSecret []byte) common.BoolResult {
	if len(sharedSecret) == 0 {
		return common.ErrBool("shared secret is empty")
	}

	salt := []byte("tyke-hkdf-salt")
	info := []byte("tyke-aes256-key")

	key, err := hkdfDeriveKey(sha256.New, salt, sharedSecret, info, common.Aes256KeyLen)
	if err != nil {
		common.LogError("HKDF derive key failed", "error", err)
		return common.ErrBool("HKDF derive key failed")
	}

	c.aesKey = key
	c.initialized.Store(true)
	common.LogDebug("AES-GCM cipher initialized with HKDF")
	return common.OkBool(true)
}

func (c *AESGCMCipher) IsInitialized() bool {
	return c.initialized.Load()
}

func (c *AESGCMCipher) Encrypt(plaintext []byte) common.ByteVecResult {
	if !c.initialized.Load() {
		return common.ErrByteVec("cipher not initialized")
	}
	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		common.LogError("AES cipher creation failed", "error", err)
		return common.ErrByteVec("AES cipher creation failed")
	}
	aesGcm, err := cipher.NewGCM(block)
	if err != nil {
		common.LogError("GCM creation failed", "error", err)
		return common.ErrByteVec("GCM creation failed")
	}

	iv := make([]byte, common.AesGcmIvLen)
	counter := c.ivCounter.Add(1)
	binary.BigEndian.PutUint64(iv[4:], counter)
	if _, err := rand.Read(iv[:4]); err != nil {
		common.LogError("IV prefix generation failed", "error", err)
		return common.ErrByteVec("IV prefix generation failed")
	}

	ciphertext := aesGcm.Seal(nil, iv, plaintext, nil)
	result := make([]byte, 0, common.AesGcmIvLen+len(ciphertext))
	result = append(result, iv...)
	result = append(result, ciphertext...)
	return common.OkByteVec(result)
}

func (c *AESGCMCipher) Decrypt(ciphertext []byte) common.ByteVecResult {
	if !c.initialized.Load() {
		return common.ErrByteVec("cipher not initialized")
	}
	if len(ciphertext) < common.AesGcmIvLen+common.AesGcmTagLen {
		return common.ErrByteVec("ciphertext too short")
	}
	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		common.LogError("AES cipher creation failed", "error", err)
		return common.ErrByteVec("AES cipher creation failed")
	}
	aesGcm, err := cipher.NewGCM(block)
	if err != nil {
		common.LogError("GCM creation failed", "error", err)
		return common.ErrByteVec("GCM creation failed")
	}
	ivSize := aesGcm.NonceSize()
	iv := ciphertext[:ivSize]
	encData := ciphertext[ivSize:]
	plaintext, err := aesGcm.Open(nil, iv, encData, nil)
	if err != nil {
		common.LogError("AES-GCM decrypt failed", "error", err)
		return common.ErrByteVec("AES-GCM decrypt final failed: authentication tag mismatch")
	}
	return common.OkByteVec(plaintext)
}
