// Package core implements the Tyke framework kernel.
//
// This file defines the filter interfaces used in the request and response
// routing chain. Filters can intercept both before and after handler execution.
package core

// RequestFilter is executed before and after a request handler. Return false
// from Before or After to short-circuit the filter chain.
type RequestFilter interface {
	Before(request *Request, response *Response) bool
	After(request *Request, response *Response) bool
}

// ResponseFilter is executed before and after a response handler. Return false
// from Before or After to short-circuit the filter chain.
type ResponseFilter interface {
	Before(response *Response) bool
	After(response *Response) bool
}
