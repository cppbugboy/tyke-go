package core

import (
	"encoding/json"
	"testing"
)

func TestRequestMetadataMarshalNoHeadersMap(t *testing.T) {
	m := NewRequestMetadata()
	m.Module = "user_service"
	m.Route = "/api/user/login"
	m.MsgUuid = "test-uuid-123"
	m.AsyncUuid = "async-uuid-456"
	m.ContentType = "json"
	m.Timestamp = "2026-04-19T00:00:00Z"

	data, err := json.Marshal(&m)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal result failed: %v", err)
	}

	if result["module"] != "user_service" {
		t.Errorf("expected module=user_service, got %v", result["module"])
	}
	if result["route"] != "/api/user/login" {
		t.Errorf("expected route=/api/user/login, got %v", result["route"])
	}
	if result["msg_uuid"] != "test-uuid-123" {
		t.Errorf("expected msg_uuid=test-uuid-123, got %v", result["msg_uuid"])
	}
	if result["async_uuid"] != "async-uuid-456" {
		t.Errorf("expected async_uuid=async-uuid-456, got %v", result["async_uuid"])
	}
	if result["content_type"] != "json" {
		t.Errorf("expected content_type=json, got %v", result["content_type"])
	}
	if result["timestamp"] != "2026-04-19T00:00:00Z" {
		t.Errorf("expected timestamp=2026-04-19T00:00:00Z, got %v", result["timestamp"])
	}
}

func TestRequestMetadataMarshalWithHeadersMap(t *testing.T) {
	m := NewRequestMetadata()
	m.Module = "data_service"
	m.Route = "/api/process"
	m.HeadersMap["source"] = "go_client"
	m.HeadersMap["version"] = "1.0"
	m.HeadersMap["priority"] = int64(3)

	data, err := json.Marshal(&m)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal result failed: %v", err)
	}

	if result["source"] != "go_client" {
		t.Errorf("expected source=go_client, got %v", result["source"])
	}
	if result["version"] != "1.0" {
		t.Errorf("expected version=1.0, got %v", result["version"])
	}
	if result["module"] != "data_service" {
		t.Errorf("expected module=data_service, got %v", result["module"])
	}
}

func TestRequestMetadataMarshalHeadersMapKeyConflict(t *testing.T) {
	m := NewRequestMetadata()
	m.Module = "real_module"
	m.HeadersMap["module"] = "fake_module"

	data, err := json.Marshal(&m)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal result failed: %v", err)
	}

	if result["module"] != "real_module" {
		t.Errorf("known field should take priority, expected module=real_module, got %v", result["module"])
	}
}

func TestRequestMetadataUnmarshalKnownFields(t *testing.T) {
	jsonStr := `{
		"module": "user_service",
		"async_uuid": "async-123",
		"msg_uuid": "uuid-456",
		"route": "/api/login",
		"content_type": "json",
		"timestamp": "2026-04-19T00:00:00Z"
	}`

	var m RequestMetadata
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if m.Module != "user_service" {
		t.Errorf("expected Module=user_service, got %s", m.Module)
	}
	if m.AsyncUuid != "async-123" {
		t.Errorf("expected AsyncUuid=async-123, got %s", m.AsyncUuid)
	}
	if m.MsgUuid != "uuid-456" {
		t.Errorf("expected MsgUuid=uuid-456, got %s", m.MsgUuid)
	}
	if m.Route != "/api/login" {
		t.Errorf("expected Route=/api/login, got %s", m.Route)
	}
	if m.ContentType != "json" {
		t.Errorf("expected ContentType=json, got %s", m.ContentType)
	}
	if m.Timestamp != "2026-04-19T00:00:00Z" {
		t.Errorf("expected Timestamp=2026-04-19T00:00:00Z, got %s", m.Timestamp)
	}
}

