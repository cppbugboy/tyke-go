package component

import "sync"

type ObjectPool[T any] struct {
	pool  []*T
	mutex sync.Mutex
	zero  func() *T
	max   int
}

func NewObjectPool[T any](zero func() *T) *ObjectPool[T] {
	return &ObjectPool[T]{zero: zero, max: 0}
}

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
