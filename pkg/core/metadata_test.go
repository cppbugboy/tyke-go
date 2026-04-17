package core

import "testing"

func TestRequestMetadataJsonRoundTrip(t *testing.T) {
	m := NewRequestMetadata()
	m.Module = "user"
	m.MsgUUID = "550e8400-e29b-41d4-a716-446655440000"
	m.Route = "/user/login"
	m.ContentType = "json"
	m.Timestamp = "2026-04-17 12:00:00.000"
	m.Headers["trace_id"] = "abc123"

	jsonStr, err := m.ToJsonString()
	if err != nil {
		t.Fatalf("ToJsonString failed: %v", err)
	}

	m2 := NewRequestMetadata()
	if err := m2.FromJsonString(jsonStr); err != nil {
		t.Fatalf("FromJsonString failed: %v", err)
	}

	if m2.Module != m.Module {
		t.Errorf("Module mismatch: got %q, want %q", m2.Module, m.Module)
	}
	if m2.MsgUUID != m.MsgUUID {
		t.Errorf("MsgUUID mismatch: got %q, want %q", m2.MsgUUID, m.MsgUUID)
	}
	if m2.Route != m.Route {
		t.Errorf("Route mismatch: got %q, want %q", m2.Route, m.Route)
	}
	if m2.ContentType != m.ContentType {
		t.Errorf("ContentType mismatch: got %q, want %q", m2.ContentType, m.ContentType)
	}
}

func TestResponseMetadataJsonRoundTrip(t *testing.T) {
	m := NewResponseMetadata()
	m.Module = "user"
	m.MsgUUID = "550e8400-e29b-41d4-a716-446655440000"
	m.Route = "/user/login"
	m.ContentType = "json"
	m.Timestamp = "2026-04-17 12:00:00.000"
	m.Status = 200
	m.Reason = "OK"
	m.Headers["trace_id"] = "abc123"

	jsonStr, err := m.ToJsonString()
	if err != nil {
		t.Fatalf("ToJsonString failed: %v", err)
	}

	m2 := NewResponseMetadata()
	if err := m2.FromJsonString(jsonStr); err != nil {
		t.Fatalf("FromJsonString failed: %v", err)
	}

	if m2.Status != m.Status {
		t.Errorf("Status mismatch: got %d, want %d", m2.Status, m.Status)
	}
	if m2.Reason != m.Reason {
		t.Errorf("Reason mismatch: got %q, want %q", m2.Reason, m.Reason)
	}
}

func TestRequestMetadataEmptyHeaders(t *testing.T) {
	m := NewRequestMetadata()
	m.Module = "test"
	m.MsgUUID = "uuid"
	m.Route = "/test"
	m.ContentType = "text"
	m.Timestamp = "2026-01-01 00:00:00.000"

	jsonStr, err := m.ToJsonString()
	if err != nil {
		t.Fatalf("ToJsonString failed: %v", err)
	}

	m2 := NewRequestMetadata()
	if err := m2.FromJsonString(jsonStr); err != nil {
		t.Fatalf("FromJsonString failed: %v", err)
	}

	if m2.Headers == nil {
		t.Error("Headers should not be nil after FromJsonString")
	}
}
