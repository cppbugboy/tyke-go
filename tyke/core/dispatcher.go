package core

import (
	"time"

	"github.com/tyke/tyke/tyke/common"
)

func DispatchRequest(request *TykeRequest, response *TykeResponse) {
	start := time.Now()
	common.LogDebug("Dispatching request", "route", request.GetRoute(), "msg_uuid", request.GetMsgUuid())

	router := GetRequestRouterInstance()
	routeEntry := router.GetRouteEntry(request.GetRoute())
	if routeEntry == nil {
		common.LogWarn("Request route not found", "route", request.GetRoute(), "msg_uuid", request.GetMsgUuid())
		response.SetResult(common.HttpStatusNotFound, "Not Found")
		return
	}

	for _, filter := range routeEntry.FilterChain {
		if !filter.Before(request, response) {
			common.LogDebug("Request filter interrupted chain", "msg_uuid", request.GetMsgUuid())
			return
		}
	}

	common.LogDebug("Executing request handler", "route", request.GetRoute(), "msg_uuid", request.GetMsgUuid())
	routeEntry.Handler(request, response)

	for i := len(routeEntry.FilterChain) - 1; i >= 0; i-- {
		if !routeEntry.FilterChain[i].After(request, response) {
			common.LogDebug("Request filter interrupted chain", "msg_uuid", request.GetMsgUuid())
			return
		}
	}

	elapsed := time.Since(start)
	common.LogInfo("Request dispatched", "route", request.GetRoute(), "msg_uuid", request.GetMsgUuid(), "elapsed", elapsed)
}

func DispatchResponse(response *TykeResponse) {
	common.LogDebug("Dispatching response", "route", response.GetRoute(), "msg_uuid", response.GetMsgUuid())

	router := GetResponseRouterInstance()
	routeEntry := router.GetRouteEntry(response.GetRoute())
	if routeEntry == nil {
		common.LogWarn("Response route not found", "route", response.GetRoute(), "msg_uuid", response.GetMsgUuid())
		return
	}

	for _, filter := range routeEntry.FilterChain {
		if !filter.Before(response) {
			common.LogDebug("Response filter interrupted chain", "msg_uuid", response.GetMsgUuid())
			return
		}
	}

	common.LogDebug("Executing response handler", "route", response.GetRoute(), "msg_uuid", response.GetMsgUuid())
	routeEntry.Handler(response)

	for i := len(routeEntry.FilterChain) - 1; i >= 0; i-- {
		if !routeEntry.FilterChain[i].After(response) {
			common.LogDebug("Response filter interrupted chain", "msg_uuid", response.GetMsgUuid())
			return
		}
	}

	common.LogDebug("Response dispatched successfully", "route", response.GetRoute(), "msg_uuid", response.GetMsgUuid())
}
