package core

import (
	"encoding/json"

	"github.com/cppbugboy/tyke-go/tyke/common"
)

type RequestMetadata struct {
	MetadataBase
}

func NewRequestMetadata() RequestMetadata {
	return RequestMetadata{MetadataBase: NewMetadataBase()}
}

func (r RequestMetadata) MarshalJSON() ([]byte, error) {
	raw := map[string]any{
		"module":       r.Module,
		"async_uuid":   r.AsyncUUID,
		"msg_uuid":     r.MsgUUID,
		"route":        r.Route,
		"content_type": r.ContentType,
		"timestamp":    r.Timestamp,
		"timeout":      r.Timeout,
	}
	for k, v := range r.HeadersMap {
		if _, exists := raw[k]; !exists {
			raw[k] = v.Value()
		}
	}
	return json.Marshal(raw)
}

func (r *RequestMetadata) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.Module = jsonStringField(raw, "module")
	r.AsyncUUID = jsonStringField(raw, "async_uuid")
	r.MsgUUID = jsonStringField(raw, "msg_uuid")
	r.Route = jsonStringField(raw, "route")
	r.ContentType = jsonStringField(raw, "content_type")
	r.Timestamp = jsonStringField(raw, "timestamp")
	r.Timeout = jsonUint64Field(raw, "timeout")
	if r.HeadersMap == nil {
		r.HeadersMap = make(map[string]common.JsonValueHolder)
	}
	for k, v := range raw {
		if !RequestMetadataKnownKeys[k] {
			r.HeadersMap[k] = common.JsonToVariant(v)
		}
	}
	return nil
}

var RequestMetadataKnownKeys = map[string]bool{
	"module": true, "async_uuid": true, "msg_uuid": true,
	"route": true, "content_type": true, "timestamp": true, "timeout": true,
}

func (r *RequestMetadata) FromJsonString(jsonStr string) error {
	return json.Unmarshal([]byte(jsonStr), r)
}
