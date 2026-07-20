// Package core 实现 Tyke 框架内核。
//
// 本文件提供全局请求路由器的单例访问器。
package core

import "sync"

var (
	requestRouterInstance *RouterBase[RequestFilter, RequestHandlerFunc]
	requestRouterOnce     sync.Once
)

// GetRequestRouter 返回单例请求路由器实例。
func GetRequestRouter() *RouterBase[RequestFilter, RequestHandlerFunc] {
	requestRouterOnce.Do(func() {
		requestRouterInstance = NewRouterBase[RequestFilter, RequestHandlerFunc]()
	})
	return requestRouterInstance
}
