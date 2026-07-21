//go:build linux

// Package ipc provides a Linux platform implementation using Unix domain sockets
// (abstract namespace). Implements ClientConnection and Server interfaces defined
// in ipc_internal_platform.go.
package ipc

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"tyke-go/common"
	"tyke-go/component"
)

// serverReadTimeout 是服务端单次读取客户端数据的超时时间，
// 超时后关闭连接，避免慢/卡死客户端永久占用 goroutine。
const serverReadTimeout = 30 * time.Second

type clientConnectionImplLinux struct {
	mu         sync.Mutex
	conn       net.Conn
	reassembly FragmentReassembly
}

func newClientConnectionImplLinux() ClientConnection {
	return &clientConnectionImplLinux{}
}

func (c *clientConnectionImplLinux) Connect(serverName string, timeoutMs uint32) common.BoolResult {
	common.LogInfo("IPC client connecting", "server_name", serverName)
	addr := &net.UnixAddr{Name: "\x00tyke_" + serverName, Net: "unix"}
	dialer := net.Dialer{Timeout: time.Duration(timeoutMs) * time.Millisecond}
	conn, err := dialer.Dial("unix", addr.Name)
	if err != nil {
		return common.ErrBool("connect failed: " + err.Error())
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	return common.OkBool(true)
}

func (c *clientConnectionImplLinux) Write(data []byte, timeoutMs uint32) common.BoolResult {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return common.ErrBool("write failed: not connected")
	}

	if uint32(len(data)) <= FragmentChunkSize {
		frame := BuildFrame(MsgData, data)
		conn.SetWriteDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
		_, err := conn.Write(frame)
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
		fragmentPayload := BuildFragmentPayload(uint32(len(data)), uint32(offset), chunk)
		frame := BuildFrame(MsgDataFragment, fragmentPayload)
		conn.SetWriteDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
		_, err := conn.Write(frame)
		if err != nil {
			return common.ErrBool("write fragment failed: " + err.Error())
		}
		offset += chunkSize
		remaining -= chunkSize
	}
	return common.OkBool(true)
}

func (c *clientConnectionImplLinux) ReadLoop(callback ClientRecvDataCallback, timeoutMs uint32) common.BoolResult {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return common.ErrBool("read loop: connection not established")
	}
	var rawBuf []byte
	var plainBuf []byte
	chunk := make([]byte, 1048576) // 1MB，与 Windows 平台一致
	for {
		conn.SetReadDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
		n, err := conn.Read(chunk)
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
				plainBuf = append(plainBuf, payload...)
			} else if frameType == MsgDataFragment {
				totalSize, offset, chunkData, parseErr := ParseFragmentHeader(payload)
				if parseErr != nil {
					return common.ErrBool("fragment parse failed: " + parseErr.Error())
				}
				if offset == 0 {
					c.reassembly.Reset(totalSize)
				}
				if !c.reassembly.ValidateOffset(offset, len(chunkData)) {
					return common.ErrBool("fragment offset out of order or overflow")
				}
				copy(c.reassembly.Buffer[offset:], chunkData)
				c.reassembly.Received += uint32(len(chunkData))
				c.reassembly.NextOffset = offset + uint32(len(chunkData))
				if c.reassembly.IsComplete() {
					plainBuf = append(plainBuf, c.reassembly.Buffer...)
					c.reassembly = FragmentReassembly{}
				}
			} else {
				return common.ErrBool(fmt.Sprintf("unknown frame type: 0x%02X", frameType))
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
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	if conn != nil {
		conn.Close()
	}
}

func (c *clientConnectionImplLinux) IsValid() bool {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	return conn != nil
}

type clientContextLinux struct {
	conn         net.Conn
	rawRecvBuf   []byte
	writeMu      sync.Mutex
	pendingWrite []byte
	reassembly   FragmentReassembly
}

type serverImplLinux struct {
	serverName string
	listener   net.Listener
	running    atomic.Bool
	mu         sync.Mutex
	clients    map[ClientId]*clientContextLinux
	callback   ServerRecvDataCallback
}

func newServerImplLinux() Server {
	return &serverImplLinux{clients: make(map[ClientId]*clientContextLinux)}
}

