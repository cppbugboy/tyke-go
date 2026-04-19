package core

type RequestHandlerFunc func(request *TykeRequest, response *TykeResponse)

type RequestRouteEntry = RouteEntry[RequestFilter, RequestHandlerFunc]

type RequestRouterGroup = RouterGroup[RequestFilter, RequestHandlerFunc]