func TestRequestMetadataUnmarshalUnknownFieldsToHeadersMap(t *testing.T) {
	jsonStr := `{
		"module": "data_service",
		"route": "/api/process",
		"source": "go_client",
		"version": "1.0",
		"priority": 3
	}`

	var m RequestMetadata
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if m.Module != "data_service" {
		t.Errorf("expected Module=data_service, got %s", m.Module)
	}
	if m.Route != "/api/process" {
		t.Errorf("expected Route=/api/process, got %s", m.Route)
	}

	if v, ok := m.HeadersMap["source"]; !ok || v != "go_client" {
		t.Errorf("expected HeadersMap[source]=go_client, got %v", v)
	}
	if v, ok := m.HeadersMap["version"]; !ok || v != "1.0" {
		t.Errorf("expected HeadersMap[version]=1.0, got %v", v)
	}
	if v, ok := m.HeadersMap["priority"]; !ok {
		t.Errorf("expected HeadersMap[priority] to exist")
	} else {
		switch pv := v.(type) {
		case int64:
			if pv != 3 {
				t.Errorf("expected HeadersMap[priority]=3, got %d", pv)
			}
		case float64:
			if pv != 3 {
				t.Errorf("expected HeadersMap[priority]=3, got %f", pv)
			}
		default:
			t.Errorf("expected HeadersMap[priority] to be numeric, got %T", v)
		}
	}

	if _, ok := m.HeadersMap["module"]; ok {
		t.Error("known field 'module' should not be in HeadersMap")
	}
	if _, ok := m.HeadersMap["route"]; ok {
		t.Error("known field 'route' should not be in HeadersMap")
	}
}

func TestRequestMetadataRoundTrip(t *testing.T) {
	original := NewRequestMetadata()
	original.Module = "test_module"
	original.Route = "/test/route"
	original.MsgUuid = "uuid-roundtrip"
	original.AsyncUuid = "async-roundtrip"
	original.ContentType = "json"
	original.Timestamp = "2026-04-19T12:00:00Z"
	original.HeadersMap["custom_key"] = "custom_value"
	original.HeadersMap["count"] = int64(42)

	data, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var restored RequestMetadata
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if restored.Module != original.Module {
		t.Errorf("Module mismatch: %s != %s", restored.Module, original.Module)
	}
	if restored.Route != original.Route {
		t.Errorf("Route mismatch: %s != %s", restored.Route, original.Route)
	}
	if restored.MsgUuid != original.MsgUuid {
		t.Errorf("MsgUuid mismatch: %s != %s", restored.MsgUuid, original.MsgUuid)
	}
	if restored.AsyncUuid != original.AsyncUuid {
		t.Errorf("AsyncUuid mismatch: %s != %s", restored.AsyncUuid, original.AsyncUuid)
	}
	if restored.ContentType != original.ContentType {
		t.Errorf("ContentType mismatch: %s != %s", restored.ContentType, original.ContentType)
	}
	if restored.Timestamp != original.Timestamp {
		t.Errorf("Timestamp mismatch: %s != %s", restored.Timestamp, original.Timestamp)
	}
	if v, ok := restored.HeadersMap["custom_key"]; !ok || v != "custom_value" {
		t.Errorf("HeadersMap[custom_key] mismatch: %v", v)
	}
}

func TestRequestMetadataFromJsonString(t *testing.T) {
	jsonStr := `{
		"module": "from_string_module",
		"route": "/from/string",
		"extra_field": "extra_value"
	}`

	var m RequestMetadata
	if err := m.FromJsonString(jsonStr); err != nil {
		t.Fatalf("FromJsonString failed: %v", err)
	}

	if m.Module != "from_string_module" {
		t.Errorf("expected Module=from_string_module, got %s", m.Module)
	}
	if v, ok := m.HeadersMap["extra_field"]; !ok || v != "extra_value" {
		t.Errorf("expected HeadersMap[extra_field]=extra_value, got %v", v)
	}
}

func TestResponseMetadataMarshalNoHeadersMap(t *testing.T) {
	m := NewResponseMetadata()
	m.Module = "user_service"
	m.Route = "/api/user/login"
	m.MsgUuid = "resp-uuid-123"
	m.ContentType = "json"
	m.Timestamp = "2026-04-19T00:00:00Z"
	m.Status = 200
	m.Reason = "OK"

	data, err := json.Marshal(&m)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal result failed: %v", err)
	}

	if result["module"] != "user_service" {
		t.Errorf("expected module=user_service, got %v", result["module"])
	}
	if result["status"] != float64(200) {
		t.Errorf("expected status=200, got %v", result["status"])
	}
	if result["reason"] != "OK" {
		t.Errorf("expected reason=OK, got %v", result["reason"])
	}
	if _, exists := result["async_uuid"]; exists {
		t.Error("async_uuid should not be present in ResponseMetadata")
	}
}

