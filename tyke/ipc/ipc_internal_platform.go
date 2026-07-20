// Package ipc provides an inter-process communication layer.
//
// This file defines the platform-agnostic interfaces that platform-specific
// implementations must satisfy: ClientConnection and Server.
package ipc

import "tyke-go/common"

// ClientConnection is the interface for platform-specific client connections.
type ClientConnection interface {
	Connect(serverName string, timeoutMs uint32) common.BoolResult
	Write(data []byte, timeoutMs uint32) common.BoolResult
	ReadLoop(callback ClientRecvDataCallback, timeoutMs uint32) common.BoolResult
	Close()
	IsValid() bool
}

// Server is the interface for platform-specific server implementations.
type Server interface {
	Start(serverName string, callback ServerRecvDataCallback) common.BoolResult
	Stop()
	SendToClient(id ClientId, data []byte) common.BoolResult
}