func (s *serverImplLinux) Start(serverName string, callback ServerRecvDataCallback) common.BoolResult {
	common.LogInfo("IPC server starting", "server_name", serverName)
	if s.running.Load() {
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
	s.running.Store(true)
	go s.acceptLoop()
	return common.OkBool(true)
}

func (s *serverImplLinux) acceptLoop() {
	var clientIdCounter atomic.Uint64
	for s.running.Load() {
		conn, err := s.listener.Accept()
		if err != nil {
			if !s.running.Load() {
				return
			}
			continue
		}
		ctx := &clientContextLinux{
			conn: conn,
		}
		cid := ClientId(clientIdCounter.Add(1))
		s.mu.Lock()
		s.clients[cid] = ctx
		s.mu.Unlock()
		go s.handleClient(cid, ctx)
	}
}

func (s *serverImplLinux) handleClient(cid ClientId, ctx *clientContextLinux) {
	common.LogInfo("Server handling client connection", "client_id", cid)
	chunk := make([]byte, 1048576) // 1MB，与 Windows 平台一致
	for s.running.Load() {
		if ctx.conn != nil {
			ctx.conn.SetReadDeadline(time.Now().Add(serverReadTimeout))
		}
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

func (s *serverImplLinux) processFrames(cid ClientId, ctx *clientContextLinux) bool {
	for {
		frameType, payload, err := ExtractFrame(&ctx.rawRecvBuf)
		if err != nil {
			break
		}
		common.LogDebug("Server processing frame", "client_id", cid, "frame_type", fmt.Sprintf("0x%02X", frameType), "payload_len", len(payload))
		if frameType == MsgData {
			dataCopy := payload
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
			totalSize, offset, chunkData, parseErr := ParseFragmentHeader(payload)
			if parseErr != nil {
				common.LogError("Server fragment parse failed", "error", parseErr)
				return false
			}
			if offset == 0 {
				ctx.reassembly.Reset(totalSize)
			}
			if !ctx.reassembly.ValidateOffset(offset, len(chunkData)) {
				common.LogError("Server fragment offset out of order or overflow", "offset", offset, "chunk_size", len(chunkData), "total", ctx.reassembly.Total)
				return false
			}
			copy(ctx.reassembly.Buffer[offset:], chunkData)
			ctx.reassembly.Received += uint32(len(chunkData))
			ctx.reassembly.NextOffset = offset + uint32(len(chunkData))
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
	return true
}

// writeToClientLocked 在**已持有 ctx.writeMu** 的前提下写出 pendingWrite。
// 供 SendToClient 多分片路径在整段持锁的临界区内调用，避免分片交错。
func (s *serverImplLinux) writeToClientLocked(ctx *clientContextLinux) bool {
	for len(ctx.pendingWrite) > 0 {
		n, err := ctx.conn.Write(ctx.pendingWrite)
		if err != nil {
			return false
		}
		ctx.pendingWrite = ctx.pendingWrite[n:]
	}
	return true
}

// writeToClient 自带 writeMu 加锁，适用于小消息路径等短临界区场景。
func (s *serverImplLinux) writeToClient(ctx *clientContextLinux) bool {
	ctx.writeMu.Lock()
	defer ctx.writeMu.Unlock()
	return s.writeToClientLocked(ctx)
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
	if !s.running.Load() {
		return
	}
	s.running.Store(false)
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
		frame := BuildFrame(MsgData, data)
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
	// 多分片路径：整段 append + 写出在同一个 writeMu 临界区内完成，
	// 避免与其他 SendToClient 调用交错导致同一 client 的分片流损坏（与 Windows 版本对齐）。
	ctx.writeMu.Lock()
	defer ctx.writeMu.Unlock()
	for remaining > 0 {
		chunkSize := int(FragmentChunkSize)
		if remaining < chunkSize {
			chunkSize = remaining
		}
		chunk := data[offset : offset+chunkSize]
		fragmentPayload := BuildFragmentPayload(uint32(len(data)), uint32(offset), chunk)
		frame := BuildFrame(MsgDataFragment, fragmentPayload)
		ctx.pendingWrite = append(ctx.pendingWrite, frame...)
		offset += chunkSize
		remaining -= chunkSize
	}
	if !s.writeToClientLocked(ctx) {
		return common.ErrBool("write fragments to client failed")
	}
	return common.OkBool(true)
}

func createClientConnectionImpl() ClientConnection {
	return newClientConnectionImplLinux()
}

func createServerImpl() Server {
	return newServerImplLinux()
}
