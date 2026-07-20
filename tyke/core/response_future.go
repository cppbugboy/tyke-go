// Package core 实现 Tyke 框架内核。
//
// 本文件定义了 ResponseFuture，一个类似 Promise 的类型，
// 可阻塞调用者直到异步响应到达或超时到期。
package core

import (
	"fmt"
	"time"

	"tyke-go/common"
)

// ResponseFuture 表示异步请求的 Future 结果。
// 提供阻塞方法以等待响应，可带可选的超时时间。
type ResponseFuture struct {
	msgUuid string
	ch      chan *Response
}

// NewResponseFuture 为指定的消息 UUID 和通道创建一个新的 ResponseFuture。
func NewResponseFuture(msgUuid string, ch chan *Response) ResponseFuture {
	return ResponseFuture{msgUuid: msgUuid, ch: ch}
}

// GetResponse 阻塞直到收到响应，使用默认存根超时时间。
func (f *ResponseFuture) GetResponse() *Response {
	resp, _ := f.GetResponseWithTimeout(common.DefaultStubTimeoutMs)
	return resp
}

// GetResponseWithTimeout 阻塞直到收到响应或指定的超时时间到期。
// 超时时返回错误。
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
