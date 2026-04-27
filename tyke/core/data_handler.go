package core

import (
	"encoding/binary"
	"time"

	"tyke-go/common"
	"tyke-go/component"
	"tyke-go/ipc"
)

func DataCallback(clientId ipc.ClientId, dataVec []byte, sendDataHandler SendDataHandler) *uint32 {
	common.LogDebug("DataCallback invoked", "client_id", clientId, "data_size", len(dataVec))

	if len(dataVec) <= common.ProtocolHeaderSize {
		common.LogWarn("Data too short for protocol header, discarding", "size", len(dataVec))
		zero := uint32(0)
		return &zero
	}

	var header common.ProtocolHeader
	offset := uint32(0)
	copy(header.Magic[:], dataVec[offset:offset+4])
	offset += 4
	header.MsgType = common.MessageType(binary.LittleEndian.Uint32(dataVec[offset:]))

	if header.Magic != common.ProtocolMagic {
		common.LogWarn("Protocol magic mismatch, expected=TYKE, discarding", "bytes", len(dataVec))
		return nil
	}

	common.LogDebug("Received message", "type", int(header.MsgType), "metadata_len", header.MetadataLen, "content_len", header.ContentLen)

	var used uint32

	switch header.MsgType {
	case common.MessageTypeRequest:
		request := AcquireRequest()
		if DecodeRequest(dataVec, request, &used) {
			common.LogDebug("Processing sync request", "route", request.GetRoute())
			RequestHandler(clientId, request, sendDataHandler)
		} else {
			if used == 0 {
				common.LogWarn("Decode request failed, data incomplete, waiting for more data")
				ReleaseRequest(request)
				return nil
			}
			common.LogWarn("Decode request failed, invalid data, discarding")
			ReleaseRequest(request)
			zero := uint32(0)
			return &zero
		}
		ReleaseRequest(request)

	case common.MessageTypeRequestAsync, common.MessageTypeRequestAsyncFunc, common.MessageTypeRequestAsyncFuture:
		request := AcquireRequest()
		if DecodeRequest(dataVec, request, &used) {
			common.LogDebug("Processing async request", "route", request.GetRoute(), "msg_type", int(header.MsgType))
			RequestHandlerAsync(request)
		} else {
			if used == 0 {
				common.LogWarn("Decode async request failed, data incomplete, waiting for more data")
				ReleaseRequest(request)
				return nil
			}
			common.LogWarn("Decode async request failed, invalid data, discarding")
			ReleaseRequest(request)
			zero := uint32(0)
			return &zero
		}

	case common.MessageTypeResponseAsync, common.MessageTypeResponseAsyncFunc, common.MessageTypeResponseAsyncFuture:
		response := AcquireResponse()
		if DecodeResponse(dataVec, response, &used) {
			common.LogDebug("Processing async response", "route", response.GetRoute(), "msg_uuid", response.GetMsgUUID())
			ResponseHandler(response)
		} else {
			ReleaseResponse(response)
			if used == 0 {
				common.LogWarn("Decode async response failed, data incomplete, waiting for more data")
				return nil
			}
			common.LogWarn("Decode async response failed, invalid data, discarding")
			zero := uint32(0)
			return &zero
		}

	default:
		common.LogWarn("Unknown message type", "type", int(header.MsgType))
	}

	return &used
}

