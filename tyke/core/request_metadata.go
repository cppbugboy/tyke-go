package core

import (
	"encoding/json"

	"github.com/tyke/tyke/tyke/common"
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
		"async_uuid":   r.AsyncUuid,
		"msg_uuid":     r.MsgUuid,
		"route":        r.Route,
		"content_type": r.ContentType,
		"timestamp":    r.Timestamp,
	}
	for k, v := range r.HeadersMap {
		if _, exists := raw[k]; !exists {
			raw[k] = v
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
	r.AsyncUuid = jsonStringField(raw, "async_uuid")
	r.MsgUuid = jsonStringField(raw, "msg_uuid")
	r.Route = jsonStringField(raw, "route")
	r.ContentType = jsonStringField(raw, "content_type")
	r.Timestamp = jsonStringField(raw, "timestamp")
	if r.HeadersMap == nil {
		r.HeadersMap = make(map[string]common.JsonValue)
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
	"route": true, "content_type": true, "timestamp": true,
}

func (r *RequestMetadata) FromJsonString(jsonStr string) error {
	return json.Unmarshal([]byte(jsonStr), r)
}
