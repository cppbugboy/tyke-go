// Copyright 2026 Tyke Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package common 提供Tyke框架的公共类型定义和常量。
//
// 本包包含协议头定义、消息类型枚举、内容类型枚举等核心类型，
// 是Tyke框架的基础依赖包。所有其他包都依赖于本包定义的类型。
//
// 主要类型:
//   - ProtocolHeader: Tyke协议头部结构，固定28字节
//   - ContentType: 内容类型枚举（Text/Json/Binary）
//   - MessageType: 消息类型枚举（请求/响应模式）
//
// 作者: Nick
// 创建日期: 2026-04-17
// 最后修改: 2026-04-17
package common

// 缓冲区和超时常量定义。
const (
	// DefaultBufferSize 默认缓冲区大小，用于IPC数据读写。
	DefaultBufferSize = 4096
	// DefaultTimeoutMs 默认超时时间（毫秒），用于IPC连接和读写操作。
	DefaultTimeoutMs = 5000
	// DefaultWorkerPoolSize 默认工作池大小，用于异步任务处理。
	DefaultWorkerPoolSize = 4
	// MaxEpollEvents epoll最大事件数，用于Linux平台的事件循环。
	MaxEpollEvents = 100
	// MaxEvent 最大事件数，用于事件处理队列。
	MaxEvent = 128
	// ProtocolHeaderSize 协议头大小，固定为28字节。
	ProtocolHeaderSize = 28
)

// ProtocolMagic 协议魔数，用于标识Tyke协议数据包。
// 固定为 "TYKE" 四个ASCII字符。
var ProtocolMagic = [4]byte{'T', 'Y', 'K', 'E'}

// ContentType 定义内容类型枚举。
//
// 内容类型用于标识请求和响应中携带的数据格式，
// 便于接收方正确解析数据内容。
type ContentType int

const (
	// ContentTypeText 文本类型，纯文本数据。
	ContentTypeText ContentType = iota
	// ContentTypeJson JSON类型，JSON格式数据。
	ContentTypeJson
	// ContentTypeBinary 二进制类型，原始二进制数据。
	ContentTypeBinary
)

// ContentTypeMap 提供ContentType到字符串的映射。
// 用于将枚举值转换为可读的字符串表示。
var ContentTypeMap = map[ContentType]string{
	ContentTypeText:   "text",
	ContentTypeJson:   "json",
	ContentTypeBinary: "binary",
}

// ContentTypeReverseMap 提供字符串到ContentType的反向映射。
// 用于从字符串解析出对应的ContentType枚举值。
var ContentTypeReverseMap = map[string]ContentType{
	"text":   ContentTypeText,
	"json":   ContentTypeJson,
	"binary": ContentTypeBinary,
}

// MessageType 定义消息类型枚举。
//
// 消息类型用于区分不同的请求和响应模式，
// 包括同步请求、异步请求（三种方式）和对应的响应类型。
type MessageType int

const (
	// MessageTypeNone 无效消息类型。
	MessageTypeNone MessageType = 0
	// MessageTypeRequest 同步请求，发送方阻塞等待响应。
	MessageTypeRequest MessageType = 1
	// MessageTypeRequestAsync 异步请求（无回调），发送后不等待响应。
	MessageTypeRequestAsync MessageType = 2
	// MessageTypeRequestAsyncFunc 异步请求（回调方式），响应到达时调用回调函数。
	MessageTypeRequestAsyncFunc MessageType = 3
	// MessageTypeRequestAsyncFuture 异步请求（Future方式），通过channel接收响应。
	MessageTypeRequestAsyncFuture MessageType = 4
	// MessageTypeResponse 同步响应，对应同步请求的响应。
	MessageTypeResponse MessageType = 5
	// MessageTypeResponseAsync 异步响应（无回调）。
	MessageTypeResponseAsync MessageType = 6
	// MessageTypeResponseAsyncFunc 异步响应（回调方式）。
	MessageTypeResponseAsyncFunc MessageType = 7
	// MessageTypeResponseAsyncFuture 异步响应（Future方式）。
	MessageTypeResponseAsyncFuture MessageType = 8
)

// ProtocolHeader 定义Tyke协议的头部结构。
//
// 协议头固定为28字节，采用小端字节序序列化。
// 每个Tyke数据包都以协议头开始，后跟元数据JSON和内容二进制数据。
//
// 数据包格式:
//
//	[协议头 28字节][元数据JSON 变长][内容二进制 变长]
//
// 字段说明:
//   - Magic: 协议魔数，固定为 "TYKE"，用于快速识别数据包
//   - MsgType: 消息类型，标识请求/响应模式
//   - Reserved: 保留字段，共12字节，用于未来扩展
//   - MetadataLen: 元数据JSON长度，用于解析元数据边界
//   - ContentLen: 内容二进制长度，用于解析内容边界
type ProtocolHeader struct {
	// Magic 协议魔数，固定为 "TYKE"。
	Magic [4]byte
	// MsgType 消息类型，参见MessageType枚举。
	MsgType MessageType
	// Reserved 保留字段，共3个uint32，用于未来扩展。
	Reserved [3]uint32
	// MetadataLen 元数据JSON长度（字节）。
	MetadataLen uint32
	// ContentLen 内容二进制长度（字节）。
	ContentLen uint32
}
