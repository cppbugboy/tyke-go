package component

import (
	"sync"
	"sync/atomic"

	"github.com/cppbugboy/tyke-go/tyke/common"
)

type ObjectPoolMetrics struct {
	TotalAcquire    uint64
	TotalRelease    uint64
	TotalCreate     uint64
	TotalReset      uint64
	CurrentPoolSize int32
	PeakPoolSize    int32
	CacheHitRate    float64
}

type ObjectPoolConfig struct {
	InitialSize   int
	MaxCapacity   int
	EnableMetrics bool
}

func DefaultObjectPoolConfig() ObjectPoolConfig {
	return ObjectPoolConfig{
		InitialSize:   16,
		MaxCapacity:   1024,
		EnableMetrics: true,
	}
}

type ObjectPool[T any] struct {
	pool    []T
	mu      sync.Mutex
	zero    func() T
	reset   func(T)
	max     int
	metrics ObjectPoolMetrics
	config  ObjectPoolConfig

	acquireCount atomic.Uint64
	releaseCount atomic.Uint64
	createCount  atomic.Uint64
	resetCount   atomic.Uint64
	peakSize     atomic.Int32
}

func NewObjectPool[T any](zero func() T) *ObjectPool[T] {
	return NewObjectPoolWithConfig(zero, DefaultObjectPoolConfig())
}

func NewObjectPoolWithMax[T any](zero func() T, maxCapacity int) *ObjectPool[T] {
	config := DefaultObjectPoolConfig()
	config.MaxCapacity = maxCapacity
	return NewObjectPoolWithConfig(zero, config)
}

func NewObjectPoolWithConfig[T any](zero func() T, config ObjectPoolConfig) *ObjectPool[T] {
	if config.InitialSize < 0 {
		config.InitialSize = 0
	}
	if config.MaxCapacity <= 0 {
		config.MaxCapacity = 1024
	}

	p := &ObjectPool[T]{
		zero:   zero,
		max:    config.MaxCapacity,
		config: config,
		pool:   make([]T, 0, config.InitialSize),
	}

	common.LogDebug("ObjectPool created", "initial_size", config.InitialSize, "max_capacity", config.MaxCapacity)
	return p
}

func (p *ObjectPool[T]) SetReset(reset func(T)) {
	p.reset = reset
}

func (p *ObjectPool[T]) Acquire() T {
	p.acquireCount.Add(1)

	p.mu.Lock()
	if len(p.pool) == 0 {
		p.mu.Unlock()
		p.createCount.Add(1)
		return p.zero()
	}

	obj := p.pool[len(p.pool)-1]
	p.pool = p.pool[:len(p.pool)-1]
	p.mu.Unlock()

	if p.reset != nil {
		p.reset(obj)
		p.resetCount.Add(1)
	}

	return obj
}

func (p *ObjectPool[T]) Release(obj T) {
	if p == nil {
		return
	}

	p.releaseCount.Add(1)

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.max > 0 && len(p.pool) >= p.max {
		return
	}

	p.pool = append(p.pool, obj)

	currentSize := int32(len(p.pool))
	for {
		peak := p.peakSize.Load()
		if currentSize <= peak {
			break
		}
		if p.peakSize.CompareAndSwap(peak, currentSize) {
			break
		}
	}
}

func (p *ObjectPool[T]) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pool = nil
}

func (p *ObjectPool[T]) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.pool)
}

func (p *ObjectPool[T]) GetMetrics() ObjectPoolMetrics {
	acquire := p.acquireCount.Load()
	release := p.releaseCount.Load()
	create := p.createCount.Load()
	reset := p.resetCount.Load()

	var hitRate float64
	if acquire > 0 {
		hitRate = float64(acquire-create) / float64(acquire)
	}

	p.mu.Lock()
	currentSize := int32(len(p.pool))
	p.mu.Unlock()

	return ObjectPoolMetrics{
		TotalAcquire:    acquire,
		TotalRelease:    release,
		TotalCreate:     create,
		TotalReset:      reset,
		CurrentPoolSize: currentSize,
		PeakPoolSize:    p.peakSize.Load(),
		CacheHitRate:    hitRate,
	}
}

func (p *ObjectPool[T]) Preload(count int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := 0; i < count && len(p.pool) < p.max; i++ {
		obj := p.zero()
		p.pool = append(p.pool, obj)
		p.createCount.Add(1)
	}

	common.LogDebug("ObjectPool preloaded", "count", count, "pool_size", len(p.pool))
}
