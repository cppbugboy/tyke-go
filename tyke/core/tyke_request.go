package core

import (
	"fmt"

	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/component"
	"github.com/tyke/tyke/tyke/ipc"
)

// TykeRequest 表示一个 IPC 请求对象，支持同步和异步发送。
type TykeRequest struct {
	protocolHeader common.ProtocolHeader
	metadata       RequestMetadata
	content        []byte
}

var requestPool = component.NewObjectPool(func() *TykeRequest {
	return &TykeRequest{
		protocolHeader: common.ProtocolHeader{Magic: common.ProtocolMagic},
		metadata:       NewRequestMetadata(),
	}
})

// AcquireRequest 从对象池获取一个 TykeRequest 实例。
func AcquireRequest() *TykeRequest {
	common.LogDebug("Acquiring request from pool")
	return requestPool.Acquire()
}

// ReleaseRequest 将 TykeRequest 实例归还到对象池。
func ReleaseRequest(req *TykeRequest) {
	if req != nil {
		common.LogDebug("Releasing request object to pool", "msg_uuid", req.GetMsgUUID())
		req.Reset()
		requestPool.Release(req)
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

func (r *TykeRequest) GetMsgUUID() string {
	return r.metadata.GetMsgUUID()
}

func (r *TykeRequest) SetAsyncUUID(asyncUuid string) *TykeRequest {
	r.metadata.SetAsyncUUID(asyncUuid)
	return r
}

func (r *TykeRequest) GetAsyncUUID() string {
	return r.metadata.GetAsyncUUID()
}

func (r *TykeRequest) AddMetadata(key string, value common.JsonValue) common.BoolResult {
	return r.metadata.AddMetadata(key, value)
}

func (r *TykeRequest) GetMetadata(key string) (common.JsonValue, bool) {
	return r.metadata.GetMetadata(key)
}

func (r *TykeRequest) Send(sendUuid string, response *TykeResponse, timeoutMs ...uint32) common.BoolResult {
	tm := uint32(common.DefaultTimeoutMs)
	if len(timeoutMs) > 0 {
		tm = timeoutMs[0]
	}
	common.LogDebug("Send", "send_uuid", sendUuid, "route", r.GetRoute(), "timeout", tm)

	r.protocolHeader.MsgType = common.MessageTypeRequest
	r.metadata.SetMsgUUID(common.GenerateUUID()).SetTimestamp(common.GenerateTimestamp())

	dataVec, err := EncodeRequest(r)
	if err != nil {
		common.LogError("Encode request failed", "error", err)
		return common.ErrBool("encode request failed")
	}

	sendResult := ipc.IPCClientSend(sendUuid, dataVec, func(recvData []byte) bool {
		var dataSize uint32
		decodeResult := DecodeResponse(recvData, response, &dataSize)
		return decodeResult
	}, tm)
	if !sendResult.HasValue() {
		common.LogError("Send request failed", "error", sendResult.Err)
		return common.ErrBool("send request failed: " + sendResult.Err)
	}

	common.LogDebug("Sync request completed", "msg_uuid", r.GetMsgUUID())
	return common.OkBool(true)
}

func (r *TykeRequest) SendAsync(sendUuid string, timeoutMs ...uint32) common.BoolResult {
	return r.encodeAndSend(sendUuid, common.MessageTypeRequestAsync, timeoutMs...)
}

func (r *TykeRequest) SendAsyncWithFunc(sendUuid string, fn func(*TykeResponse), timeoutMs ...uint32) common.BoolResult {
	tm := uint32(common.DefaultTimeoutMs)
	if len(timeoutMs) > 0 {
		tm = timeoutMs[0]
	}
	common.LogDebug("SendAsyncWithFunc", "send_uuid", sendUuid, "route", r.GetRoute(), "timeout", tm)
	result := r.encodeAndSend(sendUuid, common.MessageTypeRequestAsyncFunc, timeoutMs...)
	if result.HasValue() {
		RequestStubAddFunc(r.metadata.GetMsgUUID(), fn, tm)
		common.LogDebug("Async callback registered", "msg_uuid", r.GetMsgUUID())
	}
	return result
}

func (r *TykeRequest) SendAsyncWithFuture(sendUuid string, timeoutMs ...uint32) (ResponseFuture, error) {
	tm := uint32(common.DefaultTimeoutMs)
	if len(timeoutMs) > 0 {
		tm = timeoutMs[0]
	}
	common.LogDebug("SendAsyncWithFuture", "send_uuid", sendUuid, "route", r.GetRoute(), "timeout", tm)
	result := r.encodeAndSend(sendUuid, common.MessageTypeRequestAsyncFuture, timeoutMs...)
	if !result.HasValue() {
		return ResponseFuture{}, fmt.Errorf("%s", result.Err)
	}
	ch := make(chan *TykeResponse, 1)
	RequestStubAddFuture(r.metadata.GetMsgUUID(), ch, tm)
	future := NewResponseFuture(r.metadata.GetMsgUUID(), ch)
	common.LogDebug("Future registered", "msg_uuid", r.GetMsgUUID())
	return future, nil
}

func (r *TykeRequest) encodeAndSend(sendUuid string, msgType common.MessageType, timeoutMs ...uint32) common.BoolResult {
	tm := uint32(common.DefaultTimeoutMs)
	if len(timeoutMs) > 0 {
		tm = timeoutMs[0]
	}
	common.LogDebug("EncodeAndSend", "send_uuid", sendUuid, "route", r.GetRoute(), "msg_type", int(msgType), "timeout", tm)
	r.protocolHeader.MsgType = msgType
	r.metadata.SetMsgUUID(common.GenerateUUID()).SetTimestamp(common.GenerateTimestamp())

	dataVec, err := EncodeRequest(r)
	if err != nil {
		common.LogError("Encode request failed", "error", err)
		return common.ErrBool("encode request failed")
	}

	sendResult := ipc.IPCClientSendAsync(sendUuid, dataVec, tm)
	if !sendResult.HasValue() {
		common.LogError("Send request failed", "error", sendResult.Err)
		return common.ErrBool("send request failed: " + sendResult.Err)
	}

	common.LogDebug("Request sent successfully", "msg_uuid", r.GetMsgUUID())
	return common.OkBool(true)
}
