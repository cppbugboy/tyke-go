// Package component 提供tyke框架的核心可复用组件。
//
// 本文件实现了一个类似Go标准库context包的上下文系统，
// 但使用对象池来减少GC压力。支持取消、截止时间、超时和值传播。
//
// # 主要特性
//
// - CancelContext: 手动取消，支持回调
// - TimerContext: 在截止时间/超时后自动取消
// - ValueContext: 键值对传播
// - 所有上下文类型都使用对象池
// - 线程安全操作
//
// # 使用示例
//
//	// 创建可取消的上下文
//	ctx, cancel := component.ContextWithCancel(nil)
//	defer cancel()
//
//	// 创建超时上下文
//	ctx, cancel := component.ContextWithTimeout(nil, 5*time.Second)
//	defer cancel()
//
//	// 检查是否完成
//	if ctx.IsDone() {
//	    return ctx.Err()
//	}
//
// # 作者
//
// Nick
package component

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"tyke-go/common"
)

// ContextError 表示上下文被取消的原因。
type ContextError int

const (
	// ContextErrorNone 表示上下文仍然活跃。
	ContextErrorNone ContextError = iota
	// ContextErrorCanceled 表示上下文被手动取消。
	ContextErrorCanceled
	// ContextErrorDeadlineExceeded 表示上下文截止时间已过。
	ContextErrorDeadlineExceeded
)

// CancelToken 是已注册的取消回调的唯一标识符。
type CancelToken uint64

// InvalidCancelToken 表示无效或未设置的取消令牌。
const InvalidCancelToken CancelToken = 0

// Context 定义所有上下文类型的接口。
// 它提供了截止时间检查、取消、等待和值检索的方法。
type Context interface {
	// Deadline 返回上下文将被取消的时间（如果有）。
	// 如果没有设置截止时间，返回ok=false。
	Deadline() (time.Time, bool)

	// IsDone 如果上下文已被取消，返回true。
	IsDone() bool

	// Err 返回取消原因，如果未取消则返回ContextErrorNone。
	Err() ContextError

	// Wait 阻塞直到上下文被取消。
	Wait()

	// Value 检索与给定键关联的值。
	Value(key any) any

	// Reset 清除上下文状态以供复用。
	Reset()
}

// cancelState 保存取消管理的内部状态。
type cancelState struct {
	mu         sync.Mutex             // 保护所有字段
	cond       *sync.Cond             // 用于Wait()的条件变量
	atomicDone atomic.Bool            // 完成状态的快速检查
	err        ContextError           // 取消原因
	nextToken  atomic.Uint64          // 下一个回调令牌
	callbacks  map[CancelToken]func() // 已注册的回调
}