func RequestHandler(clientId ipc.ClientId, request *Request, sendDataHandler SendDataHandler) {
	common.LogDebug("RequestHandler", "client_id", clientId, "route", request.GetRoute(), "msg_uuid", request.GetMsgUUID())

	response := AcquireResponse()
	response.SetClientId(clientId).
		SetMessageType(common.MessageTypeResponse).
		SetModule(request.GetModule()).
		SetMsgUUID(request.GetMsgUUID()).
		SetRoute(request.GetRoute()).
		SetAsyncUUID(request.GetAsyncUUID()).
		SetSendDataHandler(sendDataHandler)

	defer ReleaseResponse(response)

	timeoutMs := request.GetTimeout()
	if timeoutMs == 0 {
		timeoutMs = uint64(common.DefaultTimeoutMs)
	}

	ctx, cancel := component.ContextWithTimeout(component.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	timerCtx, ok := ctx.(*component.TimerContext)
	if !ok {
		common.LogError("Failed to cast context to TimerContext")
		response.SetResult(int(common.StatusInternalError), "internal error")
		if sendResult := response.Send(); !sendResult.HasValue() {
			common.LogError("Send response failed", "error", sendResult.Err)
		}
		return
	}

	token := timerCtx.RegisterCallback(func() {
		response.SetResult(int(common.StatusTimeout), "timeout")
		if sendResult := response.Send(); !sendResult.HasValue() {
			common.LogError("Send response failed", "error", sendResult.Err)
		}
	})
	timerCtx.ActivateTimer()
	request.SetContext(timerCtx)

	DispatchRequest(request, response)

	if !response.IsSent() {
		timerCtx.UnregisterCallback(token)
		if sendResult := response.Send(); !sendResult.HasValue() {
			common.LogError("Send response failed", "error", sendResult.Err)
		}
	}
}

func RequestHandlerAsync(request *Request) {
	defer ReleaseRequest(request)
	common.LogDebug("RequestHandlerAsync", "route", request.GetRoute(), "msg_uuid", request.GetMsgUUID())

	response := AcquireResponse()
	response.SetAsyncUUID(request.GetAsyncUUID()).
		SetMessageType(common.MessageTypeResponseAsync).
		SetModule(request.GetModule()).
		SetMsgUUID(request.GetMsgUUID()).
		SetRoute(request.GetRoute())

	defer ReleaseResponse(response)

	switch request.GetMessageType() {
	case common.MessageTypeRequestAsync:
		response.SetMessageType(common.MessageTypeResponseAsync)
	case common.MessageTypeRequestAsyncFunc:
		response.SetMessageType(common.MessageTypeResponseAsyncFunc)
	case common.MessageTypeRequestAsyncFuture:
		response.SetMessageType(common.MessageTypeResponseAsyncFuture)
	}

	timeoutMs := request.GetTimeout()
	if timeoutMs == 0 {
		timeoutMs = uint64(common.DefaultTimeoutMs)
	}

	ctx, cancel := component.ContextWithTimeout(component.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	timerCtx, ok := ctx.(*component.TimerContext)
	if !ok {
		common.LogError("Failed to cast context to TimerContext")
		response.SetResult(int(common.StatusInternalError), "internal error")
		if sendResult := response.SendAsync(); !sendResult.HasValue() {
			common.LogError("Send async response failed", "error", sendResult.Err)
		}
		return
	}

	token := timerCtx.RegisterCallback(func() {
		response.SetResult(int(common.StatusTimeout), "timeout")
		if sendResult := response.SendAsync(); !sendResult.HasValue() {
			common.LogError("Send async response failed", "error", sendResult.Err)
		}
	})
	timerCtx.ActivateTimer()
	request.SetContext(timerCtx)

	DispatchRequest(request, response)

	if !response.IsSent() {
		timerCtx.UnregisterCallback(token)
		if sendResult := response.SendAsync(); !sendResult.HasValue() {
			common.LogError("Send async response failed", "error", sendResult.Err)
		}
	}

}

func ResponseHandler(response *Response) {
	common.LogDebug("ResponseHandler", "route", response.GetRoute(), "msg_uuid", response.GetMsgUUID(), "msg_type", int(response.GetMessageType()))

	switch response.GetMessageType() {
	case common.MessageTypeResponseAsync:
		DispatchResponse(response)
	case common.MessageTypeResponseAsyncFunc:
		RequestStubExecFunc(response)
	case common.MessageTypeResponseAsyncFuture:
		RequestStubSetFuture(response)
	default:
		common.LogWarn("Unknown response type", "type", int(response.GetMessageType()))
	}
}
