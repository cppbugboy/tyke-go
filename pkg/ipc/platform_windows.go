//go:build windows

package ipc

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/tyke/tyke/pkg/component"
)

type clientConnectionImpl struct {
	conn   net.Conn
	cipher *AesGcmCipher
}

func newClientConnectionImpl() *clientConnectionImpl {
	return &clientConnectionImpl{
		cipher: NewAesGcmCipher(),
	}
}

func (c *clientConnectionImpl) Connect(serverName string, timeoutMs uint32, rwTimeoutMs uint32) error {
	pipePath := fmt.Sprintf(`\\.\pipe\%s`, serverName)
	timeout := time.Duration(timeoutMs) * time.Millisecond

	conn, err := winio.DialPipe(pipePath, &timeout)
	if err != nil {
		return fmt.Errorf("connect failed for: %s: %w", serverName, err)
	}
	c.conn = conn

	if rwTimeoutMs > 0 {
		deadline := time.Now().Add(time.Duration(rwTimeoutMs) * time.Millisecond)
		c.conn.SetDeadline(deadline)
	}

	return c.doHandshake(timeoutMs)
}

func (c *clientConnectionImpl) doHandshake(timeoutMs uint32) error {
	ecdh := NewEcdhKeyExchange()
	if err := ecdh.GenerateKey(); err != nil {
		return fmt.Errorf("handshake: key generation failed: %w", err)
	}

	pubDer, err := ecdh.GetPublicKeyDer()
	if err != nil {
		return fmt.Errorf("handshake: get public key failed: %w", err)
	}

	fp := FrameParser{}
	initFrame := fp.BuildFrame(FrameMsgHandshakeInit, pubDer)
	if _, err := c.conn.Write(initFrame); err != nil {
		return fmt.Errorf("handshake: write init frame failed: %w", err)
	}

	buf := make([]byte, 4096)
	n, err := c.conn.Read(buf)
	if err != nil {
		return fmt.Errorf("handshake: read failed: %w", err)
	}

	recvBuf := buf[:n]
	frameType, payload, err := fp.ExtractFrame(&recvBuf)
	if err != nil {
		return fmt.Errorf("handshake: extract frame failed: %w", err)
	}
	if frameType != FrameMsgHandshakeResp {
		return fmt.Errorf("handshake: unexpected frame type %d", frameType)
	}

	secret, err := ecdh.ComputeSharedSecret(payload)
	if err != nil {
		return fmt.Errorf("handshake: compute shared secret failed: %w", err)
	}

	if err := c.cipher.Init(secret); err != nil {
		return fmt.Errorf("handshake: cipher init failed: %w", err)
	}

	return nil
}

func (c *clientConnectionImpl) WriteEncrypted(data []byte, timeoutMs uint32) error {
	encrypted, err := c.cipher.Encrypt(data)
	if err != nil {
		return fmt.Errorf("encrypt failed: %w", err)
	}
	fp := FrameParser{}
	frame := fp.BuildFrame(FrameMsgData, encrypted)
	if timeoutMs > 0 {
		c.conn.SetDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
	}
	_, err = c.conn.Write(frame)
	return err
}

func (c *clientConnectionImpl) ReadLoop(callback ClientRecvDataCallback, timeoutMs uint32) error {
	fp := FrameParser{}
	rawBuf := make([]byte, 0, 4096)
	chunk := make([]byte, 4096)

	for {
		if timeoutMs > 0 {
			c.conn.SetDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
		}
		n, err := c.conn.Read(chunk)
		if err != nil {
			return fmt.Errorf("read loop: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("read loop: connection closed")
		}

		rawBuf = append(rawBuf, chunk[:n]...)
		var plainBuf []byte

		for {
			frameType, payload, extractErr := fp.ExtractFrame(&rawBuf)
			if extractErr != nil {
				break
			}
			if frameType == FrameMsgData {
				decrypted, decErr := c.cipher.Decrypt(payload)
				if decErr != nil {
					return fmt.Errorf("read loop decrypt failed: %w", decErr)
				}
				plainBuf = append(plainBuf, decrypted...)
			}
		}

		if len(plainBuf) > 0 {
			if callback(plainBuf) {
				return nil
			}
		}
	}
}

func (c *clientConnectionImpl) Close() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *clientConnectionImpl) IsValid() bool {
	return c.conn != nil && c.cipher.IsInitialized()
}

