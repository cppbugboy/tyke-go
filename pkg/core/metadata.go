// Copyright 2026 Tyke Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"encoding/json"
	"fmt"
)

// RequestMetadata 请求元数据结构。
//
// 请求元数据包含请求的描述信息，以JSON格式序列化传输。
// 元数据与内容分离，便于路由和过滤处理。
type RequestMetadata struct {
	// Module 模块名称，用于模块级别的路由和过滤
	Module string `json:"module"`
	// AsyncUUID 异步请求的目标UUID，用于响应路由
	AsyncUUID string `json:"async_uuid,omitempty"`
	// MsgUUID 消息唯一标识，用于请求追踪和响应匹配
	MsgUUID string `json:"msg_uuid"`
	// Route 路由路径，如 "/user/login"
	Route string `json:"route"`
	// ContentType 内容类型，如 "text"/"json"/"binary"
	ContentType string `json:"content_type"`
	// Timestamp 时间戳，格式为 "YYYY-MM-DD HH:MM:SS.mmm"
	Timestamp string `json:"timestamp"`
	// Headers 扩展头信息，用于传递自定义元数据
	Headers map[string]any `json:"headers,omitempty"`
}

// NewRequestMetadata 创建一个新的请求元数据实例。
//
// 返回值:
//   - *RequestMetadata: 新创建的请求元数据指针
func NewRequestMetadata() *RequestMetadata {
	return &RequestMetadata{
		Headers: make(map[string]any),
	}
}

// ToJsonString 将请求元数据序列化为JSON字符串。
//
// 返回值:
//   - string: JSON格式的字符串
//   - error: 序列化失败时返回错误
func (m *RequestMetadata) ToJsonString() (string, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("request metadata marshal failed: %w", err)
	}
	return string(data), nil
}

// FromJsonString 从JSON字符串反序列化请求元数据。
//
// 参数:
//   - jsonStr: JSON格式的字符串
//
// 返回值:
//   - error: 反序列化失败时返回错误
func (m *RequestMetadata) FromJsonString(jsonStr string) error {
	if err := json.Unmarshal([]byte(jsonStr), m); err != nil {
		return fmt.Errorf("request metadata unmarshal failed: %w", err)
	}
	if m.Headers == nil {
		m.Headers = make(map[string]any)
	}
	return nil
}

// ResponseMetadata 响应元数据结构。
//
// 响应元数据包含响应的描述信息和处理结果，
// 以JSON格式序列化传输。
type ResponseMetadata struct {
	// Module 模块名称
	Module string `json:"module"`
	// MsgUUID 消息唯一标识，与请求的MsgUUID对应
	MsgUUID string `json:"msg_uuid"`
	// Route 路由路径
	Route string `json:"route"`
	// ContentType 内容类型
	ContentType string `json:"content_type"`
	// Timestamp 时间戳
	Timestamp string `json:"timestamp"`
	// Status 状态码，如 200/404/500
	Status int `json:"status"`
	// Reason 状态原因描述
	Reason string `json:"reason"`
	// Headers 扩展头信息
	Headers map[string]any `json:"headers,omitempty"`
}

// NewResponseMetadata 创建一个新的响应元数据实例。
//
// 返回值:
//   - *ResponseMetadata: 新创建的响应元数据指针
func NewResponseMetadata() *ResponseMetadata {
	return &ResponseMetadata{
		Headers: make(map[string]any),
	}
}

// ToJsonString 将响应元数据序列化为JSON字符串。
//
// 返回值:
//   - string: JSON格式的字符串
//   - error: 序列化失败时返回错误
func (m *ResponseMetadata) ToJsonString() (string, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("response metadata marshal failed: %w", err)
	}
	return string(data), nil
}

// FromJsonString 从JSON字符串反序列化响应元数据。
//
// 参数:
//   - jsonStr: JSON格式的字符串
//
// 返回值:
//   - error: 反序列化失败时返回错误
func (m *ResponseMetadata) FromJsonString(jsonStr string) error {
	if err := json.Unmarshal([]byte(jsonStr), m); err != nil {
		return fmt.Errorf("response metadata unmarshal failed: %w", err)
	}
	if m.Headers == nil {
		m.Headers = make(map[string]any)
	}
	return nil
}
