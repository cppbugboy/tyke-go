package component

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cppbugboy/tyke-go/tyke/common"
)

const (
	DefaultQueueSize       = 4096
	MinQueueSize           = 256
	MaxQueueSize           = 65536
	DefaultScaleThreshold  = 0.8
	DefaultShrinkThreshold = 0.2
	DefaultScaleInterval   = 5 * time.Second
)

type TaskPriority int

const (
	PriorityLow    TaskPriority = iota
	PriorityMedium TaskPriority = iota
	PriorityHigh   TaskPriority = iota
)

type PoolState int32

const (
	StateIdle PoolState = iota
	StateRunning
	StateStopping
	StateStopped
)

type TaskWrapper struct {
	fn       func()
	priority TaskPriority
	enqTime  time.Time
}

type PoolMetrics struct {
	TotalTasksSubmitted  uint64
	TotalTasksCompleted  uint64
	TotalTasksDropped    uint64
	TotalTasksTimeout    uint64
	CurrentQueueSize     int32
	HighQueueSize        int32
	MediumQueueSize      int32
	LowQueueSize         int32
	CurrentActiveWorkers int32
	CurrentIdleWorkers   int32
	PeakQueueSize        int32
	PeakActiveWorkers    int32
	AverageTaskLatencyNs uint64
	ScaleUpCount         uint64
	ScaleDownCount       uint64
	QueueFullRejectCount uint64
}

type ScalingConfig struct {
	EnableAutoScale   bool
	ScaleThreshold    float64
	ShrinkThreshold   float64
	ScaleInterval     time.Duration
	MinWorkers        int
	MaxWorkers        int
	ScaleUpStep       int
	ScaleDownStep     int
	ScaleUpCooldown   time.Duration
	ScaleDownCooldown time.Duration
}

func DefaultScalingConfig() ScalingConfig {
	cpuCount := runtime.NumCPU()
	return ScalingConfig{
		EnableAutoScale:   true,
		ScaleThreshold:    DefaultScaleThreshold,
		ShrinkThreshold:   DefaultShrinkThreshold,
		ScaleInterval:     DefaultScaleInterval,
		MinWorkers:        cpuCount,
		MaxWorkers:        cpuCount * 8,
		ScaleUpStep:       2,
		ScaleDownStep:     1,
		ScaleUpCooldown:   2 * time.Second,
		ScaleDownCooldown: 10 * time.Second,
	}
}

type CoroutinePoolConfig struct {
	InitialWorkers int
	InitialQueue   int
	Scaling        ScalingConfig
	EnableMetrics  bool
	TaskTimeout    time.Duration
}

func DefaultCoroutinePoolConfig() CoroutinePoolConfig {
	cpuCount := runtime.NumCPU()
	return CoroutinePoolConfig{
		InitialWorkers: cpuCount,
		InitialQueue:   DefaultQueueSize,
		Scaling:        DefaultScalingConfig(),
		EnableMetrics:  true,
		TaskTimeout:    30 * time.Second,
	}
}

type atomicMetrics struct {
	totalTasksSubmitted  atomic.Uint64
	totalTasksCompleted  atomic.Uint64
	totalTasksDropped    atomic.Uint64
	totalTasksTimeout    atomic.Uint64
	queueFullRejectCount atomic.Uint64
	scaleUpCount         atomic.Uint64
	scaleDownCount       atomic.Uint64
}

