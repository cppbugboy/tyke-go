package core

import (
	"encoding/binary"

	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/ipc"
)

func DataCallback(clientId ipc.ClientId, dataVec []byte, sendDataHandler SendDataHandler) *uint32 {
	common.LogDebug("DataCallback invoked", "client_id", clientId, "data_size", len(dataVec))

	if len(dataVec) <= common.ProtocolHeaderSize {
		common.LogWarn("Data too short for protocol header", "size", len(dataVec))
		zero := uint32(0)
		return &zero
	}

	var header common.ProtocolHeader
	offset := uint32(0)
	copy(header.Magic[:], dataVec[offset:offset+4])
	offset += 4
	header.MsgType = common.MessageType(binary.LittleEndian.Uint32(dataVec[offset:]))

	if header.Magic != common.ProtocolMagic {
		common.LogWarn("Protocol magic mismatch, discarding")
		return nil
	}

	common.LogDebug("Received message", "type", int(header.MsgType))

	var used uint32

	switch header.MsgType {
	case common.MessageTypeRequest:
		request := AcquireRequest()
		defer ReleaseRequest(request)
		if DecodeRequest(dataVec, request, &used) {
			common.LogDebug("Processing sync request", "route", request.GetRoute())
			RequestHandler(clientId, request, sendDataHandler)
		}

	case common.MessageTypeRequestAsync, common.MessageTypeRequestAsyncFunc, common.MessageTypeRequestAsyncFuture:
		request := AcquireRequest()
		defer ReleaseRequest(request)
		if DecodeRequest(dataVec, request, &used) {
			common.LogDebug("Processing async request", "route", request.GetRoute())
			RequestHandlerAsync(request)
		}

	case common.MessageTypeResponseAsync, common.MessageTypeResponseAsyncFunc, common.MessageTypeResponseAsyncFuture:
		response := AcquireResponse()
		defer ReleaseResponse(response)
		if DecodeResponse(dataVec, response, &used) {
			common.LogDebug("Processing async response", "route", response.GetRoute())
			ResponseHandler(response)
		}

	default:
		common.LogWarn("Unknown message type", "type", int(header.MsgType))
	}

	return &used
}

func RequestHandler(clientId ipc.ClientId, request *TykeRequest, sendDataHandler SendDataHandler) {
	common.LogDebug("RequestHandler", "client_id", clientId, "route", request.GetRoute(), "msg_uuid", request.GetMsgUuid())

	response := AcquireResponse()
	defer ReleaseResponse(response)
	response.SetClientId(clientId).
		SetMessageType(common.MessageTypeResponse).
		SetModule(request.GetModule()).
		SetMsgUuid(request.GetMsgUuid()).
		SetRoute(request.GetRoute()).
		SetAsyncUuid(request.GetAsyncUuid()).
		SetSendDataHandler(sendDataHandler)

	DispatchRequest(request, response)

	if sendResult := response.Send(); !sendResult.HasValue() {
		common.LogError("Send response failed", "error", sendResult.Err)
	}
}

func RequestHandlerAsync(request *TykeRequest) {
	common.LogDebug("RequestHandlerAsync", "route", request.GetRoute(), "msg_uuid", request.GetMsgUuid())

	response := AcquireResponse()
	defer ReleaseResponse(response)
	response.SetAsyncUuid(request.GetAsyncUuid()).
		SetModule(request.GetModule()).
		SetMsgUuid(request.GetMsgUuid()).
		SetRoute(request.GetRoute())

	switch request.GetMessageType() {
	case common.MessageTypeRequestAsync:
		response.SetMessageType(common.MessageTypeResponseAsync)
	case common.MessageTypeRequestAsyncFunc:
		response.SetMessageType(common.MessageTypeResponseAsyncFunc)
	case common.MessageTypeRequestAsyncFuture:
		response.SetMessageType(common.MessageTypeResponseAsyncFuture)
	}

	DispatchRequest(request, response)

	if sendResult := response.SendAsync(); !sendResult.HasValue() {
		common.LogError("Send async response failed", "error", sendResult.Err)
	}
}

func ResponseHandler(response *TykeResponse) {
	common.LogDebug("ResponseHandler", "route", response.GetRoute(), "msg_uuid", response.GetMsgUuid())

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
