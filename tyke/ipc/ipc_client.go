// Package ipc 为 Tyke 框架提供进程间通信层。
//
// 本文件定义了 IPCConnection 以及顶层 IPCClientSend / IPCClientSendAsync 函数。
// 连接从 ConnectionPoolFactory 管理的 ConnectionPool 中获取。
package ipc

import (
	"sync/atomic"
	"time"

	"tyke-go/common"
)

// IPCConnection 包装一个平台特定的 ClientConnection，带最后使用时间
// 跟踪以支持空闲连接清理。
type IPCConnection struct {
	impl     ClientConnection
	lastUsed atomic.Int64
}

// NewIPCConnection 使用平台特定的实现创建一个新的 IPCConnection。
func NewIPCConnection() *IPCConnection {
	conn := &IPCConnection{impl: createClientConnectionImpl()}
	conn.UpdateLastUsedTime()
	common.LogDebug("IPCConnection constructed")
	return conn
}

// Connect 建立到指定名称的 IPC 服务器的连接。
func (c *IPCConnection) Connect(serverName string, timeoutMs uint32) common.BoolResult {
	common.LogDebug("Connecting to server", "server_name", serverName, "timeout", timeoutMs)
	result := c.impl.Connect(serverName, timeoutMs)
	if !result.HasValue() {
		common.LogError("Connect failed", "error", result.Err)
		return common.ErrBool("connect failed: " + result.Err)
	}
	common.LogDebug("Connected to server", "server_name", serverName)
	return common.OkBool(true)
}

func (c *IPCConnection) Write(data []byte, timeoutMs uint32) common.BoolResult {
	common.LogDebug("Write", "size", len(data), "timeout", timeoutMs)
	result := c.impl.Write(data, timeoutMs)
	if !result.HasValue() {
		common.LogError("Write failed", "error", result.Err)
		return common.ErrBool("write failed: " + result.Err)
	}
	return common.OkBool(true)
}

func (c *IPCConnection) ReadLoop(callback ClientRecvDataCallback, timeoutMs uint32) common.BoolResult {
	common.LogDebug("ReadLoop", "timeout", timeoutMs)
	result := c.impl.ReadLoop(callback, timeoutMs)
	if !result.HasValue() {
		common.LogError("ReadLoop failed", "error", result.Err)
		return common.ErrBool("read loop failed: " + result.Err)
	}
	return common.OkBool(true)
}

func (c *IPCConnection) Close() {
	common.LogDebug("Closing connection")
	c.impl.Close()
}

func (c *IPCConnection) IsValid() bool {
	return c.impl.IsValid()
}

// UpdateLastUsedTime 将最后使用时间戳设置为当前时间。
func (c *IPCConnection) UpdateLastUsedTime() {
	c.lastUsed.Store(time.Now().UnixNano())
}

// GetLastUsedTime 返回此连接上最近活动的时间戳。
func (c *IPCConnection) GetLastUsedTime() time.Time {
	return time.Unix(0, c.lastUsed.Load())
}

// IPCClientSend 同步地向指定名称的服务器发送请求，从连接池获取
// 连接，并通过提供的回调等待响应。调用完成后
// 连接被归还到连接池。
func IPCClientSend(serverName string, request []byte, callback ClientRecvDataCallback, timeoutMs ...uint32) common.BoolResult {
	tm := uint32(IPCDefaultTimeoutMs)
	if len(timeoutMs) > 0 {
		tm = timeoutMs[0]
	}
	common.LogDebug("IPC client sending data", "server_name", serverName, "request_size", len(request))

	pool := GetConnectionPoolFactory().GetPool(serverName)
	conn, err := pool.Acquire()
	if err != nil {
		common.LogError("IPC client sending data acquire connection failed", "error", err)
		return common.ErrBool("send: " + err.Error())
	}

	// 使用 defer 确保连接始终被归还，即使在 panic 或意外错误时也是如此。
	shouldReconnect := true
	defer func() {
		pool.Release(conn, shouldReconnect)
	}()

	if writeResult := conn.Write(request, tm); !writeResult.HasValue() {
		common.LogError("IPC client sending data write failed", "error", writeResult.Err)
		return common.ErrBool("send: " + writeResult.Err)
	}
	if readResult := conn.ReadLoop(callback, tm); !readResult.HasValue() {
		common.LogError("IPC client sending data read failed", "error", readResult.Err)
		return common.ErrBool("send: " + readResult.Err)
	}

	shouldReconnect = false // 成功 — 将健康连接归还到池中
	common.LogDebug("IPC client sending data completed successfully")
	return common.OkBool(true)
}

// IPCClientSendAsync 异步地（发后即忘）向指定名称的服务器发送请求。
// 连接在写入完成后立即归还到池中。
func IPCClientSendAsync(serverName string, request []byte, timeoutMs ...uint32) common.BoolResult {
	tm := uint32(IPCDefaultTimeoutMs)
	if len(timeoutMs) > 0 {
		tm = timeoutMs[0]
	}
	common.LogDebug("IPC client sending dataAsync", "server_name", serverName, "request_size", len(request))

	pool := GetConnectionPoolFactory().GetPool(serverName)
	conn, err := pool.Acquire()
	if err != nil {
		common.LogError("IPC client sending dataAsync acquire connection failed", "error", err)
		return common.ErrBool("send async: " + err.Error())
	}

	if writeResult := conn.Write(request, tm); !writeResult.HasValue() {
		common.LogError("IPC client sending dataAsync write failed", "error", writeResult.Err)
		pool.Release(conn, true)
		return common.ErrBool("send async: write failed")
	}

	// 成功路径归还连接以复用（与 C++ RAII Release(conn,false) 行为对齐）
	pool.Release(conn, false)

	common.LogDebug("IPC client sending dataAsync completed successfully")
	return common.OkBool(true)
}
