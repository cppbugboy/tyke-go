package component

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/cppbugboy/tyke-go/tyke/common"
)

type ContextError int

const (
	ContextErrorNone ContextError = iota
	ContextErrorCanceled
	ContextErrorDeadlineExceeded
)

type CancelToken uint64

const InvalidCancelToken CancelToken = 0

type Context interface {
	Deadline() (time.Time, bool)
	IsDone() bool
	Err() ContextError
	Wait()
	Value(key interface{}) interface{}
	Reset()
}

type cancelState struct {
	mu         sync.Mutex
	atomicDone atomic.Bool
	err        ContextError
	nextToken  atomic.Uint64
	callbacks  map[CancelToken]func()
}

func newCancelState() *cancelState {
	return &cancelState{
		callbacks: make(map[CancelToken]func()),
	}
}

func (s *cancelState) Reset() {
	s.mu.Lock()
	s.atomicDone.Store(false)
	s.err = ContextErrorNone
	s.nextToken.Store(1)
	s.callbacks = make(map[CancelToken]func())
	s.mu.Unlock()
}

type EmptyContext struct{}

var backgroundInstance = &EmptyContext{}

func Background() *EmptyContext {
	return backgroundInstance
}

func TODO() *EmptyContext {
	return backgroundInstance
}

func (e *EmptyContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (e *EmptyContext) IsDone() bool {
	return false
}

func (e *EmptyContext) Err() ContextError {
	return ContextErrorNone
}

func (e *EmptyContext) Wait() {
}

func (e *EmptyContext) Value(key interface{}) interface{} {
	return nil
}

func (e *EmptyContext) Reset() {
}

type CancelContext struct {
	parent Context
	state  *cancelState
}

var cancelPool = NewObjectPool(func() *CancelContext {
	return &CancelContext{state: newCancelState()}
})

func (c *CancelContext) Init(parent Context) {
	c.parent = parent
	if parent != nil && parent.IsDone() {
		c.Cancel(parent.Err())
	}
}

func (c *CancelContext) Reset() {
	c.state.Reset()
	c.parent = nil
}

func (c *CancelContext) Deadline() (time.Time, bool) {
	if c.parent != nil {
		return c.parent.Deadline()
	}
	return time.Time{}, false
}

func (c *CancelContext) IsDone() bool {
	return c.state.atomicDone.Load()
}

func (c *CancelContext) Err() ContextError {
	c.state.mu.Lock()
	err := c.state.err
	c.state.mu.Unlock()
	return err
}

func (c *CancelContext) Wait() {
	if c.IsDone() {
		return
	}
	c.state.mu.Lock()
	for !c.state.atomicDone.Load() {
		c.state.mu.Unlock()
		time.Sleep(1 * time.Millisecond)
		c.state.mu.Lock()
	}
	c.state.mu.Unlock()
}

func (c *CancelContext) Value(key interface{}) interface{} {
	if c.parent != nil {
		return c.parent.Value(key)
	}
	return nil
}

func (c *CancelContext) Cancel(err ContextError) {
	var cbs []func()
	c.state.mu.Lock()
	if c.state.err != ContextErrorNone {
		c.state.mu.Unlock()
		return
	}
	c.state.err = err
	c.state.atomicDone.Store(true)
	for token, cb := range c.state.callbacks {
		if cb != nil {
			cbs = append(cbs, cb)
		}
		_ = token
	}
	c.state.callbacks = make(map[CancelToken]func())
	c.state.mu.Unlock()

	for _, cb := range cbs {
		fn := cb
		go fn()
	}
}

func (c *CancelContext) RegisterCallback(cb func()) CancelToken {
	c.state.mu.Lock()
	token := CancelToken(c.state.nextToken.Add(1))
	c.state.callbacks[token] = cb
	c.state.mu.Unlock()
	return token
}

func (c *CancelContext) UnregisterCallback(token CancelToken) {
	c.state.mu.Lock()
	delete(c.state.callbacks, token)
	c.state.mu.Unlock()
}

type TimerContext struct {
	CancelContext
	deadline       time.Time
	timerId        atomic.Uint64
	timerActivated atomic.Bool
}

var timerPool = NewObjectPool(func() *TimerContext {
	return &TimerContext{CancelContext: CancelContext{state: newCancelState()}}
})

func (t *TimerContext) Init(parent Context, deadline time.Time) {
	t.CancelContext.Init(parent)

	effectiveDeadline := deadline
	if parent != nil {
		if parentDl, ok := parent.Deadline(); ok && parentDl.Before(effectiveDeadline) {
			effectiveDeadline = parentDl
		}
	}
	t.deadline = effectiveDeadline
	t.timerActivated.Store(false)
	t.timerId.Store(0)
}

func (t *TimerContext) ActivateTimer() {
	if t.timerActivated.Load() {
		return
	}
	if t.IsDone() {
		return
	}

	delay := time.Until(t.deadline)
	if delay <= 0 {
		t.Cancel(ContextErrorDeadlineExceeded)
		return
	}

	timeoutMs := uint32(delay.Milliseconds())
	if timeoutMs == 0 {
		timeoutMs = 1
	}

	tw := GetTimingWheel()
	tid := tw.AddTaskAt(time.Now().Add(delay), func() {
		t.Cancel(ContextErrorDeadlineExceeded)
	})

	t.timerId.Store(uint64(tid))
	t.timerActivated.Store(true)
}

func (t *TimerContext) Reset() {
	if id := t.timerId.Load(); id != 0 {
		GetTimingWheel().CancelTask(TimerId(id))
		t.timerId.Store(0)
	}
	t.timerActivated.Store(false)
	t.deadline = time.Time{}
	t.CancelContext.Reset()
}

func (t *TimerContext) Deadline() (time.Time, bool) {
	return t.deadline, true
}

type ValueContext struct {
	parent Context
	key    interface{}
	value  interface{}
}

var valuePool = NewObjectPool(func() *ValueContext {
	return &ValueContext{}
})

func (v *ValueContext) Set(parent Context, key interface{}, val interface{}) {
	v.parent = parent
	v.key = key
	v.value = val
}

func (v *ValueContext) Reset() {
	v.parent = nil
	v.key = nil
	v.value = nil
}

func (v *ValueContext) Deadline() (time.Time, bool) {
	if v.parent != nil {
		return v.parent.Deadline()
	}
	return time.Time{}, false
}

func (v *ValueContext) IsDone() bool {
	if v.parent != nil {
		return v.parent.IsDone()
	}
	return false
}

func (v *ValueContext) Err() ContextError {
	if v.parent != nil {
		return v.parent.Err()
	}
	return ContextErrorNone
}

func (v *ValueContext) Wait() {
	if v.parent != nil {
		v.parent.Wait()
	}
}

func (v *ValueContext) Value(key interface{}) interface{} {
	if key == v.key {
		return v.value
	}
	if v.parent != nil {
		return v.parent.Value(key)
	}
	return nil
}

type ContextPtr = Context

func AcquireCancelContext() *CancelContext {
	ctx := cancelPool.Acquire()
	return ctx
}

func ReleaseCancelContext(ctx *CancelContext) {
	if ctx != nil {
		ctx.Reset()
		cancelPool.Release(ctx)
	}
}

func AcquireTimerContext() *TimerContext {
	ctx := timerPool.Acquire()
	return ctx
}

func ReleaseTimerContext(ctx *TimerContext) {
	if ctx != nil {
		ctx.Reset()
		timerPool.Release(ctx)
	}
}

func AcquireValueContext() *ValueContext {
	ctx := valuePool.Acquire()
	return ctx
}

func ReleaseValueContext(ctx *ValueContext) {
	if ctx != nil {
		ctx.Reset()
		valuePool.Release(ctx)
	}
}

func ContextWithCancel(parent Context) (Context, func()) {
	raw := AcquireCancelContext()
	raw.Init(parent)
	cancelOnce := sync.Once{}
	cancelFunc := func() {
		cancelOnce.Do(func() {
			raw.Cancel(ContextErrorCanceled)
		})
	}
	return raw, cancelFunc
}

func ContextWithDeadline(parent Context, deadline time.Time) (Context, func()) {
	raw := AcquireTimerContext()
	raw.Init(parent, deadline)
	cancelOnce := sync.Once{}
	cancelFunc := func() {
		cancelOnce.Do(func() {
			raw.Cancel(ContextErrorCanceled)
		})
	}
	return raw, cancelFunc
}

func ContextWithTimeout(parent Context, timeout time.Duration) (Context, func()) {
	deadline := time.Now().Add(timeout)
	return ContextWithDeadline(parent, deadline)
}

func ContextWithValue(parent Context, key interface{}, val interface{}) Context {
	raw := AcquireValueContext()
	raw.Set(parent, key, val)
	return raw
}

func init() {
	common.LogDebug("Context package initialized")
}
