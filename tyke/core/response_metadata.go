package core

import (
	"encoding/json"

	"github.com/tyke/tyke/tyke/common"
)

type ResponseMetadata struct {
	MetadataBase
	Status int    `json:"status"`
	Reason string `json:"reason"`
}

func NewResponseMetadata() ResponseMetadata {
	return ResponseMetadata{MetadataBase: NewMetadataBase()}
}

func (r ResponseMetadata) MarshalJSON() ([]byte, error) {
	raw := map[string]any{
		"module":       r.Module,
		"async_uuid":   r.AsyncUUID,
		"msg_uuid":     r.MsgUUID,
		"route":        r.Route,
		"content_type": r.ContentType,
		"timestamp":    r.Timestamp,
		"timeout":      r.Timeout,
		"status":       r.Status,
		"reason":       r.Reason,
	}
	for k, v := range r.HeadersMap {
		if _, exists := raw[k]; !exists {
			raw[k] = v.Value()
		}
	}
	return json.Marshal(raw)
}

func (r *ResponseMetadata) UnmarshalJSON(data []byte) error {
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
	r.Status = jsonIntField(raw, "status")
	r.Reason = jsonStringField(raw, "reason")
	if r.HeadersMap == nil {
		r.HeadersMap = make(map[string]common.JsonValueHolder)
	}
	for k, v := range raw {
		if !ResponseMetadataKnownKeys[k] {
			r.HeadersMap[k] = common.JsonToVariant(v)
		}
	}
	return nil
}

func (r *ResponseMetadata) GetStatus() int {
	return r.Status
}

func (r *ResponseMetadata) SetStatus(status int) *ResponseMetadata {
	r.Status = status
	return r
}

func (r *ResponseMetadata) GetReason() string {
	return r.Reason
}

func (r *ResponseMetadata) SetReason(reason string) *ResponseMetadata {
	r.Reason = reason
	return r
}

var ResponseMetadataKnownKeys = map[string]bool{
	"module": true, "async_uuid": true, "msg_uuid": true, "route": true,
	"content_type": true, "timestamp": true, "timeout": true, "status": true, "reason": true,
}

func (r *ResponseMetadata) FromJsonString(jsonStr string) error {
	return json.Unmarshal([]byte(jsonStr), r)
}
