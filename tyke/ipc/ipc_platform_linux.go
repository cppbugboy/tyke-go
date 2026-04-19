//go:build linux

package ipc

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/component"
)

type clientConnectionImplLinux struct {
	conn   net.Conn
	cipher *AesGcmCipher
}

func newClientConnectionImplLinux() IClientConnectionImpl {
	return &clientConnectionImplLinux{cipher: NewAesGcmCipher()}
}

func (c *clientConnectionImplLinux) Connect(serverName string, timeoutMs uint32, rwTimeoutMs uint32) common.BoolResult {
	common.LogInfo("ipc client connecting to", "server_name", serverName)
	addr := fmt.Sprintf("/tmp/%s", serverName)
	conn, err := net.DialTimeout("unix", addr, time.Duration(timeoutMs)*time.Millisecond)
	if err != nil {
		return common.ErrBool("connect failed: " + err.Error())
	}
	c.conn = conn
	return c.doHandshake()
}

func (c *clientConnectionImplLinux) WriteEncrypted(data []byte, timeoutMs uint32) common.BoolResult {
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

func (c *clientConnectionImplLinux) ReadLoop(callback ClientRecvDataCallback, timeoutMs uint32) common.BoolResult {
	var rawBuf []byte
	var plainBuf []byte
	chunk := make([]byte, 4096)
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
					return common.ErrByteVec("decrypt failed: " + decryptResult.Err)
				}
				plainBuf = append(plainBuf, decryptResult.Value...)
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
	common.LogInfo("ipc client closing connection")
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *clientConnectionImplLinux) IsValid() bool {
	return c.conn != nil && c.cipher.IsInitialized()
}

func (c *clientConnectionImplLinux) doHandshake() common.BoolResult {
	ecdh := NewEcdhKeyExchange()
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
	ecdh       *EcdhKeyExchange
	cipher     *AesGcmCipher
	rawRecvBuf []byte
	writeMu    sync.Mutex
}

type serverImplLinux struct {
	serverName string
	listener   net.Listener
	running    bool
	mu         sync.Mutex
	clients    map[ClientId]*clientContextLinux
	callback   ServerRecvDataCallback
}

func newServerImplLinux() IServerImpl {
	return &serverImplLinux{clients: make(map[ClientId]*clientContextLinux)}
}

func (s *serverImplLinux) Start(serverName string, callback ServerRecvDataCallback) common.BoolResult {
	common.LogInfo("ipc server starting on", "server_name", serverName)
	if s.running {
		return common.ErrBool("server already running")
	}
	s.callback = callback
	addr := fmt.Sprintf("/tmp/%s", serverName)
	listener, err := net.Listen("unix", addr)
	if err != nil {
		return common.ErrBool("listen failed: " + err.Error())
	}
	s.listener = listener
	s.serverName = addr
	s.running = true
	go s.acceptLoop()
	return common.OkBool(true)
}

func (s *serverImplLinux) acceptLoop() {
	var clientIdCounter ClientId = 1
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
			ecdh:   NewEcdhKeyExchange(),
			cipher: NewAesGcmCipher(),
		}
		cid := clientIdCounter
		clientIdCounter++
		s.mu.Lock()
		s.clients[cid] = ctx
		s.mu.Unlock()
		go s.handleClient(cid, ctx)
	}
}

func (s *serverImplLinux) handleClient(cid ClientId, ctx *clientContextLinux) {
	chunk := make([]byte, 4096)
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
			if frameType != MsgData {
				return false
			}
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

func createClientConnectionImpl() IClientConnectionImpl {
	return newClientConnectionImplLinux()
}

func createServerImpl() IServerImpl {
	return newServerImplLinux()
}
