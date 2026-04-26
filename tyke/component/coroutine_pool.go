package component

import (
	"sync"
	"sync/atomic"
)

type CoroutinePool struct {
	workers  int
	tasks    chan func()
	wg       sync.WaitGroup
	stopFlag atomic.Bool
	mu       sync.Mutex
	started  bool
}

var (
	threadPoolInstance *CoroutinePool
	threadPoolOnce     sync.Once
)

func GetThreadPoolInstance() *CoroutinePool {
	threadPoolOnce.Do(func() {
		threadPoolInstance = &CoroutinePool{}
	})
	return threadPoolInstance
}

func (tp *CoroutinePool) Init(threads int) {
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

func (tp *CoroutinePool) Stop(waitForTasks bool) {
	tp.mu.Lock()
	if !tp.started || tp.stopFlag.Swap(true) {
		tp.mu.Unlock()
		return
	}
	tp.started = false

	if waitForTasks {
		tp.mu.Unlock()
		close(tp.tasks)
	} else {
		tasks := tp.tasks
		tp.tasks = make(chan func(), 0)
		close(tp.tasks)
		tp.mu.Unlock()
		close(tasks)
	}
	tp.wg.Wait()
}

func (tp *CoroutinePool) Enqueue(f func()) bool {
	if tp.stopFlag.Load() {
		return false
	}
	tp.mu.Lock()
	tasks := tp.tasks
	tp.mu.Unlock()
	select {
	case tasks <- f:
		return true
	default:
		return false
	}
}
