package core

import (
	"fmt"
	"time"

	"tyke-go/common"
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
	timer := time.NewTimer(time.Duration(timeoutMs) * time.Millisecond)
	defer timer.Stop()
	select {
	case resp := <-f.ch:
		return resp, nil
	case <-timer.C:
		common.LogWarn("GetResponse timeout", "msg_uuid", f.msgUuid, "timeout", timeoutMs)
		return nil, fmt.Errorf("GetResponse timeout for msg_uuid=%s after %dms", f.msgUuid, timeoutMs)
	}
}
