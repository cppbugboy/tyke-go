package core

import (
	"time"

	"github.com/tyke/tyke/tyke/component"
)

type ContextFactory struct{}

func (ContextFactory) Background() component.Context {
	return component.Background()
}

func (ContextFactory) TODO() component.Context {
	return component.TODO()
}

func (ContextFactory) WithCancel(parent component.Context) (component.Context, func()) {
	return component.ContextWithCancel(parent)
}

func (ContextFactory) WithDeadline(parent component.Context, deadline time.Time) (component.Context, func()) {
	return component.ContextWithDeadline(parent, deadline)
}

func (ContextFactory) WithTimeout(parent component.Context, timeout time.Duration) (component.Context, func()) {
	return component.ContextWithTimeout(parent, timeout)
}

func (ContextFactory) WithValue(parent component.Context, key interface{}, value interface{}) component.Context {
	return component.ContextWithValue(parent, key, value)
}
