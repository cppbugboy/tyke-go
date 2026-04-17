package core

import (
	"github.com/tyke/tyke/pkg/common"
)

func DispatchRequest(req *TykeRequest, resp *TykeResponse) {
	router := GetRequestRouter()
	entry := router.GetRouteEntry(req.GetRoute())
	if entry == nil {
		resp.SetResult(404, "route not found")
		resp.metadata.Route = req.GetRoute()
		resp.metadata.MsgUUID = req.GetMsgUuid()
		resp.metadata.ContentType = req.metadata.ContentType
		resp.metadata.Timestamp = common.GenerateTimestamp()
		resp.msgType = common.MessageTypeResponse
		return
	}

	resp.metadata.Module = req.GetModule()
	resp.metadata.MsgUUID = req.GetMsgUuid()
	resp.metadata.Route = req.GetRoute()
	resp.metadata.ContentType = req.metadata.ContentType
	resp.metadata.Timestamp = common.GenerateTimestamp()
	resp.msgType = common.MessageTypeResponse

	for i := 0; i < len(entry.Filters); i++ {
		if !entry.Filters[i].Before(req, resp) {
			return
		}
	}

	if entry.Handler != nil {
		entry.Handler(req, resp)
	}

	for i := len(entry.Filters) - 1; i >= 0; i-- {
		if !entry.Filters[i].After(req, resp) {
			return
		}
	}
}

func DispatchResponse(resp *TykeResponse) {
	msgType := resp.GetMessageType()

	switch msgType {
	case common.MessageTypeResponseAsyncFuture:
		GetRequestStub().SetFuture(resp)
	case common.MessageTypeResponseAsyncFunc:
		GetRequestStub().ExecFunc(resp)
	default:
		router := GetResponseRouter()
		entry := router.GetRouteEntry(resp.GetRoute())
		if entry == nil {
			return
		}

		for i := 0; i < len(entry.Filters); i++ {
			if !entry.Filters[i].Before(resp) {
				return
			}
		}

		if entry.Handler != nil {
			entry.Handler(resp)
		}

		for i := len(entry.Filters) - 1; i >= 0; i-- {
			if !entry.Filters[i].After(resp) {
				return
			}
		}
	}
}
