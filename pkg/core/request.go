package core

import (
	"fmt"
	"sync"

	"github.com/tyke/tyke/pkg/common"
	"github.com/tyke/tyke/pkg/ipc"
)

type TykeRequest struct {
	msgType  common.MessageType
	metadata *RequestMetadata
	content  []byte
}

var requestPool = sync.Pool{
	New: func() any {
		return &TykeRequest{
			metadata: NewRequestMetadata(),
			content:  make([]byte, 0),
		}
	},
}

func AcquireRequest() *TykeRequest {
	req := requestPool.Get().(*TykeRequest)
	req.Reset()
	return req
}

func ReleaseRequest(req *TykeRequest) {
	req.Reset()
	requestPool.Put(req)
}

func MakeRequestPtr() *TykeRequest {
	return AcquireRequest()
}

func (r *TykeRequest) Reset() {
	r.msgType = common.MessageTypeNone
	r.metadata = NewRequestMetadata()
	r.content = r.content[:0]
}

func (r *TykeRequest) SetModule(module string) *TykeRequest {
	r.metadata.Module = module
	return r
}

func (r *TykeRequest) GetModule() string {
	return r.metadata.Module
}

func (r *TykeRequest) SetRoute(route string) *TykeRequest {
	r.metadata.Route = route
	return r
}

func (r *TykeRequest) GetRoute() string {
	return r.metadata.Route
}

func (r *TykeRequest) SetContent(contentType common.ContentType, content []byte) *TykeRequest {
	r.metadata.ContentType = common.ContentTypeMap[contentType]
	r.content = make([]byte, len(content))
	copy(r.content, content)
	return r
}

func (r *TykeRequest) GetContent() (string, []byte) {
	return r.metadata.ContentType, r.content
}

func (r *TykeRequest) GetMsgUuid() string {
	return r.metadata.MsgUUID
}

func (r *TykeRequest) GetAsyncUuid() string {
	return r.metadata.AsyncUUID
}

func (r *TykeRequest) SetMsgUuid(uuid string) *TykeRequest {
	r.metadata.MsgUUID = uuid
	return r
}

func (r *TykeRequest) AddMetadata(key string, value any) error {
	if r.metadata.Headers == nil {
		r.metadata.Headers = make(map[string]any)
	}
	r.metadata.Headers[key] = value
	return nil
}

func (r *TykeRequest) GetMetadata(key string) (any, bool) {
	if r.metadata.Headers == nil {
		return nil, false
	}
	val, ok := r.metadata.Headers[key]
	return val, ok
}

func (r *TykeRequest) GetMessageType() common.MessageType {
	return r.msgType
}

func (r *TykeRequest) Send(sendUuid string, response *TykeResponse) error {
	r.msgType = common.MessageTypeRequest
	r.metadata.MsgUUID = common.GenerateUUID()
	r.metadata.Timestamp = common.GenerateTimestamp()

	encoded, err := EncodeRequest(r)
	if err != nil {
		return fmt.Errorf("send: encode failed: %w", err)
	}

	callback := func(data []byte) bool {
		resp, _, decErr := DecodeResponse(data)
		if decErr != nil {
			return false
		}
		*response = *resp
		ReleaseResponse(resp)
		return true
	}

	if err := ipc.IpcClientSend(sendUuid, encoded, callback, common.DefaultTimeoutMs); err != nil {
		return fmt.Errorf("send: ipc send failed: %w", err)
	}

	return nil
}

func (r *TykeRequest) SendAsync(sendUuid string, recvUuid string) error {
	r.msgType = common.MessageTypeRequestAsync
	r.metadata.MsgUUID = common.GenerateUUID()
	r.metadata.Timestamp = common.GenerateTimestamp()
	r.metadata.AsyncUUID = recvUuid

	encoded, err := EncodeRequest(r)
	if err != nil {
		return fmt.Errorf("send async: encode failed: %w", err)
	}

	return ipc.IpcClientSendAsync(sendUuid, encoded, common.DefaultTimeoutMs)
}

func (r *TykeRequest) SendAsyncWithFunc(sendUuid string, fn func(*TykeResponse)) error {
	r.msgType = common.MessageTypeRequestAsyncFunc
	r.metadata.MsgUUID = common.GenerateUUID()
	r.metadata.Timestamp = common.GenerateTimestamp()

	GetRequestStub().AddFunc(r.metadata.MsgUUID, fn)

	encoded, err := EncodeRequest(r)
	if err != nil {
		GetRequestStub().DelFunc(r.metadata.MsgUUID)
		return fmt.Errorf("send async with func: encode failed: %w", err)
	}

	if err := ipc.IpcClientSendAsync(sendUuid, encoded, common.DefaultTimeoutMs); err != nil {
		GetRequestStub().DelFunc(r.metadata.MsgUUID)
		return fmt.Errorf("send async with func: ipc send failed: %w", err)
	}

	return nil
}

func (r *TykeRequest) SendAsyncWithFuture(sendUuid string, recvUuid string) (<-chan *TykeResponse, error) {
	r.msgType = common.MessageTypeRequestAsyncFuture
	r.metadata.MsgUUID = common.GenerateUUID()
	r.metadata.Timestamp = common.GenerateTimestamp()
	r.metadata.AsyncUUID = recvUuid

	ch := make(chan *TykeResponse, 1)
	GetRequestStub().AddFuture(r.metadata.MsgUUID, ch)

	encoded, err := EncodeRequest(r)
	if err != nil {
		GetRequestStub().DelFuture(r.metadata.MsgUUID)
		return nil, fmt.Errorf("send async with future: encode failed: %w", err)
	}

	if err := ipc.IpcClientSendAsync(sendUuid, encoded, common.DefaultTimeoutMs); err != nil {
		GetRequestStub().DelFuture(r.metadata.MsgUUID)
		return nil, fmt.Errorf("send async with future: ipc send failed: %w", err)
	}

	return ch, nil
}
