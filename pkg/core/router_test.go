package core

import (
	"testing"

	"github.com/tyke/tyke/pkg/common"
)

type testFilter struct {
	beforeCalled bool
	afterCalled  bool
	beforeResult bool
	afterResult  bool
}

func (f *testFilter) Before(req *TykeRequest, resp *TykeResponse) bool {
	f.beforeCalled = true
	return f.beforeResult
}

func (f *testFilter) After(req *TykeRequest, resp *TykeResponse) bool {
	f.afterCalled = true
	return f.afterResult
}

func TestRouterAddRouteHandler(t *testing.T) {
	router := GetRequestRouter()
	root := router.GetRoot()

	called := false
	root.AddRouteHandler("/test/route1", func(req *TykeRequest, resp *TykeResponse) {
		called = true
	})

	entry := router.GetRouteEntry("/test/route1")
	if entry == nil {
		t.Fatal("route not found")
	}

	req := AcquireRequest()
	defer ReleaseRequest(req)
	req.SetRoute("/test/route1")
	req.msgType = common.MessageTypeRequest
	req.metadata.MsgUUID = "test-uuid"
	req.metadata.Timestamp = common.GenerateTimestamp()

	resp := AcquireResponse()
	defer ReleaseResponse(resp)

	entry.Handler(req, resp)
	if !called {
		t.Error("handler was not called")
	}
}

func TestRouterSubGroup(t *testing.T) {
	router := GetRequestRouter()
	root := router.GetRoot()

	apiGroup := root.AddSubGroup("/api")
	apiGroup.AddRouteHandler("/users", func(req *TykeRequest, resp *TykeResponse) {})

	entry := router.GetRouteEntry("/api/users")
	if entry == nil {
		t.Fatal("subgroup route not found")
	}
}

func TestDispatchRequestWithFilters(t *testing.T) {
	router := GetRequestRouter()
	root := router.GetRoot()

	filter1 := &testFilter{beforeResult: true, afterResult: true}

	group := root.AddSubGroup("/filter-test")
	group.AddFilter(filter1)
	group.AddRouteHandler("/endpoint", func(req *TykeRequest, resp *TykeResponse) {
		resp.SetResult(200, "OK")
	})

	req := AcquireRequest()
	defer ReleaseRequest(req)
	req.SetRoute("/filter-test/endpoint")
	req.msgType = common.MessageTypeRequest
	req.metadata.MsgUUID = "test-uuid"
	req.metadata.Timestamp = common.GenerateTimestamp()

	resp := AcquireResponse()
	defer ReleaseResponse(resp)

	DispatchRequest(req, resp)

	if !filter1.beforeCalled {
		t.Error("filter Before was not called")
	}
	if !filter1.afterCalled {
		t.Error("filter After was not called")
	}
}

func TestDispatchRequestFilterInterrupt(t *testing.T) {
	router := GetRequestRouter()
	root := router.GetRoot()

	blockFilter := &testFilter{beforeResult: false, afterResult: true}
	handlerCalled := false

	group := root.AddSubGroup("/interrupt-test")
	group.AddFilter(blockFilter)
	group.AddRouteHandler("/blocked", func(req *TykeRequest, resp *TykeResponse) {
		handlerCalled = true
	})

	req := AcquireRequest()
	defer ReleaseRequest(req)
	req.SetRoute("/interrupt-test/blocked")
	req.msgType = common.MessageTypeRequest
	req.metadata.MsgUUID = "test-uuid"
	req.metadata.Timestamp = common.GenerateTimestamp()

	resp := AcquireResponse()
	defer ReleaseResponse(resp)

	DispatchRequest(req, resp)

	if handlerCalled {
		t.Error("handler should not be called when filter returns false")
	}
}

func TestDispatchRequestRouteNotFound(t *testing.T) {
	req := AcquireRequest()
	defer ReleaseRequest(req)
	req.SetRoute("/nonexistent")
	req.msgType = common.MessageTypeRequest
	req.metadata.MsgUUID = "test-uuid"
	req.metadata.Timestamp = common.GenerateTimestamp()

	resp := AcquireResponse()
	defer ReleaseResponse(resp)

	DispatchRequest(req, resp)

	status, reason := resp.GetResult()
	if status != 404 {
		t.Errorf("status = %d, want 404", status)
	}
	if reason != "route not found" {
		t.Errorf("reason = %q, want %q", reason, "route not found")
	}
}
