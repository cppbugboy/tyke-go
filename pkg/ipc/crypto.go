package ipc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
)

const (
	FrameMsgHandshakeInit byte = 0x01
	FrameMsgHandshakeResp byte = 0x02
	FrameMsgData          byte = 0x03
)

var (
	ErrBufferTooSmall    = errors.New("buffer too small for frame header")
	ErrBufferIncomplete  = errors.New("buffer incomplete")
	ErrKeyNotAvailable   = errors.New("no ECDH key available")
	ErrCipherNotInit     = errors.New("cipher not initialized")
	ErrCiphertextTooShort = errors.New("ciphertext too short")
)

type FrameParser struct{}

func (FrameParser) BuildFrame(frameType byte, payload []byte) []byte {
	totalLen := uint32(1 + len(payload))
	frame := make([]byte, 0, 4+totalLen)
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, totalLen)
	frame = append(frame, buf...)
	frame = append(frame, frameType)
	frame = append(frame, payload...)
	return frame
}

func (FrameParser) ExtractFrame(buffer *[]byte) (frameType byte, payload []byte, err error) {
	buf := *buffer
	if len(buf) < 5 {
		return 0, nil, ErrBufferTooSmall
	}
	totalLen := binary.BigEndian.Uint32(buf[:4])
	if uint32(len(buf)) < 4+totalLen {
		return 0, nil, fmt.Errorf("%w: expected %d bytes, got %d", ErrBufferIncomplete, 4+totalLen, len(buf))
	}
	frameType = buf[4]
	payload = make([]byte, totalLen-1)
	copy(payload, buf[5:4+totalLen])
	*buffer = buf[4+totalLen:]
	return frameType, payload, nil
}

type EcdhKeyExchange struct {
	privateKey *ecdh.PrivateKey
}

func NewEcdhKeyExchange() *EcdhKeyExchange {
	return &EcdhKeyExchange{}
}

func (e *EcdhKeyExchange) GenerateKey() error {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("ECDH key generation failed: %w", err)
	}
	e.privateKey = key
	return nil
}

func (e *EcdhKeyExchange) GetPublicKeyDer() ([]byte, error) {
	if e.privateKey == nil {
		return nil, ErrKeyNotAvailable
	}
	der, err := x509.MarshalPKIXPublicKey(e.privateKey.PublicKey())
	if err != nil {
		return nil, fmt.Errorf("failed to export public key DER: %w", err)
	}
	return der, nil
}

func (e *EcdhKeyExchange) ComputeSharedSecret(peerPubDer []byte) ([]byte, error) {
	if e.privateKey == nil {
		return nil, ErrKeyNotAvailable
	}
	ecdhPub, err := parsePublicKeyFromDer(peerPubDer)
	if err != nil {
		return nil, fmt.Errorf("failed to parse peer public key DER: %w", err)
	}
	secret, err := e.privateKey.ECDH(ecdhPub)
	if err != nil {
		return nil, fmt.Errorf("failed to compute shared secret: %w", err)
	}
	return secret, nil
}

func parsePublicKeyFromDer(der []byte) (*ecdh.PublicKey, error) {
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, err
	}
	switch k := pub.(type) {
	case *ecdh.PublicKey:
		return k, nil
	case *ecdsa.PublicKey:
		if k.Curve != elliptic.P256() {
			return nil, errors.New("unsupported EC curve")
		}
		raw := elliptic.Marshal(k.Curve, k.X, k.Y)
		return ecdh.P256().NewPublicKey(raw)
	default:
		return nil, errors.New("unsupported public key type")
	}
}

func bigIntToBytes(n *big.Int, size int) []byte {
	b := n.Bytes()
	if len(b) < size {
		padded := make([]byte, size)
		copy(padded[size-len(b):], b)
		return padded
	}
	return b
}

type AesGcmCipher struct {
	key         [32]byte
	initialized bool
}

func NewAesGcmCipher() *AesGcmCipher {
	return &AesGcmCipher{}
}

func (c *AesGcmCipher) Init(sharedSecret []byte) error {
	if len(sharedSecret) == 0 {
		return errors.New("shared secret is empty")
	}
	hash := sha256.Sum256(sharedSecret)
	c.key = hash
	c.initialized = true
	return nil
}

func (c *AesGcmCipher) IsInitialized() bool {
	return c.initialized
}

func (c *AesGcmCipher) Encrypt(plaintext []byte) ([]byte, error) {
	if !c.initialized {
		return nil, ErrCipherNotInit
	}
	block, err := aes.NewCipher(c.key[:])
	if err != nil {
		return nil, fmt.Errorf("AES cipher creation failed: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("AES-GCM creation failed: %w", err)
	}
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("RAND_bytes for nonce failed: %w", err)
	}
	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)
	result := make([]byte, 0, len(nonce)+len(ciphertext))
	result = append(result, nonce...)
	result = append(result, ciphertext...)
	return result, nil
}

func (c *AesGcmCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	if !c.initialized {
		return nil, ErrCipherNotInit
	}
	nonceSize := 12
	tagSize := 16
	if len(ciphertext) < nonceSize+tagSize {
		return nil, ErrCiphertextTooShort
	}
	block, err := aes.NewCipher(c.key[:])
	if err != nil {
		return nil, fmt.Errorf("AES cipher creation failed: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("AES-GCM creation failed: %w", err)
	}
	nonce := ciphertext[:nonceSize]
	encryptedData := ciphertext[nonceSize:]
	plaintext, err := aesgcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("AES-GCM decrypt failed: %w", err)
	}
	return plaintext, nil
}
