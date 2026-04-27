//go:build windows

package ipc

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"tyke-go/common"
	"tyke-go/component"
)

type clientConnectionImplWin struct {
	conn       net.Conn
	cipher     *AESGCMCipher
	reassembly FragmentReassembly
}

func newClientConnectionImplWin() ClientConnection {
	return &clientConnectionImplWin{cipher: NewAESGCMCipher()}
}

func (c *clientConnectionImplWin) Connect(serverName string, timeoutMs uint32) common.BoolResult {
	common.LogInfo("IPC client connecting", "server_name", serverName)
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
	if os.IsTimeout(err) {
		return true
	}
	if isWinSysError(err, 231) || isWinSysError(err, 232) {
		return true
	}
	if pathErr, ok := err.(*os.PathError); ok {
		if isWinSysError(pathErr.Err, 231) || isWinSysError(pathErr.Err, 232) {
			return true
		}
	}
	return false
}

func isWinSysError(err error, code uintptr) bool {
	if sysErr, ok := err.(syscall.Errno); ok {
		return uintptr(sysErr) == code
	}
	return false
}

func (c *clientConnectionImplWin) WriteEncrypted(data []byte, timeoutMs uint32) common.BoolResult {
	if uint32(len(data)) <= FragmentChunkSize {
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
		if c.conn != nil {
			c.conn.SetWriteDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
			_, err := c.conn.Write(frame)
			if err != nil {
				return common.ErrBool("write fragment failed: " + err.Error())
			}
		}
		offset += chunkSize
		remaining -= chunkSize
	}
	return common.OkBool(true)
}

