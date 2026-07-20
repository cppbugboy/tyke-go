package core

import (
	"fmt"

	"tyke-go/common"
	"tyke-go/component"
	"tyke-go/ipc"
)

// Request 表示一个 IPC 请求，支持同步和异步
// 发送模式（发后即忘、回调、Future）。实例从对象池中获取
// 并归还以降低内存分配。
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

// AcquireRequest 从对象池中获取一个 Request 并设置其模块。
func AcquireRequest() *Request {
	common.LogDebug("Acquiring request from pool")
	request := requestPool.Acquire()
	request.SetModule(common.ModuleName)
	return request
}

// ReleaseRequest 重置 Request 并将其归还到对象池。
func ReleaseRequest(req *Request) {
	if req != nil {
		common.LogDebug("Releasing request object to pool", "msg_uuid", req.GetMsgUUID())
		req.Reset()
		requestPool.Release(req)
	}
}

// Reset 将 Request 重置为零值状态以供对象池复用。
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

// Send 向指定的 UUID 发送同步请求，并阻塞直到收到响应
// 或超时到期。解码后的响应写入提供的 Response 指针中。
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

// SendAsync 发送一个发后即忘的异步请求。响应到达时
// 由服务器的 ResponseRouter 进行分发。
func (r *Request) SendAsync(sendUuid string, timeoutMs ...uint32) common.BoolResult {
	return r.encodeAndSend(sendUuid, common.MessageTypeRequestAsync, timeoutMs...)
}

// SendAsyncWithFunc 发送一个异步请求并注册一个回调函数，
// 该回调在响应到达时被调用。回调在请求存根表中跟踪，
// 并在过期时清理。
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

// SendAsyncWithFuture 发送一个异步请求并返回一个 ResponseFuture，
// 可用于阻塞直到响应到达。Future 在请求存根表中跟踪，
// 如果服务器未及时回复，将收到一个超时响应。
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

// encodeAndSend 将请求编码为传输格式并通过 IPC 发送。
// 被所有 send 方法内部使用。
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