// newCancelState 创建一个新的cancelState实例。
func newCancelState() *cancelState {
	s := &cancelState{
		callbacks: make(map[CancelToken]func()),
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// Reset 清除取消状态以供复用。
func (s *cancelState) Reset() {
	s.mu.Lock()
	s.atomicDone.Store(false)
	s.err = ContextErrorNone
	s.nextToken.Store(1)
	s.callbacks = make(map[CancelToken]func())
	s.mu.Unlock()
}

// EmptyContext 是一个永远不会取消、没有截止时间或值的上下文。
// 它类似于标准库中的context.Background()。
type EmptyContext struct{}

// backgroundInstance 是单例EmptyContext实例。
var backgroundInstance = &EmptyContext{}

// Background 返回一个非nil的空上下文。它永远不会被取消。
func Background() *EmptyContext {
	return backgroundInstance
}

// TODO 返回一个非nil的空上下文。它是Background()的别名。
// 当你不确定使用哪个上下文时使用此函数。
func TODO() *EmptyContext {
	return backgroundInstance
}

// Deadline 返回(zero, false)，因为EmptyContext没有截止时间。
func (e *EmptyContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

// IsDone 总是返回false，因为EmptyContext永远不会取消。
func (e *EmptyContext) IsDone() bool {
	return false
}

// Err 总是返回ContextErrorNone。
func (e *EmptyContext) Err() ContextError {
	return ContextErrorNone
}

// Wait 立即返回，因为EmptyContext永远不会取消。
func (e *EmptyContext) Wait() {
}

// Value 总是返回nil，因为EmptyContext没有值。
func (e *EmptyContext) Value(key any) any {
	return nil
}

// Reset 对EmptyContext不做任何操作。
func (e *EmptyContext) Reset() {
}

// CancelContext 是一个可以手动取消的上下文。
// 它支持注册在取消时调用的回调函数。
type CancelContext struct {
	parent Context      // 可选的父上下文
	state  *cancelState // 内部取消状态
}

// cancelPool 是CancelContext实例的对象池。
var cancelPool = NewObjectPool(func() *CancelContext {
	return &CancelContext{state: newCancelState()}
})

// Init 使用可选的父上下文初始化CancelContext。
// 如果父上下文已经被取消，则此上下文立即被取消。
func (c *CancelContext) Init(parent Context) {
	c.parent = parent
	if parent != nil && parent.IsDone() {
		c.Cancel(parent.Err())
	}
}

// Reset 清除上下文以供对象池复用。
func (c *CancelContext) Reset() {
	c.state.Reset()
	c.parent = nil
}

// Deadline 返回父上下文的截止时间（如果有）。
func (c *CancelContext) Deadline() (time.Time, bool) {
	if c.parent != nil {
		return c.parent.Deadline()
	}
	return time.Time{}, false
}

// IsDone 如果此上下文或任何父上下文被取消，返回true。
func (c *CancelContext) IsDone() bool {
	return c.state.atomicDone.Load()
}

// Err 返回取消原因。
func (c *CancelContext) Err() ContextError {
	c.state.mu.Lock()
	err := c.state.err
	c.state.mu.Unlock()
	return err
}

// Wait 阻塞直到上下文被取消。
// 如果已经取消则立即返回。
func (c *CancelContext) Wait() {
	if c.IsDone() {
		return
	}
	c.state.mu.Lock()
	for !c.state.atomicDone.Load() {
		c.state.cond.Wait()
	}
	c.state.mu.Unlock()
}

// Value 从父上下文检索值（如果有）。
func (c *CancelContext) Value(key any) any {
	if c.parent != nil {
		return c.parent.Value(key)
	}
	return nil
}

// Cancel 以给定的错误原因取消上下文。
// 所有已注册的回调在新的goroutine中异步调用。
// 后续对Cancel的调用将被忽略。
func (c *CancelContext) Cancel(err ContextError) {
	var cbs []func()
	c.state.mu.Lock()
	if c.state.err != ContextErrorNone {
		c.state.mu.Unlock()
		return
	}
	c.state.err = err
	c.state.atomicDone.Store(true)
	c.state.cond.Broadcast()
	for token, cb := range c.state.callbacks {
		if cb != nil {
			cbs = append(cbs, cb)
		}
		_ = token
	}
	c.state.callbacks = make(map[CancelToken]func())
	c.state.mu.Unlock()

	for _, cb := range cbs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					common.LogError("Cancel callback panic recovered", "error", r)
				}
			}()
			cb()
		}()
	}
}

// RegisterCallback 注册一个在上下文取消时调用的函数。
// 返回一个可用于取消注册回调的令牌。
//
// 参数：
//   - cb: 取消时调用的回调函数。
//
// 返回：
//   - CancelToken: 用于取消注册回调的令牌。
func (c *CancelContext) RegisterCallback(cb func()) CancelToken {
	c.state.mu.Lock()
	token := CancelToken(c.state.nextToken.Add(1))
	c.state.callbacks[token] = cb
	c.state.mu.Unlock()
	return token
}

// UnregisterCallback 移除先前注册的回调。
//
// 参数：
//   - token: 从RegisterCallback返回的令牌。
func (c *CancelContext) UnregisterCallback(token CancelToken) {
	c.state.mu.Lock()
	delete(c.state.callbacks, token)
	c.state.mu.Unlock()
}

// TimerContext 是一个在截止时间自动取消的上下文。
// 它内嵌了CancelContext以支持手动取消。
type TimerContext struct {
	CancelContext
	deadline       time.Time     // 自动取消的时间
	timerId        atomic.Uint64 // 时间轮的定时器ID
	timerActivated atomic.Bool   // 定时器是否已激活
}

