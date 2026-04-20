// Package controller 实现了请求/响应控制器的注册和分发机制。
package controller

// ControllerBase 是控制器的基类，提供路由注册和处理器管理。
type ControllerBase interface {
	RegisterMethod()
}