type CoroutinePool struct {
	config      CoroutinePoolConfig
	state       atomic.Int32
	queueSize   atomic.Int32
	workers     int32
	activeTasks int32
	idleWorkers int32

	highQueue   []TaskWrapper
	mediumQueue []TaskWrapper
	lowQueue    []TaskWrapper
	queueMu     sync.Mutex
	queueCond   *sync.Cond

	wg            sync.WaitGroup
	stopCh        chan struct{}
	scaleTimer    *time.Timer
	lastScaleUp   time.Time
	lastScaleDown time.Time

	metrics       PoolMetrics
	metricsLock   sync.RWMutex
	atomicMetrics atomicMetrics

	onTaskPanic func(interface{})
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

func NewCoroutinePool(config CoroutinePoolConfig) *CoroutinePool {
	if config.InitialWorkers <= 0 {
		config.InitialWorkers = runtime.NumCPU()
	}
	if config.InitialQueue <= 0 {
		config.InitialQueue = DefaultQueueSize
	}
	if config.InitialQueue < MinQueueSize {
		config.InitialQueue = MinQueueSize
	}
	if config.InitialQueue > MaxQueueSize {
		config.InitialQueue = MaxQueueSize
	}

	p := &CoroutinePool{
		config:      config,
		highQueue:   make([]TaskWrapper, 0, config.InitialQueue/3),
		mediumQueue: make([]TaskWrapper, 0, config.InitialQueue/3),
		lowQueue:    make([]TaskWrapper, 0, config.InitialQueue/3),
		stopCh:      make(chan struct{}),
	}

	p.queueCond = sync.NewCond(&p.queueMu)
	p.onTaskPanic = defaultPanicHandler

	p.startWorkers(config.InitialWorkers)
	p.state.Store(int32(StateRunning))

	if config.Scaling.EnableAutoScale {
		p.startScalingLoop()
	}

	common.LogInfo("CoroutinePool created", "workers", config.InitialWorkers, "queue", config.InitialQueue, "priority_queues", "High/Medium/Low")
	return p
}

func defaultPanicHandler(v interface{}) {
	common.LogError("Task panic recovered", "error", fmt.Sprintf("%v", v))
}

func (tp *CoroutinePool) Init(threads int) {
	config := DefaultCoroutinePoolConfig()
	if threads > 0 {
		config.InitialWorkers = threads
	}
	tp.InitWithConfig(config)
}

func (tp *CoroutinePool) InitWithConfig(config CoroutinePoolConfig) {
	if tp.state.Load() != int32(StateIdle) {
		common.LogWarn("CoroutinePool already initialized or running")
		return
	}

	tp.config = config
	tp.highQueue = make([]TaskWrapper, 0, config.InitialQueue/3)
	tp.mediumQueue = make([]TaskWrapper, 0, config.InitialQueue/3)
	tp.lowQueue = make([]TaskWrapper, 0, config.InitialQueue/3)
	tp.queueCond = sync.NewCond(&tp.queueMu)
	tp.stopCh = make(chan struct{})
	tp.workers = 0

	tp.startWorkers(config.InitialWorkers)
	tp.state.Store(int32(StateRunning))

	if config.Scaling.EnableAutoScale {
		tp.startScalingLoop()
	}

	common.LogInfo("CoroutinePool initialized", "workers", config.InitialWorkers, "queue", config.InitialQueue, "priority_queues", "High/Medium/Low")
}

func (tp *CoroutinePool) startWorkers(count int) {
	for i := 0; i < count; i++ {
		tp.wg.Add(1)
		atomic.AddInt32(&tp.workers, 1)
		go tp.workerLoop()
	}
}

func (tp *CoroutinePool) workerLoop() {
	defer tp.wg.Done()
	defer atomic.AddInt32(&tp.workers, -1)

	for {
		select {
		case <-tp.stopCh:
			return
		default:
		}

		tp.queueMu.Lock()
		for tp.highQueue == nil && tp.mediumQueue == nil && tp.lowQueue == nil {
			tp.queueMu.Unlock()
			return
		}

		for len(tp.highQueue) == 0 && len(tp.mediumQueue) == 0 && len(tp.lowQueue) == 0 {
			select {
			case <-tp.stopCh:
				tp.queueMu.Unlock()
				return
			default:
			}

			atomic.AddInt32(&tp.idleWorkers, 1)
			tp.queueCond.Wait()
			atomic.AddInt32(&tp.idleWorkers, -1)

			select {
			case <-tp.stopCh:
				tp.queueMu.Unlock()
				return
			default:
			}
		}

		var task TaskWrapper
		if len(tp.highQueue) > 0 {
			task = tp.highQueue[0]
			tp.highQueue = tp.highQueue[1:]
		} else if len(tp.mediumQueue) > 0 {
			task = tp.mediumQueue[0]
			tp.mediumQueue = tp.mediumQueue[1:]
		} else if len(tp.lowQueue) > 0 {
			task = tp.lowQueue[0]
			tp.lowQueue = tp.lowQueue[1:]
		}
		tp.queueMu.Unlock()

		tp.queueSize.Add(-1)
		tp.executeTask(&task)
	}
}

func (tp *CoroutinePool) executeTask(task *TaskWrapper) {
	atomic.AddInt32(&tp.activeTasks, 1)
	tp.updatePeakMetrics()

	defer func() {
		atomic.AddInt32(&tp.activeTasks, -1)
		tp.recordTaskCompletion(task)
	}()

	defer func() {
		if r := recover(); r != nil {
			tp.recordTaskPanic()
			if tp.onTaskPanic != nil {
				tp.onTaskPanic(r)
			}
		}
	}()

	task.fn()
}

func (tp *CoroutinePool) Enqueue(f func()) bool {
	return tp.EnqueueWithPriority(f, PriorityMedium)
}

func (tp *CoroutinePool) EnqueueWithPriority(f func(), priority TaskPriority) bool {
	if tp.state.Load() != int32(StateRunning) {
		tp.recordTaskDropped()
		return false
	}

	task := TaskWrapper{
		fn:       f,
		priority: priority,
		enqTime:  time.Now(),
	}

	tp.queueMu.Lock()
	totalSize := len(tp.highQueue) + len(tp.mediumQueue) + len(tp.lowQueue)
	if totalSize >= tp.config.InitialQueue {
		tp.queueMu.Unlock()
		tp.recordQueueFull()
		return false
	}

	switch priority {
	case PriorityHigh:
		tp.highQueue = append(tp.highQueue, task)
	case PriorityMedium:
		tp.mediumQueue = append(tp.mediumQueue, task)
	case PriorityLow:
		tp.lowQueue = append(tp.lowQueue, task)
	default:
		tp.mediumQueue = append(tp.mediumQueue, task)
	}
	tp.queueMu.Unlock()

	tp.queueCond.Signal()
	tp.queueSize.Add(1)
	tp.recordTaskSubmitted()
	return true
}

func (tp *CoroutinePool) EnqueueWithContext(ctx context.Context, f func()) error {
	return tp.EnqueueWithContextAndPriority(ctx, f, PriorityMedium)
}

func (tp *CoroutinePool) EnqueueWithContextAndPriority(ctx context.Context, f func(), priority TaskPriority) error {
	if tp.state.Load() != int32(StateRunning) {
		tp.recordTaskDropped()
		return fmt.Errorf("pool is not running")
	}

	task := TaskWrapper{
		fn:       f,
		priority: priority,
		enqTime:  time.Now(),
	}

	if ctx != nil {
		select {
		case <-ctx.Done():
			tp.recordTaskTimeout()
			return ctx.Err()
		default:
		}
	}

	tp.queueMu.Lock()
	totalSize := len(tp.highQueue) + len(tp.mediumQueue) + len(tp.lowQueue)
	if totalSize >= tp.config.InitialQueue {
		tp.queueMu.Unlock()
		tp.recordQueueFull()
		return fmt.Errorf("queue is full")
	}

	switch priority {
	case PriorityHigh:
		tp.highQueue = append(tp.highQueue, task)
	case PriorityMedium:
		tp.mediumQueue = append(tp.mediumQueue, task)
	case PriorityLow:
		tp.lowQueue = append(tp.lowQueue, task)
	default:
		tp.mediumQueue = append(tp.mediumQueue, task)
	}
	tp.queueMu.Unlock()

	tp.queueCond.Signal()
	tp.queueSize.Add(1)
	tp.recordTaskSubmitted()
	return nil
}

func (tp *CoroutinePool) EnqueueWithTimeout(f func(), timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return tp.EnqueueWithContext(ctx, f) == nil
}

func (tp *CoroutinePool) EnqueueOrExecute(f func()) bool {
	return tp.EnqueueOrExecuteWithPriority(f, PriorityMedium)
}

func (tp *CoroutinePool) EnqueueOrExecuteWithPriority(f func(), priority TaskPriority) bool {
	if tp.state.Load() != int32(StateRunning) {
		f()
		return false
	}

	task := TaskWrapper{
		fn:       f,
		priority: priority,
		enqTime:  time.Now(),
	}

	tp.queueMu.Lock()
	totalSize := len(tp.highQueue) + len(tp.mediumQueue) + len(tp.lowQueue)
	if totalSize >= tp.config.InitialQueue {
		tp.queueMu.Unlock()
		tp.executeTaskInline(f)
		return false
	}

	switch priority {
	case PriorityHigh:
		tp.highQueue = append(tp.highQueue, task)
	case PriorityMedium:
		tp.mediumQueue = append(tp.mediumQueue, task)
	case PriorityLow:
		tp.lowQueue = append(tp.lowQueue, task)
	default:
		tp.mediumQueue = append(tp.mediumQueue, task)
	}
	tp.queueMu.Unlock()

	tp.queueCond.Signal()
	tp.queueSize.Add(1)
	tp.recordTaskSubmitted()
	return true
}

func (tp *CoroutinePool) executeTaskInline(f func()) {
	defer func() {
		if r := recover(); r != nil {
			tp.recordTaskPanic()
			if tp.onTaskPanic != nil {
				tp.onTaskPanic(r)
			}
		}
	}()
	tp.recordTaskSubmitted()
	f()
	tp.metricsLock.Lock()
	tp.metrics.TotalTasksCompleted++
	tp.metricsLock.Unlock()
}

func (tp *CoroutinePool) startScalingLoop() {
	tp.scaleTimer = time.NewTimer(tp.config.Scaling.ScaleInterval)
	go func() {
		for {
			select {
			case <-tp.stopCh:
				return
			case <-tp.scaleTimer.C:
				tp.checkAndScale()
				tp.scaleTimer.Reset(tp.config.Scaling.ScaleInterval)
			}
		}
	}()
}

func (tp *CoroutinePool) checkAndScale() {
	if !tp.config.Scaling.EnableAutoScale {
		return
	}

	currentWorkers := int(atomic.LoadInt32(&tp.workers))
	currentActive := int(atomic.LoadInt32(&tp.activeTasks))
	currentQueueSize := int(tp.queueSize.Load())

	loadFactor := float64(0)
	if currentWorkers > 0 {
		loadFactor = float64(currentActive) / float64(currentWorkers)
	}

	queueLoadFactor := float64(currentQueueSize) / float64(tp.config.InitialQueue)

	now := time.Now()

	if loadFactor >= tp.config.Scaling.ScaleThreshold || queueLoadFactor >= tp.config.Scaling.ScaleThreshold {
		if currentWorkers < tp.config.Scaling.MaxWorkers &&
			now.Sub(tp.lastScaleUp) >= tp.config.Scaling.ScaleUpCooldown {
			newWorkers := min(currentWorkers+tp.config.Scaling.ScaleUpStep, tp.config.Scaling.MaxWorkers)
			toAdd := newWorkers - currentWorkers
			if toAdd > 0 {
				tp.startWorkers(toAdd)
				tp.lastScaleUp = now
				tp.recordScaleUp()
				common.LogInfo("CoroutinePool scaling up", "from", currentWorkers, "to", newWorkers)
			}
		}
	} else if loadFactor <= tp.config.Scaling.ShrinkThreshold && queueLoadFactor <= tp.config.Scaling.ShrinkThreshold {
		if currentWorkers > tp.config.Scaling.MinWorkers &&
			now.Sub(tp.lastScaleDown) >= tp.config.Scaling.ScaleDownCooldown {
			common.LogInfo("CoroutinePool considering scale down", "workers", currentWorkers, "load", loadFactor)
		}
	}
}

func (tp *CoroutinePool) Stop(waitForTasks bool) {
	if !tp.state.CompareAndSwap(int32(StateRunning), int32(StateStopping)) {
		return
	}

	common.LogInfo("CoroutinePool stopping", "wait_for_tasks", waitForTasks)

	if tp.scaleTimer != nil {
		tp.scaleTimer.Stop()
	}

	close(tp.stopCh)

	tp.queueMu.Lock()
	tp.queueCond.Broadcast()
	tp.queueMu.Unlock()

	if waitForTasks {
		for tp.queueSize.Load() > 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	tp.wg.Wait()

	tp.queueMu.Lock()
	tp.highQueue = nil
	tp.mediumQueue = nil
	tp.lowQueue = nil
	tp.queueMu.Unlock()

	tp.state.Store(int32(StateStopped))

	common.LogInfo("CoroutinePool stopped", "metrics", tp.GetMetrics())
}

func (tp *CoroutinePool) GetMetrics() PoolMetrics {
	tp.metricsLock.RLock()
	defer tp.metricsLock.RUnlock()

	m := tp.metrics
	m.TotalTasksSubmitted = tp.atomicMetrics.totalTasksSubmitted.Load()
	m.TotalTasksCompleted = tp.atomicMetrics.totalTasksCompleted.Load()
	m.TotalTasksDropped = tp.atomicMetrics.totalTasksDropped.Load()
	m.TotalTasksTimeout = tp.atomicMetrics.totalTasksTimeout.Load()
	m.QueueFullRejectCount = tp.atomicMetrics.queueFullRejectCount.Load()
	m.ScaleUpCount = tp.atomicMetrics.scaleUpCount.Load()
	m.ScaleDownCount = tp.atomicMetrics.scaleDownCount.Load()
	m.CurrentQueueSize = tp.queueSize.Load()
	m.CurrentActiveWorkers = atomic.LoadInt32(&tp.activeTasks)
	m.CurrentIdleWorkers = atomic.LoadInt32(&tp.idleWorkers)

	tp.queueMu.Lock()
	m.HighQueueSize = int32(len(tp.highQueue))
	m.MediumQueueSize = int32(len(tp.mediumQueue))
	m.LowQueueSize = int32(len(tp.lowQueue))
	tp.queueMu.Unlock()

	return m
}

func (tp *CoroutinePool) GetQueueSize() int {
	return int(tp.queueSize.Load())
}

func (tp *CoroutinePool) GetQueueSizeByPriority(priority TaskPriority) int {
	tp.queueMu.Lock()
	defer tp.queueMu.Unlock()

	switch priority {
	case PriorityHigh:
		return len(tp.highQueue)
	case PriorityMedium:
		return len(tp.mediumQueue)
	case PriorityLow:
		return len(tp.lowQueue)
	default:
		return len(tp.mediumQueue)
	}
}

func (tp *CoroutinePool) GetWorkerCount() int {
	return int(atomic.LoadInt32(&tp.workers))
}

func (tp *CoroutinePool) GetActiveTaskCount() int {
	return int(atomic.LoadInt32(&tp.activeTasks))
}

func (tp *CoroutinePool) IsRunning() bool {
	return tp.state.Load() == int32(StateRunning)
}

func (tp *CoroutinePool) SetPanicHandler(handler func(interface{})) {
	tp.onTaskPanic = handler
}

func GetTaskPriorityByName(name string) TaskPriority {
	switch name {
	case "high", "High", "HIGH":
		return PriorityHigh
	case "low", "Low", "LOW":
		return PriorityLow
	default:
		return PriorityMedium
	}
}

func (tp *CoroutinePool) recordTaskSubmitted() {
	tp.atomicMetrics.totalTasksSubmitted.Add(1)
}

func (tp *CoroutinePool) recordTaskCompletion(task *TaskWrapper) {
	completed := tp.atomicMetrics.totalTasksCompleted.Add(1)
	latency := time.Since(task.enqTime).Nanoseconds()
	tp.metricsLock.Lock()
	avg := tp.metrics.AverageTaskLatencyNs
	tp.metrics.AverageTaskLatencyNs = (avg*(completed-1) + uint64(latency)) / completed
	tp.metricsLock.Unlock()
}

func (tp *CoroutinePool) recordTaskDropped() {
	tp.atomicMetrics.totalTasksDropped.Add(1)
}

func (tp *CoroutinePool) recordTaskTimeout() {
	tp.atomicMetrics.totalTasksTimeout.Add(1)
}

func (tp *CoroutinePool) recordQueueFull() {
	tp.atomicMetrics.queueFullRejectCount.Add(1)
}

func (tp *CoroutinePool) recordTaskPanic() {
	tp.atomicMetrics.totalTasksDropped.Add(1)
}

func (tp *CoroutinePool) recordScaleUp() {
	tp.atomicMetrics.scaleUpCount.Add(1)
}

func (tp *CoroutinePool) recordScaleDown() {
	tp.atomicMetrics.scaleDownCount.Add(1)
}

func (tp *CoroutinePool) updatePeakMetrics() {
	tp.metricsLock.Lock()
	active := atomic.LoadInt32(&tp.activeTasks)
	if active > tp.metrics.PeakActiveWorkers {
		tp.metrics.PeakActiveWorkers = active
	}
	queueSize := tp.queueSize.Load()
	if queueSize > tp.metrics.PeakQueueSize {
		tp.metrics.PeakQueueSize = queueSize
	}
	tp.metricsLock.Unlock()
}
