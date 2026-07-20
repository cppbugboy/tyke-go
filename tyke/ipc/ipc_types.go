// Package ipc provides an inter-process communication layer.
//
// This file defines IPC-specific types: constants, client/server IDs, and callback signatures.
package ipc

const (
	// IPCDefaultTimeoutMs is the default I/O timeout in milliseconds.
	IPCDefaultTimeoutMs = 5000
	// IPCDefaultMaxConnections is the default maximum connections per pool.
	IPCDefaultMaxConnections = 4
	// IPCDefaultIdleTimeoutMs is the default idle connection timeout.
	IPCDefaultIdleTimeoutMs = 30000
)

// ClientId uniquely identifies a connected client.
type ClientId uint64

// ClientRecvDataCallback is invoked when a client receives data. Return true to stop the read loop.
type ClientRecvDataCallback = func(data []byte) bool

// ServerSendDataCallback is invoked by the server to send data to a specific client.
type ServerSendDataCallback = func(clientId ClientId, data []byte) bool

// ServerRecvDataCallback is invoked when the server receives data from a client.
// Returns the number of bytes consumed, or nil if more data is needed.
type ServerRecvDataCallback = func(clientId ClientId, data []byte, sendCallback ServerSendDataCallback) *uint32
