package common

const (
	DefaultTimeoutMs      = 5000
	DefaultBufferSize     = 4096
	DefaultThreadPoolSize = 4
	// ProtocolHeaderSize 是协议头的固定字节大小。
	ProtocolHeaderSize = 28

	AesGcmIvLen          = 12
	AesGcmTagLen         = 16
	Aes256KeyLen         = 32
	DefaultStubTimeoutMs = 30000
	HttpStatusNotFound   = 404
	HttpStatusTimeout    = 408
)

// ProtocolMagic 是 Tyke 协议的魔数标识。
var ProtocolMagic = [4]byte{'T', 'Y', 'K', 'E'}

var ModuleName string

// ContentType 定义了消息内容的编码格式。
type StatusCode int

const (
	StatusNone          StatusCode = 0
	StatusSuccess       StatusCode = 1
	StatusFailure       StatusCode = 2
	StatusTimeout       StatusCode = 3
	StatusMetadataError StatusCode = 4
	StatusContentError  StatusCode = 5
	StatusRouteError    StatusCode = 6
	StatusModuleError   StatusCode = 7
	StatusInternalError StatusCode = 8
	StatusUnavailable   StatusCode = 9
	StatusUnknownError  StatusCode = 10
)

type ContentType int

const (
	ContentTypeText ContentType = iota
	ContentTypeJson
	ContentTypeBinary
)

var ContentTypeMap = map[ContentType]string{
	ContentTypeText:   "text",
	ContentTypeJson:   "json",
	ContentTypeBinary: "binary",
}

// StringToContentType 提供从字符串到 ContentType 的反向映射。
var StringToContentType = map[string]ContentType{
	"text":   ContentTypeText,
	"json":   ContentTypeJson,
	"binary": ContentTypeBinary,
}

// MessageType 定义了 IPC 消息的类型枚举。
type MessageType uint32

const (
	MessageTypeNone                MessageType = 0
	MessageTypeRequest             MessageType = 1
	MessageTypeRequestAsync        MessageType = 2
	MessageTypeRequestAsyncFunc    MessageType = 3
	MessageTypeRequestAsyncFuture  MessageType = 4
	MessageTypeResponse            MessageType = 5
	MessageTypeResponseAsync       MessageType = 6
	MessageTypeResponseAsyncFunc   MessageType = 7
	MessageTypeResponseAsyncFuture MessageType = 8
)

// ProtocolHeader 定义了 Tyke IPC 协议的头部结构，包含魔数、消息类型和长度信息。
type ProtocolHeader struct {
	Magic       [4]byte
	MsgType     MessageType
	Reserved    [3]uint32
	MetadataLen uint32
	ContentLen  uint32
}
