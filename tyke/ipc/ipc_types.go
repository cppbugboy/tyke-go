package ipc

const (
	IpcDefaultTimeoutMs      = 5000
	IpcDefaultMaxConnections = 4
	IpcDefaultIdleTimeoutMs  = 60000
)

type ClientId = uint64

type ClientRecvDataCallback = func(data []byte) bool

type ServerSendDataCallback = func(clientId ClientId, data []byte) bool

type ServerRecvDataCallback = func(clientId ClientId, data []byte, sendCallback ServerSendDataCallback) *uint32
