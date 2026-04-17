// Copyright 2026 Tyke Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package component 提供Tyke框架的基础组件实现。
//
// 本包包含对象池和工作池等通用组件，用于优化内存分配和并发任务执行。
// 这些组件是Tyke框架高性能运行的基础设施。
//
// 主要组件:
//   - ObjectPool: 泛型对象池，减少内存分配开销
//   - WorkerPool: 工作池，管理goroutine并发执行任务
//
// 作者: Nick
// 创建日期: 2026-04-17
// 最后修改: 2026-04-17
package component

import "sync"

// ObjectPool 泛型对象池，用于复用对象以减少内存分配开销。
//
// ObjectPool基于sync.Pool实现，具有以下特点：
//   - 线程安全：可安全地在多个goroutine中使用
//   - GC友好：池中的对象可能被垃圾回收器清理
//   - 自动扩展：池为空时自动创建新对象
//
// 使用对象池可以显著减少频繁创建和销毁对象的性能开销，
// 特别适用于请求对象、响应对象等频繁使用的类型。
//
// 类型参数:
//   - T: 池中存储的对象类型
type ObjectPool[T any] struct {
	pool sync.Pool
}

// NewObjectPool 创建一个新的对象池。
//
// 参数:
//   - newFunc: 创建新对象的工厂函数，当池为空时调用
//
// 返回值:
//   - *ObjectPool[T]: 新创建的对象池指针
//
// 示例:
//
//	pool := NewObjectPool(func() *MyObject {
//	    return &MyObject{Field: "default"}
//	})
func NewObjectPool[T any](newFunc func() *T) *ObjectPool[T] {
	return &ObjectPool[T]{
		pool: sync.Pool{
			New: func() any {
				return newFunc()
			},
		},
	}
}

// Acquire 从池中获取一个对象。
//
// 如果池中有可用对象，则返回该对象；
// 否则调用工厂函数创建一个新对象。
// 获取的对象应该在使用完毕后通过Release归还到池中。
//
// 返回值:
//   - *T: 获取的对象指针
//
// 示例:
//
//	obj := pool.Acquire()
//	// 使用obj...
//	pool.Release(obj)
func (p *ObjectPool[T]) Acquire() *T {
	return p.pool.Get().(*T)
}

// Release 将对象归还到池中。
//
// 归还的对象可能被后续的Acquire调用重用。
// 注意：归还前应该重置对象状态，避免数据污染。
//
// 参数:
//   - obj: 要归还的对象指针
//
// 示例:
//
//	obj := pool.Acquire()
//	obj.Reset() // 重置对象状态
//	pool.Release(obj)
func (p *ObjectPool[T]) Release(obj *T) {
	p.pool.Put(obj)
}
