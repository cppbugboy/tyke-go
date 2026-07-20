package core

import (
	"sync/atomic"

	"tyke-go/common"
	"tyke-go/component"
	"tyke-go/ipc"
)

// SendDataHandler 是 Response 用于将编码后的数据发送回 IPC 客户端的回调。
type SendDataHandler func(clientId ipc.ClientId, data []byte) bool

// Response 表示一个 IPC 响应。它支持同步回复（通过 Send）
// 和异步回复（通过 SendAsync）。实例使用对象池管理。
type Response struct {
	protocolHeader  common.ProtocolHeader
	metadata        ResponseMetadata
	content         []byte
	isSend          atomic.Bool
	clientId        ipc.ClientId
	sendDataHandler SendDataHandler
}

var responsePool = component.NewObjectPool(func() *Response {
	return &Response{
		protocolHeader: common.ProtocolHeader{Magic: common.ProtocolMagic},
		metadata:       NewResponseMetadata(),
	}
})

func init() {
	responsePool.SetReset(func(resp *Response) {
		resp.Reset()
	})
}

// AcquireResponse 从对象池中获取一个 Response。
func AcquireResponse() *Response {
	return responsePool.Acquire()
}

// ReleaseResponse 重置 Response 并将其归还到对象池。
func ReleaseResponse(resp *Response) {
	if resp != nil {
		common.LogDebug("Releasing response object to pool", "msg_uuid", resp.GetMsgUUID())
		resp.Reset()
		responsePool.Release(resp)
	}
}

// Reset 将 Response 重置为零值状态以供对象池复用。
func (r *Response) Reset() {
	r.protocolHeader = common.ProtocolHeader{Magic: common.ProtocolMagic}
	r.metadata = NewResponseMetadata()
	r.content = nil
	r.isSend.Store(false)
	r.clientId = 0
	r.sendDataHandler = nil
}

func (r *Response) GetMagic() [4]byte {
	return r.protocolHeader.Magic
}

func (r *Response) SetMessageType(msgType common.MessageType) *Response {
	r.protocolHeader.MsgType = msgType
	return r
}

func (r *Response) GetMessageType() common.MessageType {
	return r.protocolHeader.MsgType
}

func (r *Response) SetModule(module string) *Response {
	r.metadata.SetModule(module)
	return r
}

func (r *Response) GetModule() string {
	return r.metadata.GetModule()
}

func (r *Response) SetMsgUUID(msgUuid string) *Response {
	r.metadata.SetMsgUUID(msgUuid)
	return r
}

func (r *Response) GetMsgUUID() string {
	return r.metadata.GetMsgUUID()
}

func (r *Response) SetRoute(route string) *Response {
	r.metadata.SetRoute(route)
	return r
}

func (r *Response) GetRoute() string {
	return r.metadata.GetRoute()
}

func (r *Response) SetContent(contentType common.ContentType, content []byte) *Response {
	r.metadata.SetContentType(common.ContentTypeMap[contentType])
	r.content = content
	return r
}

func (r *Response) GetContent() (string, []byte) {
	return r.metadata.GetContentType(), r.content
}

func (r *Response) AddMetadata(key string, value common.JsonValueHolder) common.BoolResult {
	return r.metadata.AddMetadata(key, value)
}

func (r *Response) GetMetadata(key string) (common.JsonValueHolder, bool) {
	return r.metadata.GetMetadata(key)
}

func (r *Response) SetResult(status int, reason string) *Response {
	r.metadata.SetStatus(status).SetReason(reason)
	return r
}

func (r *Response) GetResult() (int, string) {
	return r.metadata.GetStatus(), r.metadata.GetReason()
}

func (r *Response) SetAsyncUUID(asyncUuid string) *Response {
	r.metadata.AsyncUUID = asyncUuid
	return r
}

func (r *Response) GetAsyncUUID() string {
	return r.metadata.AsyncUUID
}

func (r *Response) SetSendDataHandler(handler SendDataHandler) *Response {
	r.sendDataHandler = handler
	return r
}

func (r *Response) SetClientId(clientId ipc.ClientId) *Response {
	r.clientId = clientId
	return r
}

func (r *Response) IsSent() bool {
	return r.isSend.Load()
}

// Send 使用配置的 sendDataHandler 同步地将响应发送回原始客户端。
// 如果响应已发送过或编码/传输失败则返回错误。
func (r *Response) Send() common.BoolResult {
	common.LogDebug("Send", "route", r.GetRoute(), "msg_uuid", r.GetMsgUUID())
	if !r.isSend.CompareAndSwap(false, true) {
		common.LogWarn("Response already sent", "msg_uuid", r.GetMsgUUID())
		return common.ErrBool("response already sent")
	}
	if r.sendDataHandler == nil {
		common.LogError("Send data handler is not set", "msg_uuid", r.GetMsgUUID())
		r.isSend.Store(false)
		return common.ErrBool("send data handler is not set")
	}
	r.metadata.SetTimestamp(common.GenerateTimestamp())
	dataVec, err := EncodeResponse(r)
	if err != nil {
		common.LogError("Encode response failed", "error", err)
		r.isSend.Store(false)
		return common.ErrBool("encode response failed")
	}
	if !r.sendDataHandler(r.clientId, dataVec) {
		common.LogError("Send data handler failed", "msg_uuid", r.GetMsgUUID())
		r.isSend.Store(false)
		return common.ErrBool("send data handler failed")
	}
	common.LogDebug("Response sent successfully", "msg_uuid", r.GetMsgUUID())
	return common.OkBool(true)
}

// SendAsync 通过 IPC 异步地将响应发送到异步 UUID。
// 用于发后即忘的异步响应模式。
func (r *Response) SendAsync() common.BoolResult {
	common.LogDebug("SendAsync", "route", r.GetRoute(), "msg_uuid", r.GetMsgUUID(), "asyncUuid", r.metadata.AsyncUUID)
	if !r.isSend.CompareAndSwap(false, true) {
		common.LogWarn("Response already sent", "msg_uuid", r.GetMsgUUID())
		return common.ErrBool("response already sent")
	}
	r.metadata.SetTimestamp(common.GenerateTimestamp())
	dataVec, err := EncodeResponse(r)
	if err != nil {
		common.LogError("Encode response failed", "error", err)
		r.isSend.Store(false)
		return common.ErrBool("encode response failed")
	}
	sendResult := ipc.IPCClientSendAsync(r.metadata.AsyncUUID, dataVec)
	if !sendResult.HasValue() {
		common.LogError("Send async failed", "error", sendResult.Err)
		r.isSend.Store(false)
		return common.ErrBool("send async failed: " + sendResult.Err)
	}
	common.LogDebug("Async response sent successfully", "msg_uuid", r.GetMsgUUID())
	return common.OkBool(true)
}
