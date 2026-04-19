//go:build windows

package ipc

import (
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/component"
)

type clientConnectionImplWin struct {
	conn   net.Conn
	cipher *AesGcmCipher
}

func newClientConnectionImplWin() IClientConnectionImpl {
	return &clientConnectionImplWin{cipher: NewAesGcmCipher()}
}

func (c *clientConnectionImplWin) Connect(serverName string, timeoutMs uint32, rwTimeoutMs uint32) common.BoolResult {
	common.LogInfo("ipc client connecting to", "server_name", serverName)
	pipePath := `\\.\pipe\` + serverName
	timeout := time.Duration(timeoutMs) * time.Millisecond
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := winio.DialPipe(pipePath, nil)
		if err == nil {
			c.conn = conn
			return c.doHandshake(timeoutMs)
		}
		if os.IsNotExist(err) || isPipeBusy(err) {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		return common.ErrBool("connect failed: " + err.Error())
	}
	return common.ErrBool("connect failed: timeout waiting for pipe")
}

func isPipeBusy(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "pipe is busy") ||
		strings.Contains(err.Error(), "ERROR_PIPE_BUSY") ||
		strings.Contains(err.Error(), "No process is on the other end")
}

func (c *clientConnectionImplWin) WriteEncrypted(data []byte, timeoutMs uint32) common.BoolResult {
	encryptResult := c.cipher.Encrypt(data)
	if !encryptResult.HasValue() {
		return common.ErrBool("encrypt failed: " + encryptResult.Err)
	}
	frame := BuildFrame(MsgData, encryptResult.Value)
	if c.conn != nil {
		c.conn.SetWriteDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
		_, err := c.conn.Write(frame)
		if err != nil {
			return common.ErrBool("write failed: " + err.Error())
		}
	}
	return common.OkBool(true)
}

func (c *clientConnectionImplWin) ReadLoop(callback ClientRecvDataCallback, timeoutMs uint32) common.BoolResult {
	var rawBuf []byte
	var plainBuf []byte
	chunk := make([]byte, 4096)
	for {
		if c.conn != nil {
			c.conn.SetReadDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
		}
		n, err := c.conn.Read(chunk)
		if err != nil {
			if err == io.EOF {
				break
			}
			return common.ErrBool("read failed: " + err.Error())
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

func (c *clientConnectionImplWin) Close() {
	common.LogInfo("ipc client closing connection")
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *clientConnectionImplWin) IsValid() bool {
	return c.conn != nil && c.cipher.IsInitialized()
}

func (c *clientConnectionImplWin) doHandshake(timeoutMs uint32) common.BoolResult {
	ecdh := NewEcdhKeyExchange()
	if genResult := ecdh.GenerateKey(); !genResult.HasValue() {
		return common.ErrBool("handshake: key generation failed: " + genResult.Err)
	}
	pubDerResult := ecdh.GetPublicKeyDer()
	if !pubDerResult.HasValue() {
		return common.ErrBool("handshake: get public key failed: " + pubDerResult.Err)
	}
	initFrame := BuildFrame(MsgHandshakeInit, pubDerResult.Value)
	if c.conn != nil {
		c.conn.SetWriteDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
		_, err := c.conn.Write(initFrame)
		if err != nil {
			return common.ErrBool("handshake: write init frame failed: " + err.Error())
		}
	}

	var rawBuf []byte
	chunk := make([]byte, 1024)
	for {
		if c.conn != nil {
			c.conn.SetReadDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
		}
		n, err := c.conn.Read(chunk)
		if err != nil {
			return common.ErrBool("handshake: read failed: " + err.Error())
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

type clientState int

const (
	stateWaitHello clientState = iota
	stateEstablished
)

type clientContext struct {
	conn         net.Conn
	state        clientState
	ecdh         *EcdhKeyExchange
	cipher       *AesGcmCipher
	rawRecvBuf   []byte
	writeMu      sync.Mutex
	pendingWrite []byte
}

type serverImplWin struct {
	serverName string
	listener   net.Listener
	running    bool
	mu         sync.Mutex
	clients    map[ClientId]*clientContext
	callback   ServerRecvDataCallback
}

func newServerImplWin() IServerImpl {
	return &serverImplWin{clients: make(map[ClientId]*clientContext)}
}

func (s *serverImplWin) Start(serverName string, callback ServerRecvDataCallback) common.BoolResult {
	common.LogInfo("ipc server starting on", "server_name", serverName)
	if s.running {
		return common.ErrBool("server already running")
	}
	s.callback = callback
	pipePath := `\\.\pipe\` + serverName
	cfg := &winio.PipeConfig{
		MessageMode: true,
	}
	listener, err := winio.ListenPipe(pipePath, cfg)
	if err != nil {
		return common.ErrBool("listen failed: " + err.Error())
	}
	s.listener = listener
	s.running = true

	go s.acceptLoop()
	return common.OkBool(true)
}

func (s *serverImplWin) acceptLoop() {
	var clientIdCounter ClientId = 1
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if !s.running {
				return
			}
			continue
		}
		ctx := &clientContext{
			conn:   conn,
			state:  stateWaitHello,
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

func (s *serverImplWin) handleClient(cid ClientId, ctx *clientContext) {
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

func (s *serverImplWin) processFrames(cid ClientId, ctx *clientContext) bool {
	for {
		frameType, payload, err := ExtractFrame(&ctx.rawRecvBuf)
		if err != nil {
			break
		}
		if ctx.state == stateWaitHello {
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
			ctx.pendingWrite = append(ctx.pendingWrite, resp...)
			ctx.writeMu.Unlock()
			s.writeToClient(ctx)
			ctx.state = stateEstablished
		} else if ctx.state == stateEstablished {
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

func (s *serverImplWin) writeToClient(ctx *clientContext) bool {
	ctx.writeMu.Lock()
	defer ctx.writeMu.Unlock()
	if len(ctx.pendingWrite) == 0 {
		return true
	}
	n, err := ctx.conn.Write(ctx.pendingWrite)
	if err != nil {
		return false
	}
	ctx.pendingWrite = ctx.pendingWrite[n:]
	return true
}

func (s *serverImplWin) closeClient(cid ClientId) {
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

func (s *serverImplWin) Stop() {
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

func (s *serverImplWin) SendToClient(id ClientId, data []byte) common.BoolResult {
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
	ctx.pendingWrite = append(ctx.pendingWrite, frame...)
	ctx.writeMu.Unlock()
	if !s.writeToClient(ctx) {
		return common.ErrBool("write to client failed")
	}
	return common.OkBool(true)
}

func createClientConnectionImpl() IClientConnectionImpl {
	return newClientConnectionImplWin()
}

func createServerImpl() IServerImpl {
	return newServerImplWin()
}
