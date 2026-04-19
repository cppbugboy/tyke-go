package common

const (
	DefaultTimeoutMs      = 5000
	DefaultBufferSize     = 4096
	DefaultThreadPoolSize = 4
	ProtocolHeaderSize    = 28

	AesGcmIvLen          = 12
	AesGcmTagLen         = 16
	Aes256KeyLen         = 32
	DefaultStubTimeoutMs = 30000
	HttpStatusNotFound   = 404
	HttpStatusTimeout    = 408
)

var ProtocolMagic = [4]byte{'T', 'Y', 'K', 'E'}

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

type ProtocolHeader struct {
	Magic       [4]byte
	MsgType     MessageType
	Reserved    [3]uint32
	MetadataLen uint32
	ContentLen  uint32
}