type serverImpl struct {
	listener net.Listener
	running  bool
	mu       sync.Mutex
	clients  map[ClientId]*serverClient
	callback ServerRecvDataCallback
	nextId   ClientId
}

type serverClient struct {
	id     ClientId
	conn   net.Conn
	cipher *AesGcmCipher
}

func newServerImpl() *serverImpl {
	return &serverImpl{
		clients: make(map[ClientId]*serverClient),
	}
}

func (s *serverImpl) Start(serverName string, callback ServerRecvDataCallback) error {
	pipePath := fmt.Sprintf(`\\.\pipe\%s`, serverName)
	cfg := &winio.PipeConfig{
		SecurityDescriptor: "",
		MessageMode:        false,
		InputBufferSize:    4096,
		OutputBufferSize:   4096,
	}

	listener, err := winio.ListenPipe(pipePath, cfg)
	if err != nil {
		return fmt.Errorf("failed to create named pipe: %w", err)
	}

	s.listener = listener
	s.callback = callback
	s.running = true

	go s.acceptLoop()
	return nil
}

func (s *serverImpl) acceptLoop() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if !s.running {
				return
			}
			continue
		}

		client := &serverClient{
			id:     ClientId(uint64(len(s.clients) + 1)),
			conn:   conn,
			cipher: NewAesGcmCipher(),
		}

		s.mu.Lock()
		s.clients[client.id] = client
		s.mu.Unlock()

		go s.handleClient(client)
	}
}

func (s *serverImpl) handleClient(client *serverClient) {
	defer func() {
		client.conn.Close()
		s.mu.Lock()
		delete(s.clients, client.id)
		s.mu.Unlock()
	}()

	fp := FrameParser{}
	buf := make([]byte, 0, 4096)
	chunk := make([]byte, 4096)
	handshakeDone := false

	for s.running {
		n, err := client.conn.Read(chunk)
		if err != nil {
			return
		}
		if n == 0 {
			return
		}

		buf = append(buf, chunk[:n]...)

		for {
			frameType, payload, extractErr := fp.ExtractFrame(&buf)
			if extractErr != nil {
				break
			}

			if !handshakeDone {
				if frameType != FrameMsgHandshakeInit {
					return
				}
				ecdh := NewEcdhKeyExchange()
				if err := ecdh.GenerateKey(); err != nil {
					return
				}
				secret, err := ecdh.ComputeSharedSecret(payload)
				if err != nil {
					return
				}
				if err := client.cipher.Init(secret); err != nil {
					return
				}
				pubDer, err := ecdh.GetPublicKeyDer()
				if err != nil {
					return
				}
				respFrame := fp.BuildFrame(FrameMsgHandshakeResp, pubDer)
				if _, err := client.conn.Write(respFrame); err != nil {
					return
				}
				handshakeDone = true
			} else {
				if frameType != FrameMsgData {
					return
				}
				decrypted, err := client.cipher.Decrypt(payload)
				if err != nil {
					return
				}

				if s.callback != nil {
					dataCopy := make([]byte, len(decrypted))
					copy(dataCopy, decrypted)
					clientID := client.id
					callback := s.callback

					wp := component.GetWorkerPool()
					if wp != nil {
						wp.Submit(func() {
							sendCb := func(id ClientId, data []byte) bool {
								return s.SendToClient(id, data) == nil
							}
							callback(clientID, dataCopy, sendCb)
						})
					}
				}
			}
		}
	}
}

func (s *serverImpl) Stop() {
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}
	s.mu.Lock()
	for _, client := range s.clients {
		client.conn.Close()
	}
	s.clients = make(map[ClientId]*serverClient)
	s.mu.Unlock()
}

func (s *serverImpl) SendToClient(id ClientId, data []byte) error {
	s.mu.Lock()
	client, ok := s.clients[id]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("client not found: %d", id)
	}

	encrypted, err := client.cipher.Encrypt(data)
	if err != nil {
		return fmt.Errorf("encrypt failed for client %d: %w", id, err)
	}

	fp := FrameParser{}
	frame := fp.BuildFrame(FrameMsgData, encrypted)
	_, err = client.conn.Write(frame)
	return err
}
