package core

import (
	"testing"

	"github.com/tyke/tyke/pkg/common"
)

func TestEncodeDecodeRequest(t *testing.T) {
	req := AcquireRequest()
	defer ReleaseRequest(req)

	req.SetModule("test")
	req.SetRoute("/test/hello")
	req.SetContent(common.ContentTypeText, []byte("hello world"))
	req.msgType = common.MessageTypeRequest
	req.metadata.MsgUUID = "550e8400-e29b-41d4-a716-446655440000"
	req.metadata.Timestamp = "2026-04-17 12:00:00.000"

	encoded, err := EncodeRequest(req)
	if err != nil {
		t.Fatalf("EncodeRequest failed: %v", err)
	}

	if len(encoded) <= common.ProtocolHeaderSize {
		t.Errorf("encoded data too small: %d", len(encoded))
	}

	decoded, dataSize, err := DecodeRequest(encoded)
	if err != nil {
		t.Fatalf("DecodeRequest failed: %v", err)
	}
	defer ReleaseRequest(decoded)

	if dataSize != uint32(len(encoded)) {
		t.Errorf("dataSize = %d, want %d", dataSize, len(encoded))
	}

	if decoded.GetModule() != "test" {
		t.Errorf("module = %q, want %q", decoded.GetModule(), "test")
	}
	if decoded.GetRoute() != "/test/hello" {
		t.Errorf("route = %q, want %q", decoded.GetRoute(), "/test/hello")
	}

	ct, content := decoded.GetContent()
	if ct != "text" {
		t.Errorf("content_type = %q, want %q", ct, "text")
	}
	if string(content) != "hello world" {
		t.Errorf("content = %q, want %q", string(content), "hello world")
	}
}

func TestEncodeDecodeResponse(t *testing.T) {
	resp := AcquireResponse()
	defer ReleaseResponse(resp)

	resp.SetModule("test")
	resp.SetRoute("/test/hello")
	resp.SetResult(200, "OK")
	resp.SetContent(common.ContentTypeJson, []byte(`{"code":0}`))
	resp.msgType = common.MessageTypeResponse
	resp.metadata.MsgUUID = "550e8400-e29b-41d4-a716-446655440000"
	resp.metadata.Timestamp = "2026-04-17 12:00:00.000"

	encoded, err := EncodeResponse(resp)
	if err != nil {
		t.Fatalf("EncodeResponse failed: %v", err)
	}

	decoded, dataSize, err := DecodeResponse(encoded)
	if err != nil {
		t.Fatalf("DecodeResponse failed: %v", err)
	}
	defer ReleaseResponse(decoded)

	if dataSize != uint32(len(encoded)) {
		t.Errorf("dataSize = %d, want %d", dataSize, len(encoded))
	}

	status, reason := decoded.GetResult()
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
	if reason != "OK" {
		t.Errorf("reason = %q, want %q", reason, "OK")
	}

	ct, content := decoded.GetContent()
	if ct != "json" {
		t.Errorf("content_type = %q, want %q", ct, "json")
	}
	if string(content) != `{"code":0}` {
		t.Errorf("content = %q, want %q", string(content), `{"code":0}`)
	}
}

func TestRequestPoolAcquireRelease(t *testing.T) {
	req := AcquireRequest()
	if req == nil {
		t.Fatal("AcquireRequest returned nil")
	}
	req.SetModule("test")
	ReleaseRequest(req)

	req2 := AcquireRequest()
	if req2.GetModule() != "" {
		t.Errorf("pooled request not reset: module = %q", req2.GetModule())
	}
	ReleaseRequest(req2)
}

func TestResponsePoolAcquireRelease(t *testing.T) {
	resp := AcquireResponse()
	if resp == nil {
		t.Fatal("AcquireResponse returned nil")
	}
	resp.SetResult(200, "OK")
	ReleaseResponse(resp)

	resp2 := AcquireResponse()
	status, _ := resp2.GetResult()
	if status != 0 {
		t.Errorf("pooled response not reset: status = %d", status)
	}
	ReleaseResponse(resp2)
}

func TestRequestChainAPI(t *testing.T) {
	req := AcquireRequest()
	defer ReleaseRequest(req)

	req.SetModule("user").SetRoute("/user/login").SetContent(common.ContentTypeJson, []byte(`{"user":"test"}`))

	if req.GetModule() != "user" {
		t.Errorf("module = %q, want %q", req.GetModule(), "user")
	}
	if req.GetRoute() != "/user/login" {
		t.Errorf("route = %q, want %q", req.GetRoute(), "/user/login")
	}
}

func TestResponseChainAPI(t *testing.T) {
	resp := AcquireResponse()
	defer ReleaseResponse(resp)

	resp.SetModule("user").SetRoute("/user/login").SetResult(200, "OK").SetContent(common.ContentTypeText, []byte("success"))

	if resp.GetModule() != "user" {
		t.Errorf("module = %q, want %q", resp.GetModule(), "user")
	}
	status, reason := resp.GetResult()
	if status != 200 || reason != "OK" {
		t.Errorf("result = (%d, %q), want (200, OK)", status, reason)
	}
}
