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
	"runtime"
	"sync"
	"sync/atomic"

	"tyke-go/common"
)

const (
	MsgHandshakeInit byte = 0x01
	MsgHandshakeResp byte = 0x02
	MsgData          byte = 0x03
	MsgDataFragment  byte = 0x04

	MaxFramePayloadLen uint32 = 16 * 1024 * 1024
	FragmentChunkSize  uint32 = 64 * 1024
	FragmentHeaderSize uint32 = 8
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
	if totalLen < 5 {
		*buffer = nil
		return 0, nil, fmt.Errorf("invalid frame: total_len too small: %d < 5", totalLen)
	}
	if totalLen > MaxFramePayloadLen+1 {
		*buffer = nil
		return 0, nil, fmt.Errorf("frame payload too large: %d > %d", totalLen, MaxFramePayloadLen+1)
	}
	if uint32(len(*buffer)) < 4+totalLen {
		return 0, nil, fmt.Errorf("buffer incomplete: expected %d bytes, got %d", 4+totalLen, len(*buffer))
	}
	frameType := (*buffer)[4]
	payload := make([]byte, totalLen-1)
	copy(payload, (*buffer)[5:4+totalLen])
	*buffer = (*buffer)[4+totalLen:]
	return frameType, payload, nil
}

type ECDHKeyExchange struct {
	privateKey *ecdh.PrivateKey
}

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
		if k.Curve.Params().Name != "P-256" {
			common.LogError("Peer ECDSA key is not P-256 curve", "curve", k.Curve.Params().Name)
			return common.ErrByteVec("peer ECDSA key is not P-256 curve: " + k.Curve.Params().Name)
		}
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

type AESGCMCipher struct {
	mu          sync.RWMutex
	aesKey      []byte
	aesGcm      cipher.AEAD
	initialized atomic.Bool
	ivCounter   atomic.Uint64
}

func NewAESGCMCipher() *AESGCMCipher {
	return &AESGCMCipher{}
}

func hkdfExpand(hashFunc func() hash.Hash, prk []byte, info []byte, length int) ([]byte, error) {
	if length > 255*hashFunc().Size() {
		return nil, fmt.Errorf("HKDF expand requested length too large: %d", length)
	}

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

	salt := []byte("tyke-v1-hkdf-salt")
	info := []byte("tyke-v1-aes256-key")

	key, err := hkdfDeriveKey(sha256.New, salt, sharedSecret, info, common.Aes256KeyLen)
	if err != nil {
		common.LogError("HKDF derive key failed", "error", err)
		return common.ErrBool("HKDF derive key failed")
	}

	block, blockErr := aes.NewCipher(key)
	if blockErr != nil {
		common.LogError("AES cipher creation failed", "error", blockErr)
		return common.ErrBool("AES cipher creation failed")
	}
	gcm, gcmErr := cipher.NewGCM(block)
	if gcmErr != nil {
		common.LogError("GCM creation failed", "error", gcmErr)
		return common.ErrBool("GCM creation failed")
	}

	c.mu.Lock()
	c.aesKey = key
	c.aesGcm = gcm
	c.initialized.Store(true)
	c.mu.Unlock()

	common.LogDebug("AES-GCM cipher initialized with HKDF")
	return common.OkBool(true)
}

func (c *AESGCMCipher) IsInitialized() bool {
	return c.initialized.Load()
}

func (c *AESGCMCipher) ClearKey() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.aesKey != nil {
		for i := range c.aesKey {
			c.aesKey[i] = 0
		}
		runtime.KeepAlive(c.aesKey)
		c.aesKey = nil
	}
	c.aesGcm = nil
	c.initialized.Store(false)
}

func (c *AESGCMCipher) Encrypt(plaintext []byte) common.ByteVecResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.initialized.Load() || c.aesGcm == nil {
		return common.ErrByteVec("cipher not initialized")
	}

	iv := make([]byte, common.AesGcmIvLen)
	if _, err := rand.Read(iv[:4]); err != nil {
		common.LogError("IV prefix generation failed", "error", err)
		return common.ErrByteVec("IV prefix generation failed")
	}
	counter := c.ivCounter.Add(1)
	if counter == 0 {
		common.LogError("AES-GCM IV counter overflow: key must be rotated")
		return common.ErrByteVec("IV counter overflow: key must be rotated")
	}
	binary.BigEndian.PutUint64(iv[4:], counter)

	ciphertext := c.aesGcm.Seal(nil, iv, plaintext, nil)
	result := make([]byte, 0, common.AesGcmIvLen+len(ciphertext))
	result = append(result, iv...)
	result = append(result, ciphertext...)
	return common.OkByteVec(result)
}

func (c *AESGCMCipher) Decrypt(ciphertext []byte) common.ByteVecResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.initialized.Load() || c.aesGcm == nil {
		return common.ErrByteVec("cipher not initialized")
	}
	if len(ciphertext) < common.AesGcmIvLen+common.AesGcmTagLen {
		return common.ErrByteVec("ciphertext too short")
	}

	ivSize := c.aesGcm.NonceSize()
	iv := ciphertext[:ivSize]
	encData := ciphertext[ivSize:]
	plaintext, err := c.aesGcm.Open(nil, iv, encData, nil)
	if err != nil {
		common.LogError("AES-GCM decrypt failed", "error", err)
		return common.ErrByteVec("AES-GCM decrypt final failed: authentication tag mismatch")
	}
	return common.OkByteVec(plaintext)
}

type FragmentReassembly struct {
	Buffer     []byte
	Total      uint32
	Received   uint32
	NextOffset uint32
}

func (r *FragmentReassembly) Reset(totalSize uint32) {
	r.Buffer = make([]byte, totalSize)
	r.Total = totalSize
	r.Received = 0
	r.NextOffset = 0
}

func (r *FragmentReassembly) IsComplete() bool {
	return r.Received == r.Total && r.Total > 0
}

func (r *FragmentReassembly) ValidateOffset(offset uint32, chunkLen int) bool {
	if offset != r.NextOffset {
		return false
	}
	if int(offset)+chunkLen > int(r.Total) {
		return false
	}
	return true
}

func BuildFragmentPayload(totalSize uint32, offset uint32, encryptedChunk []byte) []byte {
	var payload []byte
	encodeU32(totalSize, &payload)
	encodeU32(offset, &payload)
	payload = append(payload, encryptedChunk...)
	return payload
}

func ParseFragmentHeader(payload []byte) (totalSize uint32, offset uint32, encryptedChunk []byte, err error) {
	if uint32(len(payload)) < FragmentHeaderSize {
		return 0, 0, nil, fmt.Errorf("fragment payload too small: %d < %d", len(payload), FragmentHeaderSize)
	}
	totalSize = decodeU32(payload[0:4])
	offset = decodeU32(payload[4:8])
	encryptedChunk = payload[FragmentHeaderSize:]
	return totalSize, offset, encryptedChunk, nil
}
