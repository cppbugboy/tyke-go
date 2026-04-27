package core

import (
	"fmt"

	"tyke-go/common"
	"tyke-go/component"
	"tyke-go/ipc"
)

// Request 表示一个 IPC 请求对象，支持同步和异步发送。
type Request struct {
	protocolHeader common.ProtocolHeader
	metadata       RequestMetadata
	content        []byte
	context        component.Context
}

var requestPool = component.NewObjectPool(func() *Request {
	return &Request{
		protocolHeader: common.ProtocolHeader{Magic: common.ProtocolMagic},
		metadata:       NewRequestMetadata(),
	}
})

func init() {
	requestPool.SetReset(func(req *Request) {
		req.Reset()
	})
}

// AcquireRequest 从对象池获取一个 Request 实例。
func AcquireRequest() *Request {
	common.LogDebug("Acquiring request from pool")
	return requestPool.Acquire()
}

// ReleaseRequest 将 Request 实例归还到对象池。
func ReleaseRequest(req *Request) {
	if req != nil {
		common.LogDebug("Releasing request object to pool", "msg_uuid", req.GetMsgUUID())
		req.Reset()
		requestPool.Release(req)
	}
}

func (r *Request) Reset() {
	r.protocolHeader = common.ProtocolHeader{Magic: common.ProtocolMagic}
	r.metadata = NewRequestMetadata()
	r.content = nil
}

func (r *Request) GetMagic() [4]byte {
	return r.protocolHeader.Magic
}

func (r *Request) GetMessageType() common.MessageType {
	return r.protocolHeader.MsgType
}

func (r *Request) SetContent(contentType common.ContentType, content []byte) *Request {
	r.metadata.SetContentType(common.ContentTypeMap[contentType])
	r.content = content
	return r
}

func (r *Request) GetContent() (string, []byte) {
	return r.metadata.GetContentType(), r.content
}

func (r *Request) SetModule(module string) *Request {
	r.metadata.SetModule(module)
	return r
}

func (r *Request) GetModule() string {
	return r.metadata.GetModule()
}

func (r *Request) SetRoute(route string) *Request {
	r.metadata.SetRoute(route)
	return r
}

func (r *Request) GetRoute() string {
	return r.metadata.GetRoute()
}

func (r *Request) GetMsgUUID() string {
	return r.metadata.GetMsgUUID()
}

func (r *Request) SetAsyncUUID(asyncUuid string) *Request {
	r.metadata.SetAsyncUUID(asyncUuid)
	return r
}

func (r *Request) GetAsyncUUID() string {
	return r.metadata.GetAsyncUUID()
}

func (r *Request) AddMetadata(key string, value common.JsonValueHolder) common.BoolResult {
	return r.metadata.AddMetadata(key, value)
}

func (r *Request) GetMetadata(key string) (common.JsonValueHolder, bool) {
	return r.metadata.GetMetadata(key)
}

func (r *Request) SetTimeout(timeout uint64) *Request {
	r.metadata.SetTimeout(timeout)
	return r
}

func (r *Request) GetTimeout() uint64 {
	return r.metadata.GetTimeout()
}

func (r *Request) SetContext(ctx component.Context) *Request {
	r.context = ctx
	return r
}

func (r *Request) GetContext() component.Context {
	return r.context
}

func (r *Request) Send(sendUuid string, response *Response, timeoutMs ...uint32) common.BoolResult {
	tm := uint32(common.DefaultTimeoutMs)
	if len(timeoutMs) > 0 {
		tm = timeoutMs[0]
	}
	common.LogDebug("Send", "send_uuid", sendUuid, "route", r.GetRoute(), "timeout", tm)

	r.metadata.SetTimeout(uint64(tm))
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

func (r *Request) SendAsync(sendUuid string, timeoutMs ...uint32) common.BoolResult {
	return r.encodeAndSend(sendUuid, common.MessageTypeRequestAsync, timeoutMs...)
}

func (r *Request) SendAsyncWithFunc(sendUuid string, fn func(*Response), timeoutMs ...uint32) common.BoolResult {
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

func (r *Request) SendAsyncWithFuture(sendUuid string, timeoutMs ...uint32) (ResponseFuture, error) {
	tm := uint32(common.DefaultTimeoutMs)
	if len(timeoutMs) > 0 {
		tm = timeoutMs[0]
	}
	common.LogDebug("SendAsyncWithFuture", "send_uuid", sendUuid, "route", r.GetRoute(), "timeout", tm)
	result := r.encodeAndSend(sendUuid, common.MessageTypeRequestAsyncFuture, timeoutMs...)
	if !result.HasValue() {
		return ResponseFuture{}, fmt.Errorf("%s", result.Err)
	}
	ch := make(chan *Response, 1)
	RequestStubAddFuture(r.metadata.GetMsgUUID(), ch, tm)
	future := NewResponseFuture(r.metadata.GetMsgUUID(), ch)
	common.LogDebug("Future registered", "msg_uuid", r.GetMsgUUID())
	return future, nil
}

func (r *Request) encodeAndSend(sendUuid string, msgType common.MessageType, timeoutMs ...uint32) common.BoolResult {
	tm := uint32(common.DefaultTimeoutMs)
	if len(timeoutMs) > 0 {
		tm = timeoutMs[0]
	}
	common.LogDebug("EncodeAndSend", "send_uuid", sendUuid, "route", r.GetRoute(), "msg_type", int(msgType), "timeout", tm)
	r.metadata.SetTimeout(uint64(tm))
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
