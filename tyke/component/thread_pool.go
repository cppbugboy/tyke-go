package component

import (
	"sync"
	"sync/atomic"
)

type ThreadPool struct {
	workers  int
	tasks    chan func()
	wg       sync.WaitGroup
	stopFlag atomic.Bool
	mu       sync.Mutex
	started  bool
}

var (
	threadPoolInstance *ThreadPool
	threadPoolOnce     sync.Once
)

func GetThreadPoolInstance() *ThreadPool {
	threadPoolOnce.Do(func() {
		threadPoolInstance = &ThreadPool{}
	})
	return threadPoolInstance
}

func (tp *ThreadPool) Init(threads int) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	if tp.started {
		return
	}
	tp.workers = threads
	tp.tasks = make(chan func(), 1024)
	tp.stopFlag.Store(false)
	tp.started = true

	for i := 0; i < threads; i++ {
		tp.wg.Add(1)
		go func() {
			defer tp.wg.Done()
			for task := range tp.tasks {
				task()
			}
		}()
	}
}

func (tp *ThreadPool) Stop(waitForTasks bool) {
	tp.mu.Lock()
	if !tp.started || tp.stopFlag.Swap(true) {
		tp.mu.Unlock()
		return
	}
	tp.started = false
	tp.mu.Unlock()

	if !waitForTasks {
		drain := make(chan func())
		close(drain)
		tp.tasks = drain
	} else {
		close(tp.tasks)
	}
	tp.wg.Wait()
}

func (tp *ThreadPool) Enqueue(f func()) bool {
	if tp.stopFlag.Load() {
		return false
	}
	tp.tasks <- f
	return true
}
