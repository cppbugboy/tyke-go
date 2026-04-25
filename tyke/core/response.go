package core

import (
	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/component"
	"github.com/tyke/tyke/tyke/ipc"
)

type SendDataHandler func(clientId ipc.ClientId, data []byte) bool

type Response struct {
	protocolHeader  common.ProtocolHeader
	metadata        ResponseMetadata
	content         []byte
	isSend          bool
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

func NewTykeResponse() *Response {
	return responsePool.Acquire()
}

func ReleaseResponse(resp *Response) {
	if resp != nil {
		common.LogDebug("Releasing response object to pool", "msg_uuid", resp.GetMsgUUID())
		resp.Reset()
		responsePool.Release(resp)
	}
}

func (r *Response) Reset() {
	r.protocolHeader = common.ProtocolHeader{Magic: common.ProtocolMagic}
	r.metadata = NewResponseMetadata()
	r.content = nil
	r.isSend = false
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

func (r *Response) AddMetadata(key string, value common.JsonValue) common.BoolResult {
	return r.metadata.AddMetadata(key, value)
}

func (r *Response) GetMetadata(key string) (common.JsonValue, bool) {
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

func (r *Response) Send() common.BoolResult {
	common.LogDebug("Send", "route", r.GetRoute(), "msg_uuid", r.GetMsgUUID())
	if r.isSend {
		common.LogWarn("Response already sent", "msg_uuid", r.GetMsgUUID())
		return common.ErrBool("response already sent")
	}
	if r.sendDataHandler == nil {
		common.LogError("Send data handler is not set", "msg_uuid", r.GetMsgUUID())
		return common.ErrBool("send data handler is not set")
	}
	r.metadata.SetTimestamp(common.GenerateTimestamp())
	dataVec, err := EncodeResponse(r)
	if err != nil {
		common.LogError("Encode response failed", "error", err)
		return common.ErrBool("encode response failed")
	}
	if !r.sendDataHandler(r.clientId, dataVec) {
		common.LogError("Send data handler failed", "msg_uuid", r.GetMsgUUID())
		return common.ErrBool("send data handler failed")
	}
	r.isSend = true
	common.LogDebug("Response sent successfully", "msg_uuid", r.GetMsgUUID())
	return common.OkBool(true)
}

func (r *Response) SendAsync() common.BoolResult {
	common.LogDebug("SendAsync", "route", r.GetRoute(), "msg_uuid", r.GetMsgUUID(), "asyncUuid", r.metadata.AsyncUUID)
	if r.isSend {
		common.LogWarn("Response already sent", "msg_uuid", r.GetMsgUUID())
		return common.ErrBool("response already sent")
	}
	r.metadata.SetTimestamp(common.GenerateTimestamp())
	dataVec, err := EncodeResponse(r)
	if err != nil {
		common.LogError("Encode response failed", "error", err)
		return common.ErrBool("encode response failed")
	}
	sendResult := ipc.IPCClientSendAsync(r.metadata.AsyncUUID, dataVec)
	if !sendResult.HasValue() {
		common.LogError("Send async failed", "error", sendResult.Err)
		return common.ErrBool("send async failed: " + sendResult.Err)
	}
	r.isSend = true
	common.LogDebug("Async response sent successfully", "msg_uuid", r.GetMsgUUID())
	return common.OkBool(true)
}
