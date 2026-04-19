package core

import "encoding/json"

type ResponseMetadata struct {
	MetadataBase
	Status int    `json:"status"`
	Reason string `json:"reason"`
}

func NewResponseMetadata() ResponseMetadata {
	return ResponseMetadata{MetadataBase: NewMetadataBase()}
}

func (r ResponseMetadata) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Module      string `json:"module"`
		MsgUuid     string `json:"msg_uuid"`
		Route       string `json:"route"`
		ContentType string `json:"content_type"`
		Timestamp   string `json:"timestamp"`
		Status      int    `json:"status"`
		Reason      string `json:"reason"`
	}{
		Module:      r.Module,
		MsgUuid:     r.MsgUuid,
		Route:       r.Route,
		ContentType: r.ContentType,
		Timestamp:   r.Timestamp,
		Status:      r.Status,
		Reason:      r.Reason,
	})
}

func (r *ResponseMetadata) UnmarshalJSON(data []byte) error {
	var aux struct {
		Module      string `json:"module"`
		MsgUuid     string `json:"msg_uuid"`
		Route       string `json:"route"`
		ContentType string `json:"content_type"`
		Timestamp   string `json:"timestamp"`
		Status      int    `json:"status"`
		Reason      string `json:"reason"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	r.Module = aux.Module
	r.MsgUuid = aux.MsgUuid
	r.Route = aux.Route
	r.ContentType = aux.ContentType
	r.Timestamp = aux.Timestamp
	r.Status = aux.Status
	r.Reason = aux.Reason
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
	"module": true, "msg_uuid": true, "route": true,
	"content_type": true, "timestamp": true, "status": true, "reason": true,
}

func (r *ResponseMetadata) FromJsonString(jsonStr string) error {
	return r.MetadataBase.FromJsonString(jsonStr, ResponseMetadataKnownKeys)
}
