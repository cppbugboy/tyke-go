package core

import (
	"time"

	"github.com/tyke/tyke/tyke/common"
)

// ResponseFuture 表示异步请求的未来结果，可通过通道等待响应。
type ResponseFuture struct {
	msgUuid string
	ch      chan *Response
}

// NewResponseFuture 创建一个新的 ResponseFuture 实例。
func NewResponseFuture(msgUuid string, ch chan *Response) ResponseFuture {
	return ResponseFuture{msgUuid: msgUuid, ch: ch}
}

func (f *ResponseFuture) GetResponse() *Response {
	resp, _ := f.GetResponseWithTimeout(common.DefaultStubTimeoutMs)
	return resp
}

func (f *ResponseFuture) GetResponseWithTimeout(timeoutMs uint32) (*Response, error) {
	select {
	case resp := <-f.ch:
		return resp, nil
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		common.LogWarn("GetResponse timeout", "msg_uuid", f.msgUuid, "timeout", timeoutMs)
		timeoutResp := NewTykeResponse()
		timeoutResp.SetMsgUUID(f.msgUuid)
		timeoutResp.SetResult(-1, "timeout")
		return timeoutResp, nil
	}
}
