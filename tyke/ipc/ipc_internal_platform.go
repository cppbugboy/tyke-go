package ipc

import "github.com/tyke/tyke/tyke/common"

// ClientConnection 定义了客户端连接的平台无关接口。
type ClientConnection interface {
	Connect(serverName string, timeoutMs uint32) common.BoolResult
	WriteEncrypted(data []byte, timeoutMs uint32) common.BoolResult
	ReadLoop(callback ClientRecvDataCallback, timeoutMs uint32) common.BoolResult
	Close()
	IsValid() bool
}

// Server 定义了服务端的平台无关接口。
type Server interface {
	Start(serverName string, callback ServerRecvDataCallback) common.BoolResult
	Stop()
	SendToClient(id ClientId, data []byte) common.BoolResult
}
