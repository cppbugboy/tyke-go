package ipc

import (
	"time"

	"github.com/tyke/tyke/tyke/common"
)

type IpcConnection struct {
	impl     IClientConnectionImpl
	lastUsed time.Time
}

func NewIpcConnection() *IpcConnection {
	conn := &IpcConnection{impl: createClientConnectionImpl()}
	conn.UpdateLastUsedTime()
	common.LogDebug("IpcConnection constructed")
	return conn
}

func (c *IpcConnection) Connect(serverName string, timeoutMs uint32, rwTimeoutMs uint32) common.BoolResult {
	common.LogDebug("Connecting to server", "server_name", serverName, "timeout", timeoutMs)
	result := c.impl.Connect(serverName, timeoutMs, rwTimeoutMs)
	if !result.HasValue() {
		common.LogError("Connect failed", "error", result.Err)
		return common.ErrBool("connect failed: " + result.Err)
	}
	common.LogDebug("Connected to server", "server_name", serverName)
	return common.OkBool(true)
}

func (c *IpcConnection) WriteEncrypted(data []byte, timeoutMs uint32) common.BoolResult {
	common.LogDebug("WriteEncrypted", "size", len(data), "timeout", timeoutMs)
	result := c.impl.WriteEncrypted(data, timeoutMs)
	if !result.HasValue() {
		common.LogError("WriteEncrypted failed", "error", result.Err)
		return common.ErrBool("write encrypted failed: " + result.Err)
	}
	return common.OkBool(true)
}

func (c *IpcConnection) ReadLoop(callback ClientRecvDataCallback, timeoutMs uint32) common.BoolResult {
	common.LogDebug("ReadLoop", "timeout", timeoutMs)
	result := c.impl.ReadLoop(callback, timeoutMs)
	if !result.HasValue() {
		common.LogError("ReadLoop failed", "error", result.Err)
		return common.ErrBool("read loop failed: " + result.Err)
	}
	return common.OkBool(true)
}

func (c *IpcConnection) Close() {
	common.LogDebug("Closing connection")
	c.impl.Close()
}

func (c *IpcConnection) IsValid() bool {
	return c.impl.IsValid()
}

func (c *IpcConnection) UpdateLastUsedTime() {
	c.lastUsed = time.Now()
}

func (c *IpcConnection) GetLastUsedTime() time.Time {
	return c.lastUsed
}

func IpcClientSend(serverName string, request []byte, callback ClientRecvDataCallback, timeoutMs ...uint32) common.BoolResult {
	tm := uint32(IpcDefaultTimeoutMs)
	if len(timeoutMs) > 0 {
		tm = timeoutMs[0]
	}
	common.LogDebug("IpcClient::Send", "server_name", serverName, "request_size", len(request))

	conn := NewIpcConnection()
	defer conn.Close()
	if connectResult := conn.Connect(serverName, tm, tm); !connectResult.HasValue() {
		common.LogError("IpcClient::Send connect failed", "error", connectResult.Err)
		return common.ErrBool("send: " + connectResult.Err)
	}
	if writeResult := conn.WriteEncrypted(request, tm); !writeResult.HasValue() {
		common.LogError("IpcClient::Send write failed", "error", writeResult.Err)
		return common.ErrBool("send: " + writeResult.Err)
	}
	if readResult := conn.ReadLoop(callback, tm); !readResult.HasValue() {
		common.LogError("IpcClient::Send read failed", "error", readResult.Err)
		return common.ErrBool("send: " + readResult.Err)
	}
	common.LogDebug("IpcClient::Send completed successfully")
	return common.OkBool(true)
}

func IpcClientSendAsync(serverName string, request []byte, timeoutMs ...uint32) common.BoolResult {
	tm := uint32(IpcDefaultTimeoutMs)
	if len(timeoutMs) > 0 {
		tm = timeoutMs[0]
	}
	common.LogDebug("IpcClient::SendAsync", "server_name", serverName, "request_size", len(request))

	conn := NewIpcConnection()
	defer conn.Close()
	if connectResult := conn.Connect(serverName, tm, tm); !connectResult.HasValue() {
		common.LogError("IpcClient::SendAsync connect failed", "error", connectResult.Err)
		return common.ErrBool("send async: " + connectResult.Err)
	}
	if writeResult := conn.WriteEncrypted(request, tm); !writeResult.HasValue() {
		common.LogError("IpcClient::SendAsync write failed", "error", writeResult.Err)
		return common.ErrBool("send async: " + writeResult.Err)
	}
	common.LogDebug("IpcClient::SendAsync completed successfully")
	return common.OkBool(true)
}