// timerPool 是TimerContext实例的对象池。
var timerPool = NewObjectPool(func() *TimerContext {
	return &TimerContext{CancelContext: CancelContext{state: newCancelState()}}
})

// Init 使用父上下文和截止时间初始化TimerContext。
// 有效截止时间是给定截止时间和父上下文截止时间中较早的一个。
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

// ActivateTimer 启动将在截止时间取消上下文的定时器。
// 这与Init分开调用，以允许延迟定时器激活。
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

// Reset 清除TimerContext以供复用，取消任何活动的定时器。
func (t *TimerContext) Reset() {
	if id := t.timerId.Load(); id != 0 {
		GetTimingWheel().CancelTask(TimerId(id))
		t.timerId.Store(0)
	}
	t.timerActivated.Store(false)
	t.deadline = time.Time{}
	t.CancelContext.Reset()
}

// Deadline 返回上下文的截止时间。
func (t *TimerContext) Deadline() (time.Time, bool) {
	return t.deadline, true
}

// ValueContext 是一个携带单个键值对的上下文。
// 它将所有其他操作委托给其父上下文。
type ValueContext struct {
	parent Context
	key    any
	value  any
}

// valuePool 是ValueContext实例的对象池。
var valuePool = NewObjectPool(func() *ValueContext {
	return &ValueContext{}
})

// Set 使用父上下文和键值对初始化ValueContext。
func (v *ValueContext) Set(parent Context, key any, val any) {
	v.parent = parent
	v.key = key
	v.value = val
}

// Reset 清除ValueContext以供复用。
func (v *ValueContext) Reset() {
	v.parent = nil
	v.key = nil
	v.value = nil
}

// Deadline 返回父上下文的截止时间（如果有）。
func (v *ValueContext) Deadline() (time.Time, bool) {
	if v.parent != nil {
		return v.parent.Deadline()
	}
	return time.Time{}, false
}

// IsDone 返回父上下文的完成状态。
func (v *ValueContext) IsDone() bool {
	if v.parent != nil {
		return v.parent.IsDone()
	}
	return false
}

// Err 返回父上下文的错误。
func (v *ValueContext) Err() ContextError {
	if v.parent != nil {
		return v.parent.Err()
	}
	return ContextErrorNone
}

// Wait 阻塞在父上下文的等待上。
func (v *ValueContext) Wait() {
	if v.parent != nil {
		v.parent.Wait()
	}
}

// Value 返回此上下文键对应的值，或委托给父上下文。
func (v *ValueContext) Value(key any) any {
	if key == v.key {
		return v.value
	}
	if v.parent != nil {
		return v.parent.Value(key)
	}
	return nil
}

// ContextPtr 是Context的别名，用于指针语义。
type ContextPtr = Context

// AcquireCancelContext 从对象池获取一个CancelContext。
// 使用完毕后调用ReleaseCancelContext。
//
// 返回：
//   - *CancelContext: 已重置的可用的CancelContext。
func AcquireCancelContext() *CancelContext {
	ctx := cancelPool.Acquire()
	return ctx
}

// ReleaseCancelContext 将CancelContext归还到对象池。
// 上下文在入池前会被重置。
//
// 参数：
//   - ctx: 要释放的上下文。nil为空操作。
func ReleaseCancelContext(ctx *CancelContext) {
	if ctx != nil {
		ctx.Reset()
		cancelPool.Release(ctx)
	}
}

// AcquireTimerContext 从对象池获取一个TimerContext。
// 使用完毕后调用ReleaseTimerContext。
//
// 返回：
//   - *TimerContext: 已重置的可用的TimerContext。
func AcquireTimerContext() *TimerContext {
	ctx := timerPool.Acquire()
	return ctx
}

// ReleaseTimerContext 将TimerContext归还到对象池。
// 上下文在入池前会被重置（包括取消定时器）。
//
// 参数：
//   - ctx: 要释放的上下文。nil为空操作。
func ReleaseTimerContext(ctx *TimerContext) {
	if ctx != nil {
		ctx.Reset()
		timerPool.Release(ctx)
	}
}

