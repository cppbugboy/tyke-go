package core

import (
	"time"

	"github.com/tyke/tyke/tyke/common"
)

type ResponseFuture struct {
	msgUuid string
	ch      chan *TykeResponse
}

func NewResponseFuture(msgUuid string, ch chan *TykeResponse) ResponseFuture {
	return ResponseFuture{msgUuid: msgUuid, ch: ch}
}

func (f *ResponseFuture) GetResponse() *TykeResponse {
	return <-f.ch
}

func (f *ResponseFuture) GetResponseWithTimeout(timeoutMs uint32) (*TykeResponse, error) {
	select {
	case resp := <-f.ch:
		return resp, nil
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		common.LogWarn("GetResponse timeout", "msg_uuid", f.msgUuid, "timeout", timeoutMs)
		timeoutResp := NewTykeResponse()
		timeoutResp.SetMsgUuid(f.msgUuid)
		timeoutResp.SetResult(-1, "timeout")
		return timeoutResp, nil
	}
}
