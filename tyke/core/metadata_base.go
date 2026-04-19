package core

import (
	"encoding/json"

	"github.com/tyke/tyke/tyke/common"
)

type MetadataBase struct {
	Module      string                      `json:"module"`
	AsyncUuid   string                      `json:"async_uuid"`
	MsgUuid     string                      `json:"msg_uuid"`
	Route       string                      `json:"route"`
	ContentType string                      `json:"content_type"`
	Timestamp   string                      `json:"timestamp"`
	HeadersMap  map[string]common.JsonValue `json:"-"`
}

func NewMetadataBase() MetadataBase {
	return MetadataBase{HeadersMap: make(map[string]common.JsonValue)}
}

func (m *MetadataBase) GetModule() string {
	return m.Module
}

func (m *MetadataBase) SetModule(module string) *MetadataBase {
	m.Module = module
	return m
}

func (m *MetadataBase) GetAsyncUuid() string {
	return m.AsyncUuid
}

func (m *MetadataBase) SetAsyncUuid(asyncUuid string) *MetadataBase {
	m.AsyncUuid = asyncUuid
	return m
}

func (m *MetadataBase) GetMsgUuid() string {
	return m.MsgUuid
}

func (m *MetadataBase) SetMsgUuid(msgUuid string) *MetadataBase {
	m.MsgUuid = msgUuid
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

func (m *MetadataBase) AddMetadata(key string, value common.JsonValue) common.BoolResult {
	if key == "" {
		return common.ErrBool("Metadata key cannot be empty")
	}
	if m.HeadersMap == nil {
		m.HeadersMap = make(map[string]common.JsonValue)
	}
	m.HeadersMap[key] = value
	return common.OkBool(true)
}

func (m *MetadataBase) GetMetadata(key string) (common.JsonValue, bool) {
	if m.HeadersMap == nil {
		return nil, false
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
