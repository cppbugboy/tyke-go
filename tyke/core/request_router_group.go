package core

type RequestHandlerFunc func(request *TykeRequest, response *TykeResponse)

type RequestRouteEntry = RouteEntry[RequestFilter, RequestHandlerFunc]

// RequestRouterGroup 按模块分组管理请求路由。
type RequestRouterGroup = RouterGroup[RequestFilter, RequestHandlerFunc]
