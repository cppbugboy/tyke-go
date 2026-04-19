package core

import "encoding/json"

type RequestMetadata struct {
	MetadataBase
}

func NewRequestMetadata() RequestMetadata {
	return RequestMetadata{MetadataBase: NewMetadataBase()}
}

func (r RequestMetadata) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Module      string `json:"module"`
		AsyncUuid   string `json:"async_uuid"`
		MsgUuid     string `json:"msg_uuid"`
		Route       string `json:"route"`
		ContentType string `json:"content_type"`
		Timestamp   string `json:"timestamp"`
	}{
		Module:      r.Module,
		AsyncUuid:   r.AsyncUuid,
		MsgUuid:     r.MsgUuid,
		Route:       r.Route,
		ContentType: r.ContentType,
		Timestamp:   r.Timestamp,
	})
}

func (r *RequestMetadata) UnmarshalJSON(data []byte) error {
	var aux struct {
		Module      string `json:"module"`
		AsyncUuid   string `json:"async_uuid"`
		MsgUuid     string `json:"msg_uuid"`
		Route       string `json:"route"`
		ContentType string `json:"content_type"`
		Timestamp   string `json:"timestamp"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	r.Module = aux.Module
	r.AsyncUuid = aux.AsyncUuid
	r.MsgUuid = aux.MsgUuid
	r.Route = aux.Route
	r.ContentType = aux.ContentType
	r.Timestamp = aux.Timestamp
	return nil
}

var RequestMetadataKnownKeys = map[string]bool{
	"module": true, "async_uuid": true, "msg_uuid": true,
	"route": true, "content_type": true, "timestamp": true,
}

func (r *RequestMetadata) FromJsonString(jsonStr string) error {
	return r.MetadataBase.FromJsonString(jsonStr, RequestMetadataKnownKeys)
}
