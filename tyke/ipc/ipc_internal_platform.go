package ipc

import "github.com/tyke/tyke/tyke/common"

type ClientConnection interface {
	Connect(serverName string, timeoutMs uint32) common.BoolResult
	WriteEncrypted(data []byte, timeoutMs uint32) common.BoolResult
	ReadLoop(callback ClientRecvDataCallback, timeoutMs uint32) common.BoolResult
	Close()
	IsValid() bool
}

type Server interface {
	Start(serverName string, callback ServerRecvDataCallback) common.BoolResult
	Stop()
	SendToClient(id ClientId, data []byte) common.BoolResult
}
