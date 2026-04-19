package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/tyke/tyke/tyke/common"
)

func TestSendAsyncWithFutureBasic(t *testing.T) {
	ch := make(chan *TykeResponse, 1)
	RequestStubAddFuture("test-uuid-basic", ch)

	resp := AcquireResponse()
	resp.metadata.SetMsgUuid("test-uuid-basic")
	resp.SetResult(200, "OK")
	resp.metadata.SetModule("test_module")
	resp.metadata.SetRoute("/test/route")
	resp.content = []byte(`{"status":"success"}`)

	RequestStubSetFuture(resp)
	ReleaseResponse(resp)

	future := NewResponseFuture("test-uuid-basic", ch)
	result := future.GetResponse()

	if result.GetMsgUuid() != "test-uuid-basic" {
		t.Errorf("expected msg_uuid=test-uuid-basic, got %s", result.GetMsgUuid())
	}
	if status, _ := result.GetResult(); status != 200 {
		t.Errorf("expected status=200, got %d", status)
	}
	if result.GetModule() != "test_module" {
		t.Errorf("expected module=test_module, got %s", result.GetModule())
	}
	if _, content := result.GetContent(); len(content) == 0 {
		t.Error("expected content to be non-empty")
	}
}

func TestSendAsyncWithFutureAfterRelease(t *testing.T) {
	ch := make(chan *TykeResponse, 1)
	RequestStubAddFuture("test-uuid-release", ch)

	resp := AcquireResponse()
	resp.metadata.SetMsgUuid("test-uuid-release")
	resp.SetResult(200, "OK")
	resp.metadata.SetModule("release_module")
	resp.metadata.SetRoute("/release/route")
	resp.content = []byte(`{"key":"value"}`)

	RequestStubSetFuture(resp)
	ReleaseResponse(resp)

	future := NewResponseFuture("test-uuid-release", ch)
	result := future.GetResponse()

	if result.GetMsgUuid() != "test-uuid-release" {
		t.Errorf("expected msg_uuid=test-uuid-release, got %s", result.GetMsgUuid())
	}
	if status, _ := result.GetResult(); status != 200 {
		t.Errorf("expected status=200, got %d", status)
	}
	if result.GetModule() != "release_module" {
		t.Errorf("expected module=release_module, got %s", result.GetModule())
	}
	if _, content := result.GetContent(); len(content) == 0 {
		t.Error("expected content to be non-empty after release")
	}
}

func TestSendAsyncWithFutureConcurrent(t *testing.T) {
	const count = 10
	done := make(chan bool, count)

	for i := 0; i < count; i++ {
		go func(idx int) {
			uuid := fmt.Sprintf("uuid-%d", idx)
			ch := make(chan *TykeResponse, 1)
			RequestStubAddFuture(uuid, ch)

			resp := AcquireResponse()
			resp.metadata.SetMsgUuid(uuid)
			resp.SetResult(200, fmt.Sprintf("OK-%d", idx))

			RequestStubSetFuture(resp)
			ReleaseResponse(resp)

			future := NewResponseFuture(uuid, ch)
			result := future.GetResponse()

			if result.GetMsgUuid() != uuid {
				t.Errorf("goroutine %d: expected msg_uuid=%s, got %s", idx, uuid, result.GetMsgUuid())
			}
			done <- true
		}(i)
	}

	for i := 0; i < count; i++ {
		<-done
	}
}

func TestSendAsyncWithFutureTimeout(t *testing.T) {
	ch := make(chan *TykeResponse, 1)
	RequestStubAddFuture("timeout-uuid-test", ch)

	go func() {
		time.Sleep(50 * time.Millisecond)
		RequestStubCleanupExpired(1)
	}()

	future := NewResponseFuture("timeout-uuid-test", ch)
	result := future.GetResponse()

	if status, _ := result.GetResult(); status != common.HttpStatusTimeout {
		t.Errorf("expected timeout status=%d, got %d", common.HttpStatusTimeout, status)
	}
}

func TestSendAsyncWithFutureNotFound(t *testing.T) {
	ch := make(chan *TykeResponse, 1)

	resp := AcquireResponse()
	resp.metadata.SetMsgUuid("non-existent-uuid")
	resp.SetResult(200, "OK")

	RequestStubSetFuture(resp)
	ReleaseResponse(resp)

	select {
	case <-ch:
		t.Error("expected no data in channel for non-existent uuid")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSendAsyncWithFutureMetadata(t *testing.T) {
	ch := make(chan *TykeResponse, 1)
	RequestStubAddFuture("metadata-uuid", ch)

	resp := AcquireResponse()
	resp.metadata.SetMsgUuid("metadata-uuid")
	resp.SetResult(200, "OK")
	resp.metadata.SetModule("metadata_module")
	resp.metadata.SetRoute("/metadata/route")
	resp.metadata.SetContentType("json")
	resp.metadata.SetTimestamp("2026-04-19T00:00:00Z")
	resp.content = []byte(`{"test":"data"}`)

	RequestStubSetFuture(resp)
	ReleaseResponse(resp)

	future := NewResponseFuture("metadata-uuid", ch)
	result := future.GetResponse()

	if result.GetMsgUuid() != "metadata-uuid" {
		t.Errorf("expected msg_uuid=metadata-uuid, got %s", result.GetMsgUuid())
	}
	if result.GetRoute() != "/metadata/route" {
		t.Errorf("expected route=/metadata/route, got %s", result.GetRoute())
	}
	if contentType, _ := result.GetContent(); contentType != "json" {
		t.Errorf("expected content_type=json, got %s", contentType)
	}
	if result.metadata.GetTimestamp() != "2026-04-19T00:00:00Z" {
		t.Errorf("expected timestamp=2026-04-19T00:00:00Z, got %s", result.metadata.GetTimestamp())
	}
}
