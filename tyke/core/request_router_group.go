// Package core implements the Tyke framework kernel.
//
// This file defines type aliases for request-specific routing: handler signature,
// route entry, and router group types.
package core

// RequestHandlerFunc is the signature for request route handlers.
type RequestHandlerFunc func(request *Request, response *Response)

// RequestRouteEntry is a route entry specialized for request handling.
type RequestRouteEntry = RouteEntry[RequestFilter, RequestHandlerFunc]

// RequestRouterGroup manages request routes grouped by module prefix.
type RequestRouterGroup = RouterGroup[RequestFilter, RequestHandlerFunc]
