package core

type ResponseHandlerFunc func(response *TykeResponse)

type ResponseRouteEntry = RouteEntry[ResponseFilter, ResponseHandlerFunc]

type ResponseRouterGroup = RouterGroup[ResponseFilter, ResponseHandlerFunc]
