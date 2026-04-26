//go:build linux

package ipc

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cppbugboy/tyke-go/tyke/common"
	"github.com/cppbugboy/tyke-go/tyke/component"
)

type clientConnectionImplLinux struct {
	conn       net.Conn
	cipher     *AESGCMCipher
	reassembly FragmentReassembly
}

func newClientConnectionImplLinux() ClientConnection {
	return &clientConnectionImplLinux{cipher: NewAESGCMCipher()}
}

func (c *clientConnectionImplLinux) Connect(serverName string, timeoutMs uint32) common.BoolResult {
	common.LogInfo("IPC client connecting", "server_name", serverName)
	addr := &net.UnixAddr{Name: "\x00tyke_" + serverName, Net: "unix"}
	dialer := net.Dialer{Timeout: time.Duration(timeoutMs) * time.Millisecond}
	conn, err := dialer.Dial("unix", addr.Name)
	if err != nil {
		return common.ErrBool("connect failed: " + err.Error())
	}
	c.conn = conn
	return c.doHandshake()
}

func (c *clientConnectionImplLinux) WriteEncrypted(data []byte, timeoutMs uint32) common.BoolResult {
	if uint32(len(data)) <= FragmentChunkSize {
		encryptResult := c.cipher.Encrypt(data)
		if !encryptResult.HasValue() {
			return common.ErrBool("encrypt failed: " + encryptResult.Err)
		}
		frame := BuildFrame(MsgData, encryptResult.Value)
		_, err := c.conn.Write(frame)
		if err != nil {
			return common.ErrBool("write failed: " + err.Error())
		}
		return common.OkBool(true)
	}

	remaining := len(data)
	offset := 0
	for remaining > 0 {
		chunkSize := int(FragmentChunkSize)
		if remaining < chunkSize {
			chunkSize = remaining
		}
		chunk := data[offset : offset+chunkSize]
		encryptResult := c.cipher.Encrypt(chunk)
		if !encryptResult.HasValue() {
			return common.ErrBool("encrypt fragment failed: " + encryptResult.Err)
		}
		fragmentPayload := BuildFragmentPayload(uint32(len(data)), uint32(offset), encryptResult.Value)
		frame := BuildFrame(MsgDataFragment, fragmentPayload)
		_, err := c.conn.Write(frame)
		if err != nil {
			return common.ErrBool("write fragment failed: " + err.Error())
		}
		offset += chunkSize
		remaining -= chunkSize
	}
	return common.OkBool(true)
}

func (c *clientConnectionImplLinux) ReadLoop(callback ClientRecvDataCallback, timeoutMs uint32) common.BoolResult {
	var rawBuf []byte
	var plainBuf []byte
	chunk := make([]byte, 131072)
	for {
		n, err := c.conn.Read(chunk)
		if err != nil {
			if err == io.EOF {
				break
			}
			return common.ErrBool("recv failed: " + err.Error())
		}
		if n == 0 {
			break
		}
		rawBuf = append(rawBuf, chunk[:n]...)
		for {
			frameType, payload, extractErr := ExtractFrame(&rawBuf)
			if extractErr != nil {
				break
			}
			if frameType == MsgData {
				decryptResult := c.cipher.Decrypt(payload)
				if !decryptResult.HasValue() {
					return common.ErrBool("decrypt failed: " + decryptResult.Err)
				}
				plainBuf = append(plainBuf, decryptResult.Value...)
			} else if frameType == MsgDataFragment {
				totalSize, offset, encryptedChunk, parseErr := ParseFragmentHeader(payload)
				if parseErr != nil {
					return common.ErrBool("fragment parse failed: " + parseErr.Error())
				}
				decryptResult := c.cipher.Decrypt(encryptedChunk)
				if !decryptResult.HasValue() {
					return common.ErrBool("decrypt fragment failed: " + decryptResult.Err)
				}
				if offset == 0 {
					c.reassembly.Reset(totalSize)
				}
				if !c.reassembly.ValidateOffset(offset, len(decryptResult.Value)) {
					return common.ErrBool("fragment offset out of order or overflow")
				}
				copy(c.reassembly.Buffer[offset:], decryptResult.Value)
				c.reassembly.Received += uint32(len(decryptResult.Value))
				c.reassembly.NextOffset = offset + uint32(len(decryptResult.Value))
				if c.reassembly.IsComplete() {
					plainBuf = append(plainBuf, c.reassembly.Buffer...)
					c.reassembly = FragmentReassembly{}
				}
			}
		}
		if len(plainBuf) > 0 {
			if callback(plainBuf) {
				return common.OkBool(true)
			}
			plainBuf = nil
		}
	}
	return common.ErrBool("read loop: connection closed")
}

