package core

import (
	"time"

	"github.com/cppbugboy/tyke-go/tyke/common"
)

func DispatchRequest(request *Request, response *Response) {
	start := time.Now()
	common.LogDebug("Dispatching request", "route", request.GetRoute(), "msg_uuid", request.GetMsgUUID())

	router := GetRequestRouterInstance()
	routeEntry := router.GetRouteEntry(request.GetRoute())
	if routeEntry == nil {
		common.LogWarn("Request route not found", "route", request.GetRoute(), "msg_uuid", request.GetMsgUUID())
		response.SetResult(int(common.StatusRouteError), "route not found")
		return
	}

	for _, filter := range routeEntry.FilterChain {
		if !filter.Before(request, response) {
			common.LogDebug("Request filter interrupted chain", "msg_uuid", request.GetMsgUUID())
			return
		}
	}

	common.LogDebug("Executing request handler", "route", request.GetRoute(), "msg_uuid", request.GetMsgUUID())
	routeEntry.Handler(request, response)

	for i := len(routeEntry.FilterChain) - 1; i >= 0; i-- {
		if !routeEntry.FilterChain[i].After(request, response) {
			common.LogDebug("Request filter interrupted chain", "msg_uuid", request.GetMsgUUID())
			return
		}
	}

	elapsed := time.Since(start)
	common.LogInfo("Request dispatched", "route", request.GetRoute(), "msg_uuid", request.GetMsgUUID(), "elapsed", elapsed)
}

func DispatchResponse(response *Response) {
	common.LogDebug("Dispatching response", "route", response.GetRoute(), "msg_uuid", response.GetMsgUUID())

	router := GetResponseRouterInstance()
	routeEntry := router.GetRouteEntry(response.GetRoute())
	if routeEntry == nil {
		common.LogWarn("Response route not found, trying stub handlers", "route", response.GetRoute(), "msg_uuid", response.GetMsgUUID())
		if RequestStubExecFuncOrSetFuture(response) {
			return
		}
		common.LogWarn("Response dropped: no route and no stub handler found", "route", response.GetRoute(), "msg_uuid", response.GetMsgUUID())
		return
	}

	for _, filter := range routeEntry.FilterChain {
		if !filter.Before(response) {
			common.LogDebug("Response filter interrupted chain", "msg_uuid", response.GetMsgUUID())
			return
		}
	}

	common.LogDebug("Executing response handler", "route", response.GetRoute(), "msg_uuid", response.GetMsgUUID())
	routeEntry.Handler(response)

	for i := len(routeEntry.FilterChain) - 1; i >= 0; i-- {
		if !routeEntry.FilterChain[i].After(response) {
			common.LogDebug("Response filter interrupted chain", "msg_uuid", response.GetMsgUUID())
			return
		}
	}

	common.LogDebug("Response dispatched successfully", "route", response.GetRoute(), "msg_uuid", response.GetMsgUUID())
}