func (c *clientConnectionImplWin) ReadLoop(callback ClientRecvDataCallback, timeoutMs uint32) common.BoolResult {
	var rawBuf []byte
	var plainBuf []byte
	chunk := make([]byte, 131072)
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

func (c *clientConnectionImplWin) Close() {
	common.LogInfo("IPC client closing connection")
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *clientConnectionImplWin) IsValid() bool {
	return c.conn != nil && c.cipher.IsInitialized()
}

func (c *clientConnectionImplWin) doHandshake(timeoutMs uint32) common.BoolResult {
	ecdh := NewECDHKeyExchange()
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
	ecdh         *ECDHKeyExchange
	cipher       *AESGCMCipher
	rawRecvBuf   []byte
	writeMu      sync.Mutex
	pendingWrite []byte
	reassembly   FragmentReassembly
}

type serverImplWin struct {
	serverName string
	listener   net.Listener
	running    bool
	mu         sync.Mutex
	clients    map[ClientId]*clientContext
	callback   ServerRecvDataCallback
}

func newServerImplWin() Server {
	return &serverImplWin{clients: make(map[ClientId]*clientContext)}
}

func (s *serverImplWin) Start(serverName string, callback ServerRecvDataCallback) common.BoolResult {
	common.LogInfo("IPC server starting", "server_name", serverName)
	if s.running {
		return common.ErrBool("server already running")
	}
	s.callback = callback
	pipePath := `\\.\pipe\` + serverName
	cfg := &winio.PipeConfig{
		MessageMode:      false,
		InputBufferSize:  262144,
		OutputBufferSize: 262144,
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
	var clientIdCounter atomic.Uint64
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

func (s *serverImplWin) handleClient(cid ClientId, ctx *clientContext) {
	common.LogInfo("Server handling client connection", "client_id", cid)
	chunk := make([]byte, 131072)
	for s.running {
		n, err := ctx.conn.Read(chunk)
		if err != nil {
			common.LogError("Server read error for client", "client_id", cid, "error", err)
			s.closeClient(cid)
			return
		}
		common.LogInfo("Server read from client", "client_id", cid, "bytes", n)
		ctx.rawRecvBuf = append(ctx.rawRecvBuf, chunk[:n]...)
		if !s.processFrames(cid, ctx) {
			common.LogError("Server processFrames failed for client", "client_id", cid)
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
		common.LogDebug("Server processing frame", "client_id", cid, "frame_type", fmt.Sprintf("0x%02X", frameType), "payload_len", len(payload))
		if ctx.state == stateWaitHello {
			if frameType != MsgHandshakeInit {
				common.LogError("Server received non-HandshakeInit frame during handshake", "frame_type", fmt.Sprintf("0x%02X", frameType))
				return false
			}
			common.LogDebug("Server processing HandshakeInit from client", "client_id", cid, "pubkey_len", len(payload))

			if genResult := ctx.ecdh.GenerateKey(); !genResult.HasValue() {
				common.LogError("Server ECDH key generation failed", "error", genResult.Err)
				return false
			}
			common.LogDebug("Server ECDH key generated")

			secretResult := ctx.ecdh.ComputeSharedSecret(payload)
			if !secretResult.HasValue() {
				common.LogError("Server compute shared secret failed", "error", secretResult.Err)
				return false
			}
			common.LogDebug("Server computed shared secret", "len", len(secretResult.Value))

			if initResult := ctx.cipher.Init(secretResult.Value); !initResult.HasValue() {
				common.LogError("Server cipher init failed", "error", initResult.Err)
				return false
			}
			common.LogDebug("Server cipher initialized")

			pubDerResult := ctx.ecdh.GetPublicKeyDer()
			if !pubDerResult.HasValue() {
				common.LogError("Server get public key DER failed", "error", pubDerResult.Err)
				return false
			}
			common.LogDebug("Server public key DER obtained", "len", len(pubDerResult.Value))

			resp := BuildFrame(MsgHandshakeResp, pubDerResult.Value)
			common.LogDebug("Server sending handshake response frame", "len", len(resp))

			ctx.writeMu.Lock()
			ctx.pendingWrite = append(ctx.pendingWrite, resp...)
			ctx.writeMu.Unlock()
			if !s.writeToClient(ctx) {
				common.LogError("Server failed to write handshake response")
				return false
			}
			common.LogInfo("Server handshake completed for client", "client_id", cid)
			ctx.state = stateEstablished
		} else if ctx.state == stateEstablished {
			if frameType == MsgData {
				decryptResult := ctx.cipher.Decrypt(payload)
				if !decryptResult.HasValue() {
					common.LogError("Server decrypt failed", "error", decryptResult.Err)
					return false
				}
				dataCopy := decryptResult.Value
				callback := s.callback
				tp := component.GetCoroutinePoolInstance()
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
					common.LogError("Server fragment parse failed", "error", parseErr)
					return false
				}
				decryptResult := ctx.cipher.Decrypt(encryptedChunk)
				if !decryptResult.HasValue() {
					common.LogError("Server fragment decrypt failed", "error", decryptResult.Err)
					return false
				}
				if offset == 0 {
					ctx.reassembly.Reset(totalSize)
				}
				if !ctx.reassembly.ValidateOffset(offset, len(decryptResult.Value)) {
					common.LogError("Server fragment offset out of order or overflow", "offset", offset, "chunk_size", len(decryptResult.Value), "total", ctx.reassembly.Total)
					return false
				}
				copy(ctx.reassembly.Buffer[offset:], decryptResult.Value)
				ctx.reassembly.Received += uint32(len(decryptResult.Value))
				ctx.reassembly.NextOffset = offset + uint32(len(decryptResult.Value))
				if ctx.reassembly.IsComplete() {
					dataCopy := ctx.reassembly.Buffer
					ctx.reassembly = FragmentReassembly{}
					callback := s.callback
					tp := component.GetCoroutinePoolInstance()
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
				common.LogError("Server received unknown frame type", "frame_type", fmt.Sprintf("0x%02X", frameType))
				return false
			}
		}
	}
	return true
}

func (s *serverImplWin) writeToClient(ctx *clientContext) bool {
	ctx.writeMu.Lock()
	defer ctx.writeMu.Unlock()
	for len(ctx.pendingWrite) > 0 {
		n, err := ctx.conn.Write(ctx.pendingWrite)
		if err != nil {
			return false
		}
		ctx.pendingWrite = ctx.pendingWrite[n:]
	}
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

	if uint32(len(data)) <= FragmentChunkSize {
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

	remaining := len(data)
	offset := 0
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
		ctx.writeMu.Lock()
		ctx.pendingWrite = append(ctx.pendingWrite, frame...)
		ctx.writeMu.Unlock()
		offset += chunkSize
		remaining -= chunkSize
	}
	if !s.writeToClient(ctx) {
		return common.ErrBool("write fragments to client failed")
	}
	return common.OkBool(true)
}

func createClientConnectionImpl() ClientConnection {
	return newClientConnectionImplWin()
}

func createServerImpl() Server {
	return newServerImplWin()
}
