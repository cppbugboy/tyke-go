// Package component 提供了 Tyke 框架的基础组件，包括对象池、线程池、时间轮和单例模式。
package component

import "sync"

// ObjectPool 是泛型对象池，提供对象的复用和管理。
type ObjectPool[T any] struct {
	pool  []*T
	mutex sync.Mutex
	zero  func() *T
	max   int
}

// NewObjectPool 创建一个新的 ObjectPool 实例。
func NewObjectPool[T any](zero func() *T) *ObjectPool[T] {
	return &ObjectPool[T]{zero: zero, max: 0}
}

// NewObjectPoolWithMax 创建一个带最大容量的 ObjectPool 实例。
func NewObjectPoolWithMax[T any](zero func() *T, maxCapacity int) *ObjectPool[T] {
	return &ObjectPool[T]{zero: zero, max: maxCapacity}
}

func (p *ObjectPool[T]) Acquire() *T {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if len(p.pool) == 0 {
		return p.zero()
	}
	obj := p.pool[len(p.pool)-1]
	p.pool = p.pool[:len(p.pool)-1]
	return obj
}

func (p *ObjectPool[T]) Release(obj *T) {
	if obj == nil {
		return
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.max > 0 && len(p.pool) >= p.max {
		return
	}
	p.pool = append(p.pool, obj)
}

func (p *ObjectPool[T]) Clear() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.pool = nil
}
