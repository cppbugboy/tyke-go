package ipc

const (
	// IPCDefaultTimeoutMs 是 IPC 操作的默认超时时间（毫秒）。
	IPCDefaultTimeoutMs = 5000
	// IPCDefaultMaxConnections 是连接池的默认最大连接数。
	IPCDefaultMaxConnections = 4
	// IPCDefaultIdleTimeoutMs 是连接的默认空闲超时时间（毫秒）。
	IPCDefaultIdleTimeoutMs = 60000
)

// ClientId 标识 IPC 服务器端的客户端连接。
type ClientId uint64

type ClientRecvDataCallback = func(data []byte) bool

type ServerSendDataCallback = func(clientId ClientId, data []byte) bool

type ServerRecvDataCallback = func(clientId ClientId, data []byte, sendCallback ServerSendDataCallback) *uint32
