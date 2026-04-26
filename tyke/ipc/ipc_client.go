package ipc

import (
	"time"

	"github.com/cppbugboy/tyke-go/tyke/common"
)

type IPCConnection struct {
	impl     ClientConnection
	lastUsed time.Time
}

func NewIPCConnection() *IPCConnection {
	conn := &IPCConnection{impl: createClientConnectionImpl()}
	conn.UpdateLastUsedTime()
	common.LogDebug("IPCConnection constructed")
	return conn
}

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

func (c *IPCConnection) WriteEncrypted(data []byte, timeoutMs uint32) common.BoolResult {
	common.LogDebug("WriteEncrypted", "size", len(data), "timeout", timeoutMs)
	result := c.impl.WriteEncrypted(data, timeoutMs)
	if !result.HasValue() {
		common.LogError("WriteEncrypted failed", "error", result.Err)
		return common.ErrBool("write encrypted failed: " + result.Err)
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

func (c *IPCConnection) UpdateLastUsedTime() {
	c.lastUsed = time.Now()
}

func (c *IPCConnection) GetLastUsedTime() time.Time {
	return c.lastUsed
}

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

	if writeResult := conn.WriteEncrypted(request, tm); !writeResult.HasValue() {
		common.LogError("IPC client sending data write failed", "error", writeResult.Err)
		pool.Release(conn, true)
		return common.ErrBool("send: " + writeResult.Err)
	}
	if readResult := conn.ReadLoop(callback, tm); !readResult.HasValue() {
		common.LogError("IPC client sending data read failed", "error", readResult.Err)
		pool.Release(conn, true)
		return common.ErrBool("send: " + readResult.Err)
	}

	pool.Release(conn, false)
	common.LogDebug("IPC client sending data completed successfully")
	return common.OkBool(true)
}

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

	if writeResult := conn.WriteEncrypted(request, tm); !writeResult.HasValue() {
		common.LogError("IPC client sending dataAsync write failed", "error", writeResult.Err)
		pool.Release(conn, true)
		return common.ErrBool("send async: write encrypted failed")
	}

	pool.Release(conn, true)

	common.LogDebug("IPC client sending dataAsync completed successfully")
	return common.OkBool(true)
}
