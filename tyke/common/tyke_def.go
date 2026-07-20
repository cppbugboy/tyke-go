// Package common 提供 Tyke 框架中使用的共享类型定义、常量和工具函数。
//
// 本文件定义核心 IPC 协议常量、内容/消息类型枚举以及协议头结构体。
package common

const (
	// DefaultTimeoutMs 默认请求超时时间（毫秒）。
	DefaultTimeoutMs = 5000
	// DefaultBufferSize 默认 I/O 缓冲区大小（字节）。
	DefaultBufferSize = 4096
	// DefaultThreadPoolSize 默认工作协程数量。
	DefaultThreadPoolSize = 4
	// ProtocolHeaderSize Tyke 协议头的固定字节大小。
	ProtocolHeaderSize = 28

	// DefaultStubTimeoutMs 请求存根（func/future）的默认超时时间。
	DefaultStubTimeoutMs = 30000
	// HttpStatusNotFound HTTP 404 状态码，用于路由未找到场景。
	HttpStatusNotFound = 404
	// HttpStatusTimeout HTTP 408 状态码，用于超时场景。
	HttpStatusTimeout = 408
)

// ProtocolMagic 是 Tyke 协议的魔数 ("TYKE")。
var ProtocolMagic = [4]byte{'T', 'Y', 'K', 'E'}

// ModuleName 是当前模块名称，通过 core.SetModuleName 设置。
var ModuleName string

// StatusCode 表示请求/响应状态码。
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

// ContentType 定义消息内容的编码格式。
type ContentType int

const (
	ContentTypeText   ContentType = iota // 纯文本
	ContentTypeJson                      // JSON 编码
	ContentTypeBinary                    // 原始二进制
)

// ContentTypeMap 将 ContentType 值映射到其字符串表示形式。
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

// MessageType 定义 IPC 消息类型枚举。它区分同步请求、
// 异步变体（发后即忘、回调、Future）及其对应的响应类型。
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

// ProtocolHeader 是固定大小的 Tyke IPC 协议头（28 字节）。
// 包含魔数、消息类型、保留字段以及后续
// 元数据和内容部分的字节长度。
type ProtocolHeader struct {
	Magic       [4]byte     // 协议魔数，必须等于 ProtocolMagic
	MsgType     MessageType // 消息类型（请求、响应等）
	Reserved    [3]uint32   // 保留供将来使用
	MetadataLen uint32      // JSON 元数据部分的字节长度
	ContentLen  uint32      // 内容/载荷部分的字节长度
}
