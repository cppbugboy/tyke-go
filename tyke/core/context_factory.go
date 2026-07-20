// Package core implements the Tyke framework kernel.
//
// This file provides ContextFactory, a convenience wrapper that delegates to
// the component package's context creation functions.
package core

import (
	"time"

	"tyke-go/component"
)

// ContextFactory provides convenience methods for creating Tyke Context instances.
type ContextFactory struct{}

// Background returns a non-nil empty context that is never cancelled.
func (ContextFactory) Background() component.Context {
	return component.Background()
}

// TODO returns a non-nil empty context. Alias for Background().
func (ContextFactory) TODO() component.Context {
	return component.TODO()
}

// WithCancel creates a cancellable context with the given parent.
func (ContextFactory) WithCancel(parent component.Context) (component.Context, func()) {
	return component.ContextWithCancel(parent)
}

// WithDeadline creates a context that cancels at the specified deadline.
func (ContextFactory) WithDeadline(parent component.Context, deadline time.Time) (component.Context, func()) {
	return component.ContextWithDeadline(parent, deadline)
}

// WithTimeout creates a context that cancels after the specified duration.
func (ContextFactory) WithTimeout(parent component.Context, timeout time.Duration) (component.Context, func()) {
	return component.ContextWithTimeout(parent, timeout)
}

// WithValue creates a context that carries the given key-value pair.
func (ContextFactory) WithValue(parent component.Context, key interface{}, value interface{}) component.Context {
	return component.ContextWithValue(parent, key, value)
}
