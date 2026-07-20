// Package core 实现 Tyke 框架内核。
//
// 本文件提供全局响应路由器的单例访问器。
package core

import "sync"

var (
	responseRouterInstance *RouterBase[ResponseFilter, ResponseHandlerFunc]
	responseRouterOnce     sync.Once
)

// GetResponseRouter 返回单例响应路由器实例。
func GetResponseRouter() *RouterBase[ResponseFilter, ResponseHandlerFunc] {
	responseRouterOnce.Do(func() {
		responseRouterInstance = NewRouterBase[ResponseFilter, ResponseHandlerFunc]()
	})
	return responseRouterInstance
}