func TestResponseMetadataMarshalWithHeadersMap(t *testing.T) {
	m := NewResponseMetadata()
	m.Module = "data_service"
	m.Status = 200
	m.HeadersMap["trace_id"] = "abc-123"
	m.HeadersMap["server_version"] = "2.0"

	data, err := json.Marshal(&m)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal result failed: %v", err)
	}

	if result["trace_id"] != "abc-123" {
		t.Errorf("expected trace_id=abc-123, got %v", result["trace_id"])
	}
	if result["server_version"] != "2.0" {
		t.Errorf("expected server_version=2.0, got %v", result["server_version"])
	}
	if result["status"] != float64(200) {
		t.Errorf("expected status=200, got %v", result["status"])
	}
}

func TestResponseMetadataUnmarshalKnownFields(t *testing.T) {
	jsonStr := `{
		"module": "user_service",
		"msg_uuid": "resp-uuid-789",
		"route": "/api/login",
		"content_type": "json",
		"timestamp": "2026-04-19T01:00:00Z",
		"status": 404,
		"reason": "Not Found"
	}`

	var m ResponseMetadata
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if m.Module != "user_service" {
		t.Errorf("expected Module=user_service, got %s", m.Module)
	}
	if m.Status != 404 {
		t.Errorf("expected Status=404, got %d", m.Status)
	}
	if m.Reason != "Not Found" {
		t.Errorf("expected Reason=Not Found, got %s", m.Reason)
	}
	if m.MsgUuid != "resp-uuid-789" {
		t.Errorf("expected MsgUuid=resp-uuid-789, got %s", m.MsgUuid)
	}
}

func TestResponseMetadataUnmarshalUnknownFieldsToHeadersMap(t *testing.T) {
	jsonStr := `{
		"module": "data_service",
		"status": 200,
		"reason": "OK",
		"trace_id": "xyz-999",
		"server_region": "us-east"
	}`

	var m ResponseMetadata
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if m.Status != 200 {
		t.Errorf("expected Status=200, got %d", m.Status)
	}
	if v, ok := m.HeadersMap["trace_id"]; !ok || v != "xyz-999" {
		t.Errorf("expected HeadersMap[trace_id]=xyz-999, got %v", v)
	}
	if v, ok := m.HeadersMap["server_region"]; !ok || v != "us-east" {
		t.Errorf("expected HeadersMap[server_region]=us-east, got %v", v)
	}
	if _, ok := m.HeadersMap["status"]; ok {
		t.Error("known field 'status' should not be in HeadersMap")
	}
	if _, ok := m.HeadersMap["reason"]; ok {
		t.Error("known field 'reason' should not be in HeadersMap")
	}
}

func TestResponseMetadataRoundTrip(t *testing.T) {
	original := NewResponseMetadata()
	original.Module = "test_module"
	original.Route = "/test/response"
	original.MsgUuid = "resp-roundtrip"
	original.ContentType = "json"
	original.Timestamp = "2026-04-19T12:00:00Z"
	original.Status = 200
	original.Reason = "OK"
	original.HeadersMap["custom_key"] = "custom_value"

	data, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var restored ResponseMetadata
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if restored.Module != original.Module {
		t.Errorf("Module mismatch: %s != %s", restored.Module, original.Module)
	}
	if restored.Status != original.Status {
		t.Errorf("Status mismatch: %d != %d", restored.Status, original.Status)
	}
	if restored.Reason != original.Reason {
		t.Errorf("Reason mismatch: %s != %s", restored.Reason, original.Reason)
	}
	if v, ok := restored.HeadersMap["custom_key"]; !ok || v != "custom_value" {
		t.Errorf("HeadersMap[custom_key] mismatch: %v", v)
	}
}

func TestResponseMetadataFromJsonString(t *testing.T) {
	jsonStr := `{
		"module": "from_string_module",
		"status": 500,
		"reason": "Internal Error",
		"debug_info": "stack_trace_here"
	}`

	var m ResponseMetadata
	if err := m.FromJsonString(jsonStr); err != nil {
		t.Fatalf("FromJsonString failed: %v", err)
	}

	if m.Status != 500 {
		t.Errorf("expected Status=500, got %d", m.Status)
	}
	if m.Reason != "Internal Error" {
		t.Errorf("expected Reason=Internal Error, got %s", m.Reason)
	}
	if v, ok := m.HeadersMap["debug_info"]; !ok || v != "stack_trace_here" {
		t.Errorf("expected HeadersMap[debug_info]=stack_trace_here, got %v", v)
	}
}
