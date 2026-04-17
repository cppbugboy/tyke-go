package component

import "sync"

var (
	globalWorkerPool     *WorkerPool
	globalWorkerPoolOnce sync.Once
	globalWorkerPoolMu   sync.RWMutex
)

func InitWorkerPool(workers int) {
	globalWorkerPoolOnce.Do(func() {
		globalWorkerPoolMu.Lock()
		globalWorkerPool = NewWorkerPool(workers)
		globalWorkerPool.Start()
		globalWorkerPoolMu.Unlock()
	})
}

func GetWorkerPool() *WorkerPool {
	globalWorkerPoolMu.RLock()
	defer globalWorkerPoolMu.RUnlock()
	return globalWorkerPool
}

func StopWorkerPool(waitForTasks bool) {
	globalWorkerPoolMu.Lock()
	defer globalWorkerPoolMu.Unlock()
	if globalWorkerPool != nil {
		globalWorkerPool.Stop(waitForTasks)
		globalWorkerPool = nil
	}
}
