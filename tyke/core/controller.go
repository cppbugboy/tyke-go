// Package core implements the Tyke framework kernel.
//
// This file defines the ControllerBase interface and type aliases for request
// and response controllers.
package core

// ControllerBase is the base interface for controllers. Implementations register
// their route handlers in the RegisterMethod callback.
type ControllerBase interface {
	RegisterMethod()
}

// RequestController is a type alias for request-handling controllers.
type RequestController = ControllerBase

// ResponseController is a type alias for response-handling controllers.
type ResponseController = ControllerBase