func (c *clientConnectionImplLinux) Close() {
	common.LogInfo("IPC client closing connection")
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *clientConnectionImplLinux) IsValid() bool {
	return c.conn != nil && c.cipher.IsInitialized()
}

func (c *clientConnectionImplLinux) doHandshake() common.BoolResult {
	ecdh := NewECDHKeyExchange()
	if genResult := ecdh.GenerateKey(); !genResult.HasValue() {
		return common.ErrBool("handshake: key generation failed: " + genResult.Err)
	}
	pubDerResult := ecdh.GetPublicKeyDer()
	if !pubDerResult.HasValue() {
		return common.ErrBool("handshake: get public key failed: " + pubDerResult.Err)
	}
	initFrame := BuildFrame(MsgHandshakeInit, pubDerResult.Value)
	_, err := c.conn.Write(initFrame)
	if err != nil {
		return common.ErrBool("handshake: write init frame failed: " + err.Error())
	}

	var rawBuf []byte
	chunk := make([]byte, 1024)
	for {
		n, err := c.conn.Read(chunk)
		if err != nil {
			return common.ErrBool("handshake: recv failed: " + err.Error())
		}
		if n == 0 {
			return common.ErrBool("handshake: connection closed")
		}
		rawBuf = append(rawBuf, chunk[:n]...)
		frameType, payload, extractErr := ExtractFrame(&rawBuf)
		if extractErr != nil {
			continue
		}
		if frameType == MsgHandshakeResp {
			secretResult := ecdh.ComputeSharedSecret(payload)
			if !secretResult.HasValue() {
				return common.ErrBool("handshake: compute shared secret failed: " + secretResult.Err)
			}
			if initResult := c.cipher.Init(secretResult.Value); !initResult.HasValue() {
				return common.ErrBool("handshake: cipher init failed: " + initResult.Err)
			}
			return common.OkBool(true)
		}
		return common.ErrBool("handshake: unexpected frame type")
	}
}

type clientStateLinux int

const (
	stateWaitHelloLinux clientStateLinux = iota
	stateEstablishedLinux
)

type clientContextLinux struct {
	conn       net.Conn
	state      clientStateLinux
	ecdh       *ECDHKeyExchange
	cipher     *AESGCMCipher
	rawRecvBuf []byte
	writeMu    sync.Mutex
	reassembly FragmentReassembly
}

type serverImplLinux struct {
	serverName string
	listener   net.Listener
	running    bool
	mu         sync.Mutex
	clients    map[ClientId]*clientContextLinux
	callback   ServerRecvDataCallback
}

func newServerImplLinux() Server {
	return &serverImplLinux{clients: make(map[ClientId]*clientContextLinux)}
}

func (s *serverImplLinux) Start(serverName string, callback ServerRecvDataCallback) common.BoolResult {
	common.LogInfo("IPC server starting", "server_name", serverName)
	if s.running {
		return common.ErrBool("server already running")
	}
	s.callback = callback
	addr := &net.UnixAddr{Name: "\x00tyke_" + serverName, Net: "unix"}
	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		return common.ErrBool("listen failed: " + err.Error())
	}
	s.listener = listener
	s.serverName = serverName
	s.running = true
	go s.acceptLoop()
	return common.OkBool(true)
}

func (s *serverImplLinux) acceptLoop() {
	var clientIdCounter atomic.Uint64
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if !s.running {
				return
			}
			continue
		}
		ctx := &clientContextLinux{
			conn:   conn,
			state:  stateWaitHelloLinux,
			ecdh:   NewECDHKeyExchange(),
			cipher: NewAESGCMCipher(),
		}
		cid := ClientId(clientIdCounter.Add(1))
		s.mu.Lock()
		s.clients[cid] = ctx
		s.mu.Unlock()
		go s.handleClient(cid, ctx)
	}
}

func (s *serverImplLinux) handleClient(cid ClientId, ctx *clientContextLinux) {
	chunk := make([]byte, 131072)
	for s.running {
		n, err := ctx.conn.Read(chunk)
		if err != nil {
			s.closeClient(cid)
			return
		}
		ctx.rawRecvBuf = append(ctx.rawRecvBuf, chunk[:n]...)
		if !s.processFrames(cid, ctx) {
			s.closeClient(cid)
			return
		}
	}
}

