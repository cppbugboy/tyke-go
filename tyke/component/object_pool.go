// Package component 提供tyke框架的核心可复用组件。
//
// 本文件实现了一个泛型对象池，用于高效的对象复用。
// 对象池通过复用对象而非创建新对象来减少GC压力。
//
// # 主要特性
//
// - 泛型实现，支持任意类型
// - 使用互斥锁保证线程安全
// - 可选的重置函数，用于对象复用前的清理
// - 容量限制，防止无限增长
// - 指标统计，监控池效率
//
// # 使用示例
//
//	pool := component.NewObjectPool(func() *MyStruct {
//	    return &MyStruct{}
//	})
//	pool.SetReset(func(obj *MyStruct) {
//	    obj.Reset()
//	})
//
//	obj := pool.Acquire()
//	defer pool.Release(obj)
//	// 使用 obj...
//
// # 作者
//
// Nick
package component

import (
	"sync"
	"sync/atomic"

	"github.com/cppbugboy/tyke-go/tyke/common"
)

// ObjectPoolMetrics 包含对象池的运行时统计信息。
type ObjectPoolMetrics struct {
	TotalAcquire    uint64  // Acquire调用总次数
	TotalRelease    uint64  // Release调用总次数
	TotalCreate     uint64  // 创建对象总次数（缓存未命中）
	TotalReset      uint64  // 复用前重置对象总次数
	CurrentPoolSize int32   // 当前池中对象数量
	PeakPoolSize    int32   // 观察到的池大小峰值
	CacheHitRate    float64 // 缓存命中率
}

// ObjectPoolConfig 包含创建ObjectPool的配置选项。
type ObjectPoolConfig struct {
	InitialSize   int  // 池切片的初始容量
	MaxCapacity   int  // 保留的最大对象数
	EnableMetrics bool // 是否启用指标收集
}

// DefaultObjectPoolConfig 返回具有合理默认值的ObjectPoolConfig。
func DefaultObjectPoolConfig() ObjectPoolConfig {
	return ObjectPoolConfig{
		InitialSize:   16,
		MaxCapacity:   1024,
		EnableMetrics: true,
	}
}

// ObjectPool 是一个用于复用T类型对象的泛型池。
//
// 它维护一个池化对象的切片，并提供线程安全的Acquire和Release操作。
// 当池为空时，使用构造时提供的zero函数创建新对象。
//
// 类型参数T可以是任意类型，包括指针、结构体或基本类型。
type ObjectPool[T any] struct {
	pool    []T        // 池化对象切片
	mu      sync.Mutex // 保护池访问的互斥锁
	zero    func() T   // 创建新对象的工厂函数
	reset   func(T)    // 可选的复用前重置函数
	max     int        // 最大池容量
	metrics ObjectPoolMetrics
	config  ObjectPoolConfig

	acquireCount atomic.Uint64
	releaseCount atomic.Uint64
	createCount  atomic.Uint64
	resetCount   atomic.Uint64
	peakSize     atomic.Int32
}

// NewObjectPool 使用默认配置创建一个新的ObjectPool。
//
// 参数：
//   - zero: 创建T类型新零值对象的工厂函数。
//
// 返回：
//   - *ObjectPool[T]: 可使用的新对象池。
//
// 示例：
//
//	pool := NewObjectPool(func() *MyStruct {
//	    return &MyStruct{Field: "default"}
//	})
func NewObjectPool[T any](zero func() T) *ObjectPool[T] {
	return NewObjectPoolWithConfig(zero, DefaultObjectPoolConfig())
}

// NewObjectPoolWithMax 创建一个具有指定最大容量的ObjectPool。
//
// 参数：
//   - zero: 创建T类型新零值对象的工厂函数。
//   - maxCapacity: 池中保留的最大对象数。
//
// 返回：
//   - *ObjectPool[T]: 具有指定容量的新对象池。
func NewObjectPoolWithMax[T any](zero func() T, maxCapacity int) *ObjectPool[T] {
	config := DefaultObjectPoolConfig()
	config.MaxCapacity = maxCapacity
	return NewObjectPoolWithConfig(zero, config)
}

// NewObjectPoolWithConfig 使用自定义配置创建一个新的ObjectPool。
//
// 参数：
//   - zero: 创建T类型新零值对象的工厂函数。
//   - config: 池的配置选项。
//
// 返回：
//   - *ObjectPool[T]: 具有指定配置的新对象池。
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

// SetReset 设置对象复用前的重置函数。
//
// 重置函数在从池中取出对象时调用，用于在使用前重置其状态。
// 这对于清理不应在多次使用间保留的字段非常有用。
//
// 参数：
//   - reset: 将对象重置为干净状态的函数。
//
// 示例：
//
//	pool.SetReset(func(obj *MyStruct) {
//	    obj.Field = ""
//	    obj.Count = 0
//	})
func (p *ObjectPool[T]) SetReset(reset func(T)) {
	p.reset = reset
}

// Acquire 从池中获取一个对象，如果池为空则创建新对象。
//
// 如果池中包含对象，则取出一个并在重置后返回（如果设置了重置函数）。
// 如果池为空，则使用zero函数创建新对象。
//
// 返回：
//   - T: 可使用的对象。
//
// 线程安全：此方法可安全用于并发。
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

// Release 将对象归还到池中以供将来复用。
//
// 如果池已达到最大容量，则丢弃对象。
// 这可以防止对象释放速度快于获取速度时的无限内存增长。
//
// 参数：
//   - obj: 要归还到池中的对象。
//
// 线程安全：此方法可安全用于并发。
//
// 注意：在nil池上调用Release是空操作。
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

// Clear 清除池中的所有对象。
//
// 当需要垃圾回收池化对象时很有用，例如应用程序关闭或进入低内存状态时。
//
// 线程安全：此方法可安全用于并发。
func (p *ObjectPool[T]) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pool = nil
}

// Size 返回池中当前的对象数量。
//
// 返回：
//   - int: 可供复用的池化对象数量。
//
// 线程安全：此方法可安全用于并发。
func (p *ObjectPool[T]) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.pool)
}

// GetMetrics 返回池的运行时统计信息。
//
// 指标包括：
//   - 总获取和释放次数
//   - 缓存命中率（返回池化对象的获取操作百分比）
//   - 当前和峰值池大小
//
// 返回：
//   - ObjectPoolMetrics: 当前池统计信息的快照。
//
// 线程安全：此方法可安全用于并发。
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

// Preload 预先创建并添加对象到池中。
//
// 这对于在使用前预热池很有用，可减少正常运行期间的对象创建次数。
//
// 参数：
//   - count: 要创建并添加到池中的对象数量。
//
// 如果池达到最大容量，实际添加数量可能较少。
//
// 线程安全：此方法可安全用于并发。
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
