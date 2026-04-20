package core

type ResponseHandlerFunc func(response *TykeResponse)

type ResponseRouteEntry = RouteEntry[ResponseFilter, ResponseHandlerFunc]

// ResponseRouterGroup 按模块分组管理响应路由。
type ResponseRouterGroup = RouterGroup[ResponseFilter, ResponseHandlerFunc]
