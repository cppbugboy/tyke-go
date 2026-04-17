package ipc

type ClientId uint64

const (
	IpcDefaultTimeoutMs    uint32 = 5000
	IpcDefaultMaxConnections      = 4
	IpcDefaultIdleTimeoutMs uint32 = 60000
)

type ClientRecvDataCallback func(data []byte) bool

type ServerSendDataCallback func(id ClientId, data []byte) bool

type ServerRecvDataCallback func(id ClientId, data []byte, sendCb ServerSendDataCallback) uint32
