package core

import (
	"encoding/json"

	"tyke-go/common"
)

// MetadataBase 提供请求/响应元数据的公共字段和方法。
type MetadataBase struct {
	Module      string                            `json:"module"`
	AsyncUUID   string                            `json:"async_uuid"`
	MsgUUID     string                            `json:"msg_uuid"`
	Route       string                            `json:"route"`
	ContentType string                            `json:"content_type"`
	Timestamp   string                            `json:"timestamp"`
	Timeout     uint64                            `json:"timeout"`
	HeadersMap  map[string]common.JsonValueHolder `json:"-"`
}

func NewMetadataBase() MetadataBase {
	return MetadataBase{HeadersMap: make(map[string]common.JsonValueHolder)}
}

func (m *MetadataBase) GetModule() string {
	return m.Module
}

func (m *MetadataBase) SetModule(module string) *MetadataBase {
	m.Module = module
	return m
}

func (m *MetadataBase) GetAsyncUUID() string {
	return m.AsyncUUID
}

func (m *MetadataBase) SetAsyncUUID(asyncUuid string) *MetadataBase {
	m.AsyncUUID = asyncUuid
	return m
}

func (m *MetadataBase) GetMsgUUID() string {
	return m.MsgUUID
}

func (m *MetadataBase) SetMsgUUID(msgUuid string) *MetadataBase {
	m.MsgUUID = msgUuid
	return m
}

func (m *MetadataBase) GetRoute() string {
	return m.Route
}

func (m *MetadataBase) SetRoute(route string) *MetadataBase {
	m.Route = route
	return m
}

func (m *MetadataBase) GetContentType() string {
	return m.ContentType
}

func (m *MetadataBase) SetContentType(contentType string) *MetadataBase {
	m.ContentType = contentType
	return m
}

func (m *MetadataBase) GetTimestamp() string {
	return m.Timestamp
}

func (m *MetadataBase) SetTimestamp(timestamp string) *MetadataBase {
	m.Timestamp = timestamp
	return m
}

func (m *MetadataBase) GetTimeout() uint64 {
	return m.Timeout
}

func (m *MetadataBase) SetTimeout(timeout uint64) *MetadataBase {
	m.Timeout = timeout
	return m
}

func (m *MetadataBase) AddMetadata(key string, value common.JsonValueHolder) common.BoolResult {
	if key == "" {
		return common.ErrBool("Metadata key cannot be empty")
	}
	if m.HeadersMap == nil {
		m.HeadersMap = make(map[string]common.JsonValueHolder)
	}
	m.HeadersMap[key] = value
	return common.OkBool(true)
}

func (m *MetadataBase) GetMetadata(key string) (common.JsonValueHolder, bool) {
	if m.HeadersMap == nil {
		return common.JsonValueHolder{}, false
	}
	v, ok := m.HeadersMap[key]
	return v, ok
}

func jsonStringField(raw map[string]json.RawMessage, key string) string {
	if v, ok := raw[key]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			return s
		}
	}
	return ""
}

func jsonIntField(raw map[string]json.RawMessage, key string) int {
	if v, ok := raw[key]; ok {
		var n int
		if json.Unmarshal(v, &n) == nil {
			return n
		}
	}
	return 0
}

func jsonUint64Field(raw map[string]json.RawMessage, key string) uint64 {
	if v, ok := raw[key]; ok {
		var n uint64
		if json.Unmarshal(v, &n) == nil {
			return n
		}
	}
	return 0
}
