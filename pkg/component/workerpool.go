// Copyright 2026 Tyke Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package component

import (
	"errors"
	"sync"
)

// ErrPoolStopped 工作池已停止错误。
// 当尝试向已停止的工作池提交任务时返回此错误。
var ErrPoolStopped = errors.New("worker pool is stopped")

// WorkerPool 工作池，管理固定数量的goroutine执行任务。
//
// WorkerPool提供以下功能：
//   - 固定数量的工作goroutine，避免无限制创建goroutine
//   - 任务队列缓冲，支持异步提交任务
//   - 优雅停止，可选择等待队列中的任务执行完毕
//
// 工作池适用于需要限制并发goroutine数量的场景，
// 如处理请求、执行异步回调等。
type WorkerPool struct {
	// workers 工作goroutine数量
	workers int
	// taskCh 任务通道，用于传递任务函数
	taskCh chan func()
	// wg 用于等待所有工作goroutine退出
	wg sync.WaitGroup
	// stopOnce 确保Stop只执行一次
	stopOnce sync.Once
	// stopped 标记工作池是否已停止
	stopped bool
	// mu 保护stopped字段的互斥锁
	mu sync.Mutex
}

// NewWorkerPool 创建一个新的工作池。
//
// 参数:
//   - workers: 工作goroutine数量，建议设置为CPU核心数或CPU核心数*2
//
// 返回值:
//   - *WorkerPool: 新创建的工作池指针
//
// 示例:
//
//	// 创建4个工作goroutine的池
//	pool := NewWorkerPool(4)
//	pool.Start()
func NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		workers: workers,
		taskCh:  make(chan func(), 1024),
	}
}

// Start 启动工作池。
//
// 启动指定数量的工作goroutine，开始处理任务。
// 必须在提交任务之前调用此方法。
//
// 示例:
//
//	pool := NewWorkerPool(4)
//	pool.Start()
//	pool.Submit(func() { fmt.Println("hello") })
func (p *WorkerPool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for task := range p.taskCh {
				task()
			}
		}()
	}
}

// Submit 提交一个任务到工作池。
//
// 任务将被放入队列，由空闲的工作goroutine执行。
// 如果工作池已停止，返回ErrPoolStopped错误。
//
// 参数:
//   - fn: 要执行的任务函数
//
// 返回值:
//   - error: 成功返回nil，工作池已停止返回ErrPoolStopped
//
// 示例:
//
//	err := pool.Submit(func() {
//	    // 执行任务
//	    processRequest()
//	})
//	if err != nil {
//	    log.Error("submit failed:", err)
//	}
func (p *WorkerPool) Submit(fn func()) error {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return ErrPoolStopped
	}
	p.mu.Unlock()
	p.taskCh <- fn
	return nil
}

// Stop 停止工作池。
//
// 停止接收新任务，并等待已提交的任务执行完毕。
// 参数waitForTasks控制是否等待队列中的任务执行完毕：
//   - true: 优雅停止，等待所有任务完成
//   - false: 立即停止，丢弃队列中未执行的任务
//
// 此方法可以安全地多次调用，只有第一次调用有效。
//
// 参数:
//   - waitForTasks: 是否等待队列中的任务执行完毕
//
// 示例:
//
//	// 优雅停止
//	pool.Stop(true)
//
//	// 立即停止
//	pool.Stop(false)
func (p *WorkerPool) Stop(waitForTasks bool) {
	p.stopOnce.Do(func() {
		p.mu.Lock()
		p.stopped = true
		p.mu.Unlock()
		if waitForTasks {
			close(p.taskCh)
		} else {
			close(p.taskCh)
		}
		p.wg.Wait()
	})
}
