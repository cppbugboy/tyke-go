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

func TestFlatRouteTableCache(t *testing.T) {
	r := newRouter[RequestFilter, RequestHandlerFunc]()
	root := r.GetRoot()

	root.AddRouteHandler("/flat/root", func(req *TykeRequest, resp *TykeResponse) {})
	apiGroup := root.AddSubGroup("/flat/api")
	apiGroup.AddRouteHandler("/users", func(req *TykeRequest, resp *TykeResponse) {})
	apiGroup.AddRouteHandler("/posts", func(req *TykeRequest, resp *TykeResponse) {})

	if entry := r.GetRouteEntry("/flat/root"); entry == nil {
		t.Error("flat cache: root route not found")
	}
	if entry := r.GetRouteEntry("/flat/api/users"); entry == nil {
		t.Error("flat cache: subgroup route /flat/api/users not found")
	}
	if entry := r.GetRouteEntry("/flat/api/posts"); entry == nil {
		t.Error("flat cache: subgroup route /flat/api/posts not found")
	}
	if entry := r.GetRouteEntry("/nonexistent"); entry != nil {
		t.Error("flat cache: nonexistent route should return nil")
	}
}

func TestGenericRouterResponse(t *testing.T) {
	r := newRouter[ResponseFilter, ResponseHandlerFunc]()
	root := r.GetRoot()

	called := false
	root.AddRouteHandler("/resp/test", func(resp *TykeResponse) {
		called = true
	})

	entry := r.GetRouteEntry("/resp/test")
	if entry == nil {
		t.Fatal("response route not found in flat cache")
	}
	entry.Handler(nil)
	if !called {
		t.Error("response handler was not called")
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
