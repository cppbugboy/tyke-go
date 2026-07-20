// Package core implements the Tyke framework kernel.
//
// This file defines type aliases for response-specific routing: handler signature,
// route entry, and router group types.
package core

// ResponseHandlerFunc is the signature for response route handlers.
type ResponseHandlerFunc func(response *Response)

// ResponseRouteEntry is a route entry specialized for response handling.
type ResponseRouteEntry = RouteEntry[ResponseFilter, ResponseHandlerFunc]

// ResponseRouterGroup manages response routes grouped by module prefix.
type ResponseRouterGroup = RouterGroup[ResponseFilter, ResponseHandlerFunc]
