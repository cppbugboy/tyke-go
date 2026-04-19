package ipc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"fmt"

	"github.com/tyke/tyke/tyke/common"
)

const (
	MsgHandshakeInit byte = 0x01
	MsgHandshakeResp byte = 0x02
	MsgData          byte = 0x03
)

func encodeU32(val uint32, out *[]byte) {
	*out = append(*out, byte((val>>24)&0xFF), byte((val>>16)&0xFF), byte((val>>8)&0xFF), byte(val&0xFF))
}

func decodeU32(data []byte) uint32 {
	return uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
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

type EcdhKeyExchange struct {
	privateKey *ecdh.PrivateKey
}

func NewEcdhKeyExchange() *EcdhKeyExchange {
	return &EcdhKeyExchange{}
}

func (e *EcdhKeyExchange) GenerateKey() common.BoolResult {
	privateKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		common.LogError("ECDH key generation failed", "error", err)
		return common.ErrBool("ECDH key generation failed")
	}
	e.privateKey = privateKey
	common.LogDebug("ECDH key generated successfully")
	return common.OkBool(true)
}

func (e *EcdhKeyExchange) GetPublicKeyDer() common.ByteVecResult {
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

func (e *EcdhKeyExchange) ComputeSharedSecret(peerPubDer []byte) common.ByteVecResult {
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

type AesGcmCipher struct {
	aesKey      []byte
	initialized bool
}

func NewAesGcmCipher() *AesGcmCipher {
	return &AesGcmCipher{}
}

func (c *AesGcmCipher) Init(sharedSecret []byte) common.BoolResult {
	if len(sharedSecret) == 0 {
		return common.ErrBool("shared secret is empty")
	}
	hash := sha256.Sum256(sharedSecret)
	c.aesKey = hash[:]
	c.initialized = true
	common.LogDebug("AES-GCM cipher initialized")
	return common.OkBool(true)
}

func (c *AesGcmCipher) IsInitialized() bool {
	return c.initialized
}

func (c *AesGcmCipher) Encrypt(plaintext []byte) common.ByteVecResult {
	if !c.initialized {
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
	iv := make([]byte, aesGcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		common.LogError("IV generation failed", "error", err)
		return common.ErrByteVec("IV generation failed")
	}
	ciphertext := aesGcm.Seal(nil, iv, plaintext, nil)
	result := make([]byte, 0, len(iv)+len(ciphertext))
	result = append(result, iv...)
	result = append(result, ciphertext...)
	return common.OkByteVec(result)
}

func (c *AesGcmCipher) Decrypt(ciphertext []byte) common.ByteVecResult {
	if !c.initialized {
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
