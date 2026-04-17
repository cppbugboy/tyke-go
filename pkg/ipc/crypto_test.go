package ipc

import (
	"bytes"
	"testing"
)

func TestFrameParserBuildAndExtract(t *testing.T) {
	fp := FrameParser{}
	payload := []byte("hello world")
	frame := fp.BuildFrame(FrameMsgData, payload)

	if len(frame) != 4+1+len(payload) {
		t.Errorf("frame length = %d, want %d", len(frame), 4+1+len(payload))
	}

	buf := make([]byte, len(frame))
	copy(buf, frame)

	frameType, extractedPayload, err := fp.ExtractFrame(&buf)
	if err != nil {
		t.Fatalf("ExtractFrame failed: %v", err)
	}
	if frameType != FrameMsgData {
		t.Errorf("frameType = %d, want %d", frameType, FrameMsgData)
	}
	if !bytes.Equal(extractedPayload, payload) {
		t.Errorf("payload mismatch: got %v, want %v", extractedPayload, payload)
	}
	if len(buf) != 0 {
		t.Errorf("buffer not fully consumed, remaining = %d bytes", len(buf))
	}
}

func TestFrameParserExtractTooSmall(t *testing.T) {
	fp := FrameParser{}
	buf := []byte{0x00, 0x01, 0x02}
	_, _, err := fp.ExtractFrame(&buf)
	if err != ErrBufferTooSmall {
		t.Errorf("expected ErrBufferTooSmall, got %v", err)
	}
}

func TestFrameParserExtractIncomplete(t *testing.T) {
	fp := FrameParser{}
	frame := fp.BuildFrame(FrameMsgData, []byte("hello"))
	buf := frame[:6]
	_, _, err := fp.ExtractFrame(&buf)
	if err == nil {
		t.Error("expected error for incomplete buffer")
	}
}

func TestFrameParserMultipleFrames(t *testing.T) {
	fp := FrameParser{}
	frame1 := fp.BuildFrame(FrameMsgHandshakeInit, []byte("init"))
	frame2 := fp.BuildFrame(FrameMsgHandshakeResp, []byte("resp"))

	buf := make([]byte, 0, len(frame1)+len(frame2))
	buf = append(buf, frame1...)
	buf = append(buf, frame2...)

	ft1, p1, err := fp.ExtractFrame(&buf)
	if err != nil {
		t.Fatalf("ExtractFrame 1 failed: %v", err)
	}
	if ft1 != FrameMsgHandshakeInit || string(p1) != "init" {
		t.Errorf("frame 1: type=%d payload=%v", ft1, p1)
	}

	ft2, p2, err := fp.ExtractFrame(&buf)
	if err != nil {
		t.Fatalf("ExtractFrame 2 failed: %v", err)
	}
	if ft2 != FrameMsgHandshakeResp || string(p2) != "resp" {
		t.Errorf("frame 2: type=%d payload=%v", ft2, p2)
	}
}

func TestEcdhKeyExchange(t *testing.T) {
	alice := NewEcdhKeyExchange()
	bob := NewEcdhKeyExchange()

	if err := alice.GenerateKey(); err != nil {
		t.Fatalf("Alice GenerateKey failed: %v", err)
	}
	if err := bob.GenerateKey(); err != nil {
		t.Fatalf("Bob GenerateKey failed: %v", err)
	}

	alicePub, err := alice.GetPublicKeyDer()
	if err != nil {
		t.Fatalf("Alice GetPublicKeyDer failed: %v", err)
	}
	bobPub, err := bob.GetPublicKeyDer()
	if err != nil {
		t.Fatalf("Bob GetPublicKeyDer failed: %v", err)
	}

	aliceSecret, err := alice.ComputeSharedSecret(bobPub)
	if err != nil {
		t.Fatalf("Alice ComputeSharedSecret failed: %v", err)
	}
	bobSecret, err := bob.ComputeSharedSecret(alicePub)
	if err != nil {
		t.Fatalf("Bob ComputeSharedSecret failed: %v", err)
	}

	if !bytes.Equal(aliceSecret, bobSecret) {
		t.Error("shared secrets do not match")
	}
}

func TestAesGcmCipher(t *testing.T) {
	ecdh1 := NewEcdhKeyExchange()
	ecdh2 := NewEcdhKeyExchange()

	if err := ecdh1.GenerateKey(); err != nil {
		t.Fatalf("GenerateKey 1 failed: %v", err)
	}
	if err := ecdh2.GenerateKey(); err != nil {
		t.Fatalf("GenerateKey 2 failed: %v", err)
	}

	pub1, _ := ecdh1.GetPublicKeyDer()
	secret, _ := ecdh1.ComputeSharedSecret(pub1)

	cipher1 := NewAesGcmCipher()
	if err := cipher1.Init(secret); err != nil {
		t.Fatalf("Cipher Init failed: %v", err)
	}
	if !cipher1.IsInitialized() {
		t.Error("Cipher should be initialized")
	}

	plaintext := []byte("hello tyke framework")
	encrypted, err := cipher1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if len(encrypted) <= len(plaintext) {
		t.Error("encrypted data should be longer than plaintext due to nonce and tag")
	}

	decrypted, err := cipher1.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted mismatch: got %v, want %v", decrypted, plaintext)
	}
}

func TestAesGcmCipherNotInitialized(t *testing.T) {
	c := NewAesGcmCipher()
	_, err := c.Encrypt([]byte("test"))
	if err != ErrCipherNotInit {
		t.Errorf("expected ErrCipherNotInit, got %v", err)
	}
	_, err = c.Decrypt([]byte("test"))
	if err != ErrCipherNotInit {
		t.Errorf("expected ErrCipherNotInit, got %v", err)
	}
}

func TestFullHandshakeAndEncrypt(t *testing.T) {
	clientEcdh := NewEcdhKeyExchange()
	serverEcdh := NewEcdhKeyExchange()

	if err := clientEcdh.GenerateKey(); err != nil {
		t.Fatalf("client GenerateKey failed: %v", err)
	}
	if err := serverEcdh.GenerateKey(); err != nil {
		t.Fatalf("server GenerateKey failed: %v", err)
	}

	clientPub, _ := clientEcdh.GetPublicKeyDer()
	serverPub, _ := serverEcdh.GetPublicKeyDer()

	clientSecret, _ := clientEcdh.ComputeSharedSecret(serverPub)
	serverSecret, _ := serverEcdh.ComputeSharedSecret(clientPub)

	if !bytes.Equal(clientSecret, serverSecret) {
		t.Fatal("shared secrets do not match after handshake")
	}

	clientCipher := NewAesGcmCipher()
	clientCipher.Init(clientSecret)
	serverCipher := NewAesGcmCipher()
	serverCipher.Init(serverSecret)

	msg := []byte("secret message from client")
	encrypted, err := clientCipher.Encrypt(msg)
	if err != nil {
		t.Fatalf("client Encrypt failed: %v", err)
	}

	decrypted, err := serverCipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("server Decrypt failed: %v", err)
	}
	if !bytes.Equal(decrypted, msg) {
		t.Errorf("decrypted mismatch: got %q, want %q", decrypted, msg)
	}
}
