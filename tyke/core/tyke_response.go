package core

import (
	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/component"
	"github.com/tyke/tyke/tyke/ipc"
)

type SendDataHandler func(clientId ipc.ClientId, data []byte) bool

type TykeResponse struct {
	protocolHeader  common.ProtocolHeader
	metadata        ResponseMetadata
	content         []byte
	isSend          bool
	clientId        ipc.ClientId
	sendDataHandler SendDataHandler
	targetUuid      string
}

var responsePool = component.NewObjectPool(func() TykeResponse {
	return TykeResponse{
		protocolHeader: common.ProtocolHeader{Magic: common.ProtocolMagic},
		metadata:       NewResponseMetadata(),
	}
})

func NewTykeResponse() *TykeResponse {
	obj := responsePool.Acquire()
	return &obj
}

func AcquireResponse() *TykeResponse {
	common.LogDebug("Acquiring response object from pool")
	return NewTykeResponse()
}

func ReleaseResponse(resp *TykeResponse) {
	if resp != nil {
		common.LogDebug("Releasing response object to pool", "msg_uuid", resp.GetMsgUuid())
		resp.Reset()
		obj := *resp
		responsePool.Release(obj)
	}
}

func (r *TykeResponse) Reset() {
	r.protocolHeader = common.ProtocolHeader{Magic: common.ProtocolMagic}
	r.metadata = NewResponseMetadata()
	r.content = nil
	r.isSend = false
	r.clientId = 0
	r.sendDataHandler = nil
	r.targetUuid = ""
}

func (r *TykeResponse) GetMagic() [4]byte {
	return r.protocolHeader.Magic
}

func (r *TykeResponse) SetMessageType(msgType common.MessageType) *TykeResponse {
	r.protocolHeader.MsgType = msgType
	return r
}

func (r *TykeResponse) GetMessageType() common.MessageType {
	return r.protocolHeader.MsgType
}

func (r *TykeResponse) SetModule(module string) *TykeResponse {
	r.metadata.SetModule(module)
	return r
}

func (r *TykeResponse) GetModule() string {
	return r.metadata.GetModule()
}

func (r *TykeResponse) SetMsgUuid(msgUuid string) *TykeResponse {
	r.metadata.SetMsgUuid(msgUuid)
	return r
}

func (r *TykeResponse) GetMsgUuid() string {
	return r.metadata.GetMsgUuid()
}

func (r *TykeResponse) SetRoute(route string) *TykeResponse {
	r.metadata.SetRoute(route)
	return r
}

func (r *TykeResponse) GetRoute() string {
	return r.metadata.GetRoute()
}

func (r *TykeResponse) SetContent(contentType common.ContentType, content []byte) *TykeResponse {
	r.metadata.SetContentType(common.ContentTypeMap[contentType])
	r.content = content
	return r
}

func (r *TykeResponse) GetContent() (string, []byte) {
	return r.metadata.GetContentType(), r.content
}

func (r *TykeResponse) AddMetadata(key string, value common.JsonValue) common.BoolResult {
	return r.metadata.AddMetadata(key, value)
}

func (r *TykeResponse) GetMetadata(key string) (common.JsonValue, bool) {
	return r.metadata.GetMetadata(key)
}

func (r *TykeResponse) SetResult(status int, reason string) *TykeResponse {
	r.metadata.SetStatus(status).SetReason(reason)
	return r
}

func (r *TykeResponse) GetResult() (int, string) {
	return r.metadata.GetStatus(), r.metadata.GetReason()
}

func (r *TykeResponse) SetAsyncUuid(targetUuid string) *TykeResponse {
	r.targetUuid = targetUuid
	return r
}

func (r *TykeResponse) GetAsyncUuid() string {
	return r.targetUuid
}

func (r *TykeResponse) SetSendDataHandler(handler SendDataHandler) *TykeResponse {
	r.sendDataHandler = handler
	return r
}

func (r *TykeResponse) SetClientId(clientId ipc.ClientId) *TykeResponse {
	r.clientId = clientId
	return r
}

func (r *TykeResponse) Send() common.BoolResult {
	common.LogDebug("Send", "route", r.GetRoute(), "msg_uuid", r.GetMsgUuid())
	if r.isSend {
		common.LogWarn("Response already sent", "msg_uuid", r.GetMsgUuid())
		return common.ErrBool("response already sent")
	}
	if r.sendDataHandler == nil {
		common.LogError("Send data handler is not set", "msg_uuid", r.GetMsgUuid())
		return common.ErrBool("send data handler is not set")
	}
	r.metadata.SetTimestamp(common.GenerateTimestamp())
	dataVec, err := EncodeResponse(r)
	if err != nil {
		common.LogError("Encode response failed", "error", err)
		return common.ErrBool("encode response failed")
	}
	if !r.sendDataHandler(r.clientId, dataVec) {
		common.LogError("Send data handler failed", "msg_uuid", r.GetMsgUuid())
		return common.ErrBool("send data handler failed")
	}
	r.isSend = true
	common.LogDebug("Response sent successfully", "msg_uuid", r.GetMsgUuid())
	return common.OkBool(true)
}

func (r *TykeResponse) SendAsync() common.BoolResult {
	common.LogDebug("SendAsync", "route", r.GetRoute(), "msg_uuid", r.GetMsgUuid(), "target_uuid", r.targetUuid)
	if r.isSend {
		common.LogWarn("Response already sent", "msg_uuid", r.GetMsgUuid())
		return common.ErrBool("response already sent")
	}
	r.metadata.SetTimestamp(common.GenerateTimestamp())
	dataVec, err := EncodeResponse(r)
	if err != nil {
		common.LogError("Encode response failed", "error", err)
		return common.ErrBool("encode response failed")
	}
	sendResult := ipc.IpcClientSendAsync(r.targetUuid, dataVec)
	if !sendResult.HasValue() {
		common.LogError("Send async failed", "error", sendResult.Err)
		return common.ErrBool("send async failed: " + sendResult.Err)
	}
	r.isSend = true
	common.LogDebug("Async response sent successfully", "msg_uuid", r.GetMsgUuid())
	return common.OkBool(true)
}
