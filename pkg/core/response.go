package core

import (
	"fmt"
	"sync"

	"github.com/tyke/tyke/pkg/common"
	"github.com/tyke/tyke/pkg/ipc"
)

type SendDataHandler func(ipc.ClientId, []byte) bool

type TykeResponse struct {
	msgType         common.MessageType
	metadata        *ResponseMetadata
	content         []byte
	dataSize        uint32
	isSend          bool
	clientId        ipc.ClientId
	sendDataHandler SendDataHandler
	targetUuid      string
}

var responsePool = sync.Pool{
	New: func() any {
		return &TykeResponse{
			metadata: NewResponseMetadata(),
			content:  make([]byte, 0),
		}
	},
}

func AcquireResponse() *TykeResponse {
	resp := responsePool.Get().(*TykeResponse)
	resp.Reset()
	return resp
}

func ReleaseResponse(resp *TykeResponse) {
	resp.Reset()
	responsePool.Put(resp)
}

func MakeResponsePtr() *TykeResponse {
	return AcquireResponse()
}

func (r *TykeResponse) Reset() {
	r.msgType = common.MessageTypeNone
	r.metadata = NewResponseMetadata()
	r.content = r.content[:0]
	r.dataSize = 0
	r.isSend = false
	r.clientId = 0
	r.sendDataHandler = nil
	r.targetUuid = ""
}

func (r *TykeResponse) SetMessageType(msgType common.MessageType) *TykeResponse {
	r.msgType = msgType
	return r
}

func (r *TykeResponse) GetMessageType() common.MessageType {
	return r.msgType
}

func (r *TykeResponse) SetModule(module string) *TykeResponse {
	r.metadata.Module = module
	return r
}

func (r *TykeResponse) GetModule() string {
	return r.metadata.Module
}

func (r *TykeResponse) SetMsgUuid(uuid string) *TykeResponse {
	r.metadata.MsgUUID = uuid
	return r
}

func (r *TykeResponse) GetMsgUuid() string {
	return r.metadata.MsgUUID
}

func (r *TykeResponse) SetRoute(route string) *TykeResponse {
	r.metadata.Route = route
	return r
}

func (r *TykeResponse) GetRoute() string {
	return r.metadata.Route
}

func (r *TykeResponse) SetContent(contentType common.ContentType, content []byte) *TykeResponse {
	r.metadata.ContentType = common.ContentTypeMap[contentType]
	r.content = make([]byte, len(content))
	copy(r.content, content)
	return r
}

func (r *TykeResponse) GetContent() (string, []byte) {
	return r.metadata.ContentType, r.content
}

func (r *TykeResponse) SetResult(status int, reason string) *TykeResponse {
	r.metadata.Status = status
	r.metadata.Reason = reason
	return r
}

func (r *TykeResponse) GetResult() (int, string) {
	return r.metadata.Status, r.metadata.Reason
}

func (r *TykeResponse) SetAsyncUuid(uuid string) *TykeResponse {
	r.targetUuid = uuid
	return r
}

func (r *TykeResponse) GetAsyncUuid() string {
	return r.targetUuid
}

func (r *TykeResponse) SetSendDataHandler(handler SendDataHandler) *TykeResponse {
	r.sendDataHandler = handler
	return r
}

func (r *TykeResponse) SetIpcFD(clientId ipc.ClientId) *TykeResponse {
	r.clientId = clientId
	return r
}

func (r *TykeResponse) AddMetadata(key string, value any) error {
	if r.metadata.Headers == nil {
		r.metadata.Headers = make(map[string]any)
	}
	r.metadata.Headers[key] = value
	return nil
}

func (r *TykeResponse) GetMetadata(key string) (any, bool) {
	if r.metadata.Headers == nil {
		return nil, false
	}
	val, ok := r.metadata.Headers[key]
	return val, ok
}

func (r *TykeResponse) Send() error {
	if r.isSend {
		return fmt.Errorf("response already sent")
	}

	encoded, err := EncodeResponse(r)
	if err != nil {
		return fmt.Errorf("response send: encode failed: %w", err)
	}

	if r.sendDataHandler != nil {
		if !r.sendDataHandler(r.clientId, encoded) {
			return fmt.Errorf("response send: send data handler returned false")
		}
	}

	r.isSend = true
	return nil
}

func (r *TykeResponse) SendAsync() error {
	if r.isSend {
		return fmt.Errorf("response already sent")
	}

	encoded, err := EncodeResponse(r)
	if err != nil {
		return fmt.Errorf("response send async: encode failed: %w", err)
	}

	if r.targetUuid != "" {
		go ipc.IpcClientSendAsync(r.targetUuid, encoded, common.DefaultTimeoutMs)
	}

	r.isSend = true
	return nil
}