func (s *serverImplLinux) processFrames(cid ClientId, ctx *clientContextLinux) bool {
	for {
		frameType, payload, err := ExtractFrame(&ctx.rawRecvBuf)
		if err != nil {
			break
		}
		if ctx.state == stateWaitHelloLinux {
			if frameType != MsgHandshakeInit {
				return false
			}
			if genResult := ctx.ecdh.GenerateKey(); !genResult.HasValue() {
				return false
			}
			secretResult := ctx.ecdh.ComputeSharedSecret(payload)
			if !secretResult.HasValue() {
				return false
			}
			if initResult := ctx.cipher.Init(secretResult.Value); !initResult.HasValue() {
				return false
			}
			pubDerResult := ctx.ecdh.GetPublicKeyDer()
			if !pubDerResult.HasValue() {
				return false
			}
			resp := BuildFrame(MsgHandshakeResp, pubDerResult.Value)
			ctx.writeMu.Lock()
			ctx.conn.Write(resp)
			ctx.writeMu.Unlock()
			ctx.state = stateEstablishedLinux
		} else if ctx.state == stateEstablishedLinux {
			if frameType == MsgData {
				decryptResult := ctx.cipher.Decrypt(payload)
				if !decryptResult.HasValue() {
					return false
				}
				dataCopy := decryptResult.Value
				callback := s.callback
				tp := component.GetThreadPoolInstance()
				tp.Enqueue(func() {
					cbSend := func(id ClientId, buf []byte) bool {
						result := s.SendToClient(id, buf)
						return result.HasValue()
					}
					if callback != nil {
						callback(cid, dataCopy, cbSend)
					}
				})
			} else if frameType == MsgDataFragment {
				totalSize, offset, encryptedChunk, parseErr := ParseFragmentHeader(payload)
				if parseErr != nil {
					return false
				}
				decryptResult := ctx.cipher.Decrypt(encryptedChunk)
				if !decryptResult.HasValue() {
					return false
				}
				if offset == 0 {
					ctx.reassembly.Reset(totalSize)
				}
				if !ctx.reassembly.ValidateOffset(offset, len(decryptResult.Value)) {
					return false
				}
				copy(ctx.reassembly.Buffer[offset:], decryptResult.Value)
				ctx.reassembly.Received += uint32(len(decryptResult.Value))
				ctx.reassembly.NextOffset = offset + uint32(len(decryptResult.Value))
				if ctx.reassembly.IsComplete() {
					dataCopy := ctx.reassembly.Buffer
					ctx.reassembly = FragmentReassembly{}
					callback := s.callback
					tp := component.GetThreadPoolInstance()
					tp.Enqueue(func() {
						cbSend := func(id ClientId, buf []byte) bool {
							result := s.SendToClient(id, buf)
							return result.HasValue()
						}
						if callback != nil {
							callback(cid, dataCopy, cbSend)
						}
					})
				}
			} else {
				return false
			}
		}
	}
	return true
}

func (s *serverImplLinux) closeClient(cid ClientId) {
	s.mu.Lock()
	ctx, ok := s.clients[cid]
	if ok {
		delete(s.clients, cid)
	}
	s.mu.Unlock()
	if ok && ctx.conn != nil {
		ctx.conn.Close()
	}
}

func (s *serverImplLinux) Stop() {
	if !s.running {
		return
	}
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}
	s.mu.Lock()
	for cid, ctx := range s.clients {
		ctx.conn.Close()
		delete(s.clients, cid)
	}
	s.mu.Unlock()
}

func (s *serverImplLinux) SendToClient(id ClientId, data []byte) common.BoolResult {
	s.mu.Lock()
	ctx, ok := s.clients[id]
	s.mu.Unlock()
	if !ok {
		return common.ErrBool("client not found")
	}

	if uint32(len(data)) <= FragmentChunkSize {
		encryptResult := ctx.cipher.Encrypt(data)
		if !encryptResult.HasValue() {
			return common.ErrBool("encrypt failed: " + encryptResult.Err)
		}
		frame := BuildFrame(MsgData, encryptResult.Value)
		ctx.writeMu.Lock()
		defer ctx.writeMu.Unlock()
		_, err := ctx.conn.Write(frame)
		if err != nil {
			return common.ErrBool("write to client failed: " + err.Error())
		}
		return common.OkBool(true)
	}

	remaining := len(data)
	offset := 0
	ctx.writeMu.Lock()
	defer ctx.writeMu.Unlock()
	for remaining > 0 {
		chunkSize := int(FragmentChunkSize)
		if remaining < chunkSize {
			chunkSize = remaining
		}
		chunk := data[offset : offset+chunkSize]
		encryptResult := ctx.cipher.Encrypt(chunk)
		if !encryptResult.HasValue() {
			return common.ErrBool("encrypt fragment failed: " + encryptResult.Err)
		}
		fragmentPayload := BuildFragmentPayload(uint32(len(data)), uint32(offset), encryptResult.Value)
		frame := BuildFrame(MsgDataFragment, fragmentPayload)
		_, err := ctx.conn.Write(frame)
		if err != nil {
			return common.ErrBool("write fragment to client failed: " + err.Error())
		}
		offset += chunkSize
		remaining -= chunkSize
	}
	return common.OkBool(true)
}

func createClientConnectionImpl() ClientConnection {
	return newClientConnectionImplLinux()
}

func createServerImpl() Server {
	return newServerImplLinux()
}