// AcquireValueContext 从对象池获取一个ValueContext。
// 使用完毕后调用ReleaseValueContext。
//
// 返回：
//   - *ValueContext: 已重置的可用的ValueContext。
func AcquireValueContext() *ValueContext {
	ctx := valuePool.Acquire()
	return ctx
}

// ReleaseValueContext 将ValueContext归还到对象池。
// 上下文在入池前会被重置。
//
// 参数：
//   - ctx: 要释放的上下文。nil为空操作。
func ReleaseValueContext(ctx *ValueContext) {
	if ctx != nil {
		ctx.Reset()
		valuePool.Release(ctx)
	}
}

// ContextWithCancel 创建一个具有给定父上下文的新的可取消上下文。
// 返回的取消函数可以安全地多次调用。
//
// 参数：
//   - parent: 父上下文。可以为nil。
//
// 返回：
//   - Context: 新的可取消上下文。
//   - func(): 完成时调用的取消函数。
//
// 示例：
//
//	ctx, cancel := component.ContextWithCancel(parent)
//	defer cancel()
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

// ContextWithDeadline 创建一个在指定截止时间取消的上下文。
// 返回的取消函数允许提前取消。
//
// 参数：
//   - parent: 父上下文。可以为nil。
//   - deadline: 上下文应该取消的时间。
//
// 返回：
//   - Context: 新的截止时间上下文。
//   - func(): 用于提前取消的取消函数。
//
// 示例：
//
//	deadline := time.Now().Add(30 * time.Second)
//	ctx, cancel := component.ContextWithDeadline(parent, deadline)
//	defer cancel()
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

// ContextWithTimeout 创建一个在指定持续时间后取消的上下文。
// 这是ContextWithDeadline的便捷包装。
//
// 参数：
//   - parent: 父上下文。可以为nil。
//   - timeout: 取消前的持续时间。
//
// 返回：
//   - Context: 新的超时上下文。
//   - func(): 用于提前取消的取消函数。
//
// 示例：
//
//	ctx, cancel := component.ContextWithTimeout(parent, 5*time.Second)
//	defer cancel()
func ContextWithTimeout(parent Context, timeout time.Duration) (Context, func()) {
	deadline := time.Now().Add(timeout)
	return ContextWithDeadline(parent, deadline)
}

// ContextWithValue 创建一个携带键值对的上下文。
// 通过遍历上下文树来检索值。
//
// 参数：
//   - parent: 父上下文。可以为nil。
//   - key: 值的键。使用自定义类型以避免冲突。
//   - val: 要存储的值。
//
// 返回：
//   - Context: 带有值的新上下文。
//
// 示例：
//
//	type requestIDKey struct{}
//	ctx := component.ContextWithValue(parent, requestIDKey{}, "req-123")
//	id := ctx.Value(requestIDKey{}) // 返回 "req-123"
func ContextWithValue(parent Context, key any, val any) Context {
	raw := AcquireValueContext()
	raw.Set(parent, key, val)
	return raw
}

func init() {
	common.LogDebug("Context package initialized")
}

// StdContext 将自定义 Context 适配为 Go 标准库的 context.Context 接口。
// 这使得 Tyke Context 可以与接受 context.Context 的第三方库和标准库函数互操作。
type StdContext struct {
	ctx Context
}

// NewStdContext 从自定义 Context 创建一个标准 context.Context。
func NewStdContext(ctx Context) context.Context {
	return &StdContext{ctx: ctx}
}

// Deadline 实现 context.Context.Deadline。
func (c *StdContext) Deadline() (time.Time, bool) {
	return c.ctx.Deadline()
}

// Done 实现 context.Context.Done。
// 返回一个在上下文被取消时关闭的通道。
func (c *StdContext) Done() <-chan struct{} {
	done := make(chan struct{})
	if c.ctx.IsDone() {
		close(done)
		return done
	}
	go func() {
		c.ctx.Wait()
		close(done)
	}()
	return done
}

// Err 实现 context.Context.Err。
// 将自定义 ContextError 映射为标准 context 错误。
func (c *StdContext) Err() error {
	switch c.ctx.Err() {
	case ContextErrorCanceled:
		return context.Canceled
	case ContextErrorDeadlineExceeded:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

// Value 实现 context.Context.Value。
func (c *StdContext) Value(key any) any {
	return c.ctx.Value(key)
}
