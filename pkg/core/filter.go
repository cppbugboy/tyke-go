// Copyright 2026 Tyke Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

// RequestFilter 请求过滤器接口。
//
// 请求过滤器用于在请求处理前后执行自定义逻辑，
// 如认证、授权、日志记录、参数校验等。
// 过滤器可以中断处理链，返回false表示不再继续处理。
//
// 实现此接口的类型可以注册到路由组中，
// 所有匹配该路由组的请求都会经过这些过滤器。
type RequestFilter interface {
	// Before 在请求处理器执行前调用。
	//
	// 参数:
	//   - req: 请求对象（只读）
	//   - resp: 响应对象（可修改，用于设置错误响应）
	//
	// 返回值:
	//   - bool: true表示继续处理链，false表示中断处理
	Before(req *TykeRequest, resp *TykeResponse) bool

	// After 在请求处理器执行后调用。
	//
	// 参数:
	//   - req: 请求对象（只读）
	//   - resp: 响应对象（可修改，用于修改响应）
	//
	// 返回值:
	//   - bool: true表示继续处理链，false表示中断处理
	After(req *TykeRequest, resp *TykeResponse) bool
}

// ResponseFilter 响应过滤器接口。
//
// 响应过滤器用于在响应处理前后执行自定义逻辑，
// 如响应日志记录、响应修改等。
// 过滤器可以中断处理链，返回false表示不再继续处理。
type ResponseFilter interface {
	// Before 在响应处理器执行前调用。
	//
	// 参数:
	//   - resp: 响应对象（可修改）
	//
	// 返回值:
	//   - bool: true表示继续处理链，false表示中断处理
	Before(resp *TykeResponse) bool

	// After 在响应处理器执行后调用。
	//
	// 参数:
	//   - resp: 响应对象（可修改）
	//
	// 返回值:
	//   - bool: true表示继续处理链，false表示中断处理
	After(resp *TykeResponse) bool
}
