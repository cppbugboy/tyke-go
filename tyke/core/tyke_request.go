package core

import (
	"fmt"

	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/component"
	"github.com/tyke/tyke/tyke/ipc"
)

type TykeRequest struct {
	protocolHeader common.ProtocolHeader
	metadata       RequestMetadata
	content        []byte
}

var requestPool = component.NewObjectPool(func() TykeRequest {
	return TykeRequest{
		protocolHeader: common.ProtocolHeader{Magic: common.ProtocolMagic},
		metadata:       NewRequestMetadata(),
	}
})

func AcquireRequest() *TykeRequest {
	obj := requestPool.Acquire()
	return &obj
}

func ReleaseRequest(req *TykeRequest) {
	if req != nil {
		common.LogDebug("Releasing request object to pool", "msg_uuid", req.GetMsgUuid())
		req.Reset()
		obj := *req
		requestPool.Release(obj)
	}
}

func (r *TykeRequest) Reset() {
	r.protocolHeader = common.ProtocolHeader{Magic: common.ProtocolMagic}
	r.metadata = NewRequestMetadata()
	r.content = nil
}

func (r *TykeRequest) GetMagic() [4]byte {
	return r.protocolHeader.Magic
}

func (r *TykeRequest) GetMessageType() common.MessageType {
	return r.protocolHeader.MsgType
}

func (r *TykeRequest) SetContent(contentType common.ContentType, content []byte) *TykeRequest {
	r.metadata.SetContentType(common.ContentTypeMap[contentType])
	r.content = content
	return r
}

func (r *TykeRequest) GetContent() (string, []byte) {
	return r.metadata.GetContentType(), r.content
}

func (r *TykeRequest) SetModule(module string) *TykeRequest {
	r.metadata.SetModule(module)
	return r
}

func (r *TykeRequest) GetModule() string {
	return r.metadata.GetModule()
}

func (r *TykeRequest) SetRoute(route string) *TykeRequest {
	r.metadata.SetRoute(route)
	return r
}

func (r *TykeRequest) GetRoute() string {
	return r.metadata.GetRoute()
}

func (r *TykeRequest) GetMsgUuid() string {
	return r.metadata.GetMsgUuid()
}

func (r *TykeRequest) SetAsyncUuid(asyncUuid string) *TykeRequest {
	r.metadata.SetAsyncUuid(asyncUuid)
	return r
}

func (r *TykeRequest) GetAsyncUuid() string {
	return r.metadata.GetAsyncUuid()
}

func (r *TykeRequest) AddMetadata(key string, value common.JsonValue) common.BoolResult {
	return r.metadata.AddMetadata(key, value)
}

func (r *TykeRequest) GetMetadata(key string) (common.JsonValue, bool) {
	return r.metadata.GetMetadata(key)
}

func (r *TykeRequest) Send(sendUuid string, response *TykeResponse) common.BoolResult {
	common.LogDebug("Send", "send_uuid", sendUuid, "route", r.GetRoute())

	r.protocolHeader.MsgType = common.MessageTypeRequest
	r.metadata.SetMsgUuid(common.GenerateUUID()).SetTimestamp(common.GenerateTimestamp())

	dataVec, err := EncodeRequest(r)
	if err != nil {
		common.LogError("Encode request failed", "error", err)
		return common.ErrBool("encode request failed")
	}

	sendResult := ipc.IpcClientSend(sendUuid, dataVec, func(recvData []byte) bool {
		var dataSize uint32
		decodeResult := DecodeResponse(recvData, response, &dataSize)
		return decodeResult
	})
	if !sendResult.HasValue() {
		common.LogError("Send request failed", "error", sendResult.Err)
		return common.ErrBool("send request failed: " + sendResult.Err)
	}

	common.LogDebug("Sync request completed", "msg_uuid", r.GetMsgUuid())
	return common.OkBool(true)
}

func (r *TykeRequest) SendAsync(sendUuid string) common.BoolResult {
	return r.encodeAndSend(sendUuid, common.MessageTypeRequestAsync)
}

func (r *TykeRequest) SendAsyncWithFunc(sendUuid string, fn func(*TykeResponse)) common.BoolResult {
	common.LogDebug("SendAsyncWithFunc", "send_uuid", sendUuid, "route", r.GetRoute())
	result := r.encodeAndSend(sendUuid, common.MessageTypeRequestAsyncFunc)
	if result.HasValue() {
		RequestStubAddFunc(r.metadata.GetMsgUuid(), fn)
		common.LogDebug("Async callback registered", "msg_uuid", r.GetMsgUuid())
	}
	return result
}

func (r *TykeRequest) SendAsyncWithFuture(sendUuid string) (ResponseFuture, error) {
	common.LogDebug("SendAsyncWithFuture", "send_uuid", sendUuid, "route", r.GetRoute())
	result := r.encodeAndSend(sendUuid, common.MessageTypeRequestAsyncFuture)
	if !result.HasValue() {
		return ResponseFuture{}, fmt.Errorf("%s", result.Err)
	}
	ch := make(chan *TykeResponse, 1)
	RequestStubAddFuture(r.metadata.GetMsgUuid(), ch)
	future := NewResponseFuture(r.metadata.GetMsgUuid(), ch)
	common.LogDebug("Future registered", "msg_uuid", r.GetMsgUuid())
	return future, nil
}

func (r *TykeRequest) encodeAndSend(sendUuid string, msgType common.MessageType) common.BoolResult {
	common.LogDebug("EncodeAndSend", "send_uuid", sendUuid, "route", r.GetRoute(), "msg_type", int(msgType))
	r.protocolHeader.MsgType = msgType
	r.metadata.SetMsgUuid(common.GenerateUUID()).SetTimestamp(common.GenerateTimestamp())

	dataVec, err := EncodeRequest(r)
	if err != nil {
		common.LogError("Encode request failed", "error", err)
		return common.ErrBool("encode request failed")
	}

	sendResult := ipc.IpcClientSendAsync(sendUuid, dataVec)
	if !sendResult.HasValue() {
		common.LogError("Send request failed", "error", sendResult.Err)
		return common.ErrBool("send request failed: " + sendResult.Err)
	}

	common.LogDebug("Request sent successfully", "msg_uuid", r.GetMsgUuid())
	return common.OkBool(true)
}
