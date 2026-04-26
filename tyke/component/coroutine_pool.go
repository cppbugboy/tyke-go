// Package component 提供tyke框架的核心可复用组件。
//
// 本文件实现了一个高性能优先级协程池，支持高/中/低三级优先级调度。
//
// # 主要特性
//
// - 三分离优先级队列（High/Medium/Low），严格按优先级调度
// - 高优先级任务始终优先于中、低优先级任务执行
// - 使用互斥锁和条件变量保证线程安全的队列操作
// - 基于负载的动态协程扩缩容（自动伸缩）
// - 优雅降级（队列满时内联执行）
// - 完善的指标统计，包含各优先级队列大小
//
// # 使用示例
//
//	pool := component.NewCoroutinePool(component.DefaultCoroutinePoolConfig())
//	defer pool.Stop(true)
//
//	// 提交高优先级任务
//	pool.EnqueueWithPriority(func() {
//	    handleUrgentRequest()
//	}, component.PriorityHigh)
//
//	// 提交默认优先级任务（Medium）
//	pool.Enqueue(func() {
//	    processNormalTask()
//	})
//
// # 作者
//
// # Nick
//
// # 版本
//
// 2.0
package component

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cppbugboy/tyke-go/tyke/common"
)

// 队列和扩缩容相关常量，使用合理的默认值。
const (
	DefaultQueueSize       = 4096            // 默认队列容量
	MinQueueSize           = 256             // 最小队列容量
	MaxQueueSize           = 65536           // 最大队列容量
	DefaultScaleThreshold  = 0.8             // 默认扩容阈值
	DefaultShrinkThreshold = 0.2             // 默认缩容阈值
	DefaultScaleInterval   = 5 * time.Second // 默认扩缩容检查间隔
)

// TaskPriority 定义任务的优先级级别。
// 数值越大优先级越高。
// 工作协程始终按 High -> Medium -> Low 的顺序获取任务。
type TaskPriority int

const (
	// PriorityLow 低优先级：适用于日志清理、数据归档等后台任务。
	PriorityLow TaskPriority = iota
	// PriorityMedium 中优先级（默认）：适用于普通业务逻辑处理。
	PriorityMedium TaskPriority = iota
	// PriorityHigh 高优先级：适用于紧急任务、关键路径操作。
	PriorityHigh TaskPriority = iota
)

// PoolState 表示协程池的当前状态。
type PoolState int32

const (
	// StateIdle 表示协程池尚未初始化。
	StateIdle PoolState = iota
	// StateRunning 表示协程池正在处理任务。
	StateRunning
	// StateStopping 表示协程池正在关闭过程中。
	StateStopping
	// StateStopped 表示协程池已完全停止。
	StateStopped
)

// TaskWrapper 包装任务函数及其优先级和入队时间。
type TaskWrapper struct {
	fn       func()       // 要执行的任务函数
	priority TaskPriority // 任务优先级
	enqTime  time.Time    // 任务入队时间（用于延迟统计）
}

// PoolMetrics 包含协程池的运行时统计指标。
type PoolMetrics struct {
	TotalTasksSubmitted  uint64 // 累计提交任务数
	TotalTasksCompleted  uint64 // 累计完成任务数
	TotalTasksDropped    uint64 // 累计丢弃任务数（池停止时提交）
	TotalTasksTimeout    uint64 // 累计超时任务数
	CurrentQueueSize     int32  // 当前队列总大小
	HighQueueSize        int32  // 当前高优先级队列大小
	MediumQueueSize      int32  // 当前中优先级队列大小
	LowQueueSize         int32  // 当前低优先级队列大小
	CurrentActiveWorkers int32  // 当前活跃工作协程数
	CurrentIdleWorkers   int32  // 当前空闲工作协程数
	PeakQueueSize        int32  // 队列大小峰值
	PeakActiveWorkers    int32  // 活跃协程数峰值
	AverageTaskLatencyNs uint64 // 平均任务延迟（纳秒）
	ScaleUpCount         uint64 // 扩容次数
	ScaleDownCount       uint64 // 缩容次数
	QueueFullRejectCount uint64 // 队列满拒绝次数
}

// ScalingConfig 配置协程池的自动扩缩容行为。
type ScalingConfig struct {
	EnableAutoScale   bool          // 是否启用自动扩缩容
	ScaleThreshold    float64       // 扩容阈值（负载因子）
	ShrinkThreshold   float64       // 缩容阈值（负载因子）
	ScaleInterval     time.Duration // 扩缩容检查间隔
	MinWorkers        int           // 最小工作协程数
	MaxWorkers        int           // 最大工作协程数
	ScaleUpStep       int           // 每次扩容增加的协程数
	ScaleDownStep     int           // 每次缩容减少的协程数
	ScaleUpCooldown   time.Duration // 扩容冷却时间
	ScaleDownCooldown time.Duration // 缩容冷却时间
}

// DefaultScalingConfig 返回基于CPU核心数的默认扩缩容配置。
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

// CoroutinePoolConfig 包含创建协程池的所有配置选项。
type CoroutinePoolConfig struct {
	InitialWorkers int           // 初始工作协程数
	InitialQueue   int           // 初始队列容量
	Scaling        ScalingConfig // 自动扩缩容配置
	EnableMetrics  bool          // 是否启用指标统计
	TaskTimeout    time.Duration // 默认任务超时时间
}

// DefaultCoroutinePoolConfig 返回合理的默认协程池配置。
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

// atomicMetrics 提供无锁计数器用于高频指标更新。
type atomicMetrics struct {
	totalTasksSubmitted  atomic.Uint64
	totalTasksCompleted  atomic.Uint64
	totalTasksDropped    atomic.Uint64
	totalTasksTimeout    atomic.Uint64
	queueFullRejectCount atomic.Uint64
	scaleUpCount         atomic.Uint64
	scaleDownCount       atomic.Uint64
}

// CoroutinePool 是一个支持三级优先级队列的协程池。
//
// 它确保高优先级任务始终优先于中、低优先级任务执行，
// 中优先级任务优先于低优先级任务执行。
//
// 通过互斥锁和条件变量保证线程安全。
type CoroutinePool struct {
	config      CoroutinePoolConfig
	state       atomic.Int32
	queueSize   atomic.Int32
	workers     int32
	activeTasks int32
	idleWorkers int32

	highQueue   []TaskWrapper // 高优先级任务队列
	mediumQueue []TaskWrapper // 中优先级任务队列
	lowQueue    []TaskWrapper // 低优先级任务队列
	queueMu     sync.Mutex    // 保护三个队列的互斥锁
	queueCond   *sync.Cond    // 队列等待条件变量

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

// GetCoroutinePoolInstance 返回全局单例协程池实例。
//
// 单例在首次调用时延迟初始化。用于应用程序级别的任务调度。
//
// 示例：
//
//	pool := component.GetCoroutinePoolInstance()
//	pool.Init(4)
//	pool.Enqueue(func() { /* 任务 */ })
func GetCoroutinePoolInstance() *CoroutinePool {
	threadPoolOnce.Do(func() {
		threadPoolInstance = &CoroutinePool{}
	})
	return threadPoolInstance
}

// NewCoroutinePool 使用给定配置创建并启动一个新的协程池。
//
// 创建后池立即可以接受任务。
// 工作协程根据 InitialWorkers 配置自动启动。
//
// 参数：
//   - config: 池配置。零值将被替换为合理的默认值。
//
// 返回：
//   - *CoroutinePool: 一个正在运行的池实例，准备好接受任务。
//
// 示例：
//
//	config := component.DefaultCoroutinePoolConfig()
//	config.InitialWorkers = 8
//	pool := component.NewCoroutinePool(config)
//	defer pool.Stop(true)
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

// defaultPanicHandler 记录任务执行时的panic信息。
func defaultPanicHandler(v interface{}) {
	common.LogError("Task panic recovered", "error", fmt.Sprintf("%v", v))
}

// Init 使用默认配置初始化池，指定工作协程数量。
//
// 此方法用于从 GetCoroutinePoolInstance() 获取的全局单例。
// 对于新池，推荐使用 NewCoroutinePool()。
//
// 参数：
//   - threads: 工作协程数量。如果为0，使用CPU核心数。
func (cp *CoroutinePool) Init(threads int) {
	config := DefaultCoroutinePoolConfig()
	if threads > 0 {
		config.InitialWorkers = threads
	}
	cp.InitWithConfig(config)
}

// InitWithConfig 使用自定义配置初始化池。
//
// 此方法只能在空闲池上调用一次。
// 后续调用将被忽略并输出警告日志。
//
// 参数：
//   - config: 完整的池配置。
func (cp *CoroutinePool) InitWithConfig(config CoroutinePoolConfig) {
	if cp.state.Load() != int32(StateIdle) {
		common.LogWarn("CoroutinePool already initialized or running")
		return
	}

	cp.config = config
	cp.highQueue = make([]TaskWrapper, 0, config.InitialQueue/3)
	cp.mediumQueue = make([]TaskWrapper, 0, config.InitialQueue/3)
	cp.lowQueue = make([]TaskWrapper, 0, config.InitialQueue/3)
	cp.queueCond = sync.NewCond(&cp.queueMu)
	cp.stopCh = make(chan struct{})
	cp.workers = 0

	cp.startWorkers(config.InitialWorkers)
	cp.state.Store(int32(StateRunning))

	if config.Scaling.EnableAutoScale {
		cp.startScalingLoop()
	}

	common.LogInfo("CoroutinePool initialized", "workers", config.InitialWorkers, "queue", config.InitialQueue, "priority_queues", "High/Medium/Low")
}

// startWorkers 启动指定数量的工作协程。
func (cp *CoroutinePool) startWorkers(count int) {
	for i := 0; i < count; i++ {
		cp.wg.Add(1)
		atomic.AddInt32(&cp.workers, 1)
		go cp.workerLoop()
	}
}

// workerLoop 是每个工作协程的主循环。
//
// 它持续等待任务并按优先级顺序执行：High -> Medium -> Low。
// 当池停止时循环退出。
//
// 工作协程使用条件变量进行高效等待，避免忙等待。
func (cp *CoroutinePool) workerLoop() {
	defer cp.wg.Done()
	defer atomic.AddInt32(&cp.workers, -1)

	for {
		select {
		case <-cp.stopCh:
			return
		default:
		}

		cp.queueMu.Lock()
		for cp.highQueue == nil && cp.mediumQueue == nil && cp.lowQueue == nil {
			cp.queueMu.Unlock()
			return
		}

		for len(cp.highQueue) == 0 && len(cp.mediumQueue) == 0 && len(cp.lowQueue) == 0 {
			select {
			case <-cp.stopCh:
				cp.queueMu.Unlock()
				return
			default:
			}

			atomic.AddInt32(&cp.idleWorkers, 1)
			cp.queueCond.Wait()
			atomic.AddInt32(&cp.idleWorkers, -1)

			select {
			case <-cp.stopCh:
				cp.queueMu.Unlock()
				return
			default:
			}
		}

		var task TaskWrapper
		if len(cp.highQueue) > 0 {
			task = cp.highQueue[0]
			cp.highQueue = cp.highQueue[1:]
		} else if len(cp.mediumQueue) > 0 {
			task = cp.mediumQueue[0]
			cp.mediumQueue = cp.mediumQueue[1:]
		} else if len(cp.lowQueue) > 0 {
			task = cp.lowQueue[0]
			cp.lowQueue = cp.lowQueue[1:]
		}
		cp.queueMu.Unlock()

		cp.queueSize.Add(-1)
		cp.executeTask(&task)
	}
}

// executeTask 执行单个任务，包含panic恢复和指标统计。
func (cp *CoroutinePool) executeTask(task *TaskWrapper) {
	atomic.AddInt32(&cp.activeTasks, 1)
	cp.updatePeakMetrics()

	defer func() {
		atomic.AddInt32(&cp.activeTasks, -1)
		cp.recordTaskCompletion(task)
	}()

	defer func() {
		if r := recover(); r != nil {
			cp.recordTaskPanic()
			if cp.onTaskPanic != nil {
				cp.onTaskPanic(r)
			}
		}
	}()

	task.fn()
}

// Enqueue 提交默认（Medium）优先级的任务。
//
// 参数：
//   - f: 要执行的任务函数。
//
// 返回：
//   - bool: 成功入队返回true，池停止或队列满返回false。
func (cp *CoroutinePool) Enqueue(f func()) bool {
	return cp.EnqueueWithPriority(f, PriorityMedium)
}

// EnqueueWithPriority 提交指定优先级的任务。
//
// 任务被放入相应的优先级队列，将按严格优先级顺序执行：High -> Medium -> Low。
//
// 参数：
//   - f: 要执行的任务函数。
//   - priority: 优先级级别（PriorityHigh、PriorityMedium 或 PriorityLow）。
//
// 返回：
//   - bool: 成功入队返回true，池停止或队列满返回false。
//
// 示例：
//
//	pool.EnqueueWithPriority(func() {
//	    handleUrgentRequest()
//	}, component.PriorityHigh)
func (cp *CoroutinePool) EnqueueWithPriority(f func(), priority TaskPriority) bool {
	if cp.state.Load() != int32(StateRunning) {
		cp.recordTaskDropped()
		return false
	}

	task := TaskWrapper{
		fn:       f,
		priority: priority,
		enqTime:  time.Now(),
	}

	cp.queueMu.Lock()
	totalSize := len(cp.highQueue) + len(cp.mediumQueue) + len(cp.lowQueue)
	if totalSize >= cp.config.InitialQueue {
		cp.queueMu.Unlock()
		cp.recordQueueFull()
		return false
	}

	switch priority {
	case PriorityHigh:
		cp.highQueue = append(cp.highQueue, task)
	case PriorityMedium:
		cp.mediumQueue = append(cp.mediumQueue, task)
	case PriorityLow:
		cp.lowQueue = append(cp.lowQueue, task)
	default:
		cp.mediumQueue = append(cp.mediumQueue, task)
	}
	cp.queueMu.Unlock()

	cp.queueCond.Signal()
	cp.queueSize.Add(1)
	cp.recordTaskSubmitted()
	return true
}

// EnqueueWithTimeout 提交任务，带超时等待。
//
// 注意：当前实现中，此函数仅调用 Enqueue()，因为Go版本不支持等待队列空间。
// timeout 参数被忽略。此函数存在是为了与C++版本保持API兼容性。
//
// 参数：
//   - f: 要执行的任务函数。
//   - timeout: 最大等待时间（当前被忽略）。
//
// 返回：
//   - bool: 成功入队返回true，否则返回false。
func (cp *CoroutinePool) EnqueueWithTimeout(f func(), timeout time.Duration) bool {
	return cp.Enqueue(f)
}

// EnqueueOrExecute 提交任务，如果入队失败则内联执行。
//
// 这确保任务始终会运行，要么在池中执行，要么在调用者的协程中执行。
// 默认使用 Medium 优先级。
//
// 参数：
//   - f: 要执行的任务函数。
//
// 返回：
//   - bool: 成功入队返回true，内联执行返回false。
func (cp *CoroutinePool) EnqueueOrExecute(f func()) bool {
	return cp.EnqueueOrExecuteWithPriority(f, PriorityMedium)
}

// EnqueueOrExecuteWithPriority 提交指定优先级的任务，如果入队失败则内联执行。
//
// 参数：
//   - f: 要执行的任务函数。
//   - priority: 优先级级别。
//
// 返回：
//   - bool: 成功入队返回true，内联执行返回false。
func (cp *CoroutinePool) EnqueueOrExecuteWithPriority(f func(), priority TaskPriority) bool {
	if cp.state.Load() != int32(StateRunning) {
		f()
		return false
	}

	task := TaskWrapper{
		fn:       f,
		priority: priority,
		enqTime:  time.Now(),
	}

	cp.queueMu.Lock()
	totalSize := len(cp.highQueue) + len(cp.mediumQueue) + len(cp.lowQueue)
	if totalSize >= cp.config.InitialQueue {
		cp.queueMu.Unlock()
		cp.executeTaskInline(f)
		return false
	}

	switch priority {
	case PriorityHigh:
		cp.highQueue = append(cp.highQueue, task)
	case PriorityMedium:
		cp.mediumQueue = append(cp.mediumQueue, task)
	case PriorityLow:
		cp.lowQueue = append(cp.lowQueue, task)
	default:
		cp.mediumQueue = append(cp.mediumQueue, task)
	}
	cp.queueMu.Unlock()

	cp.queueCond.Signal()
	cp.queueSize.Add(1)
	cp.recordTaskSubmitted()
	return true
}

// executeTaskInline 在当前协程中执行任务，包含panic恢复。
func (cp *CoroutinePool) executeTaskInline(f func()) {
	defer func() {
		if r := recover(); r != nil {
			cp.recordTaskPanic()
			if cp.onTaskPanic != nil {
				cp.onTaskPanic(r)
			}
		}
	}()
	cp.recordTaskSubmitted()
	f()
	cp.metricsLock.Lock()
	cp.metrics.TotalTasksCompleted++
	cp.metricsLock.Unlock()
}

// startScalingLoop 启动自动扩缩容的后台协程。
func (cp *CoroutinePool) startScalingLoop() {
	cp.scaleTimer = time.NewTimer(cp.config.Scaling.ScaleInterval)
	go func() {
		for {
			select {
			case <-cp.stopCh:
				return
			case <-cp.scaleTimer.C:
				cp.checkAndScale()
				cp.scaleTimer.Reset(cp.config.Scaling.ScaleInterval)
			}
		}
	}()
}

// checkAndScale 评估当前负载并在需要时执行扩缩容。
func (cp *CoroutinePool) checkAndScale() {
	if !cp.config.Scaling.EnableAutoScale {
		return
	}

	currentWorkers := int(atomic.LoadInt32(&cp.workers))
	currentActive := int(atomic.LoadInt32(&cp.activeTasks))
	currentQueueSize := int(cp.queueSize.Load())

	loadFactor := float64(0)
	if currentWorkers > 0 {
		loadFactor = float64(currentActive) / float64(currentWorkers)
	}

	queueLoadFactor := float64(currentQueueSize) / float64(cp.config.InitialQueue)

	now := time.Now()

	if loadFactor >= cp.config.Scaling.ScaleThreshold || queueLoadFactor >= cp.config.Scaling.ScaleThreshold {
		if currentWorkers < cp.config.Scaling.MaxWorkers &&
			now.Sub(cp.lastScaleUp) >= cp.config.Scaling.ScaleUpCooldown {
			newWorkers := min(currentWorkers+cp.config.Scaling.ScaleUpStep, cp.config.Scaling.MaxWorkers)
			toAdd := newWorkers - currentWorkers
			if toAdd > 0 {
				cp.startWorkers(toAdd)
				cp.lastScaleUp = now
				cp.recordScaleUp()
				common.LogInfo("CoroutinePool scaling up", "from", currentWorkers, "to", newWorkers)
			}
		}
	} else if loadFactor <= cp.config.Scaling.ShrinkThreshold && queueLoadFactor <= cp.config.Scaling.ShrinkThreshold {
		if currentWorkers > cp.config.Scaling.MinWorkers &&
			now.Sub(cp.lastScaleDown) >= cp.config.Scaling.ScaleDownCooldown {
			common.LogInfo("CoroutinePool considering scale down", "workers", currentWorkers, "load", loadFactor)
		}
	}
}

// Stop 优雅地关闭协程池。
//
// 参数：
//   - waitForTasks: 如果为true，在停止前等待所有队列中的任务完成。
//     如果为false，立即停止并丢弃队列中的任务。
//
// 示例：
//
//	pool.Stop(true)  // 等待所有任务完成
//	pool.Stop(false) // 立即停止
func (cp *CoroutinePool) Stop(waitForTasks bool) {
	if !cp.state.CompareAndSwap(int32(StateRunning), int32(StateStopping)) {
		return
	}

	common.LogInfo("CoroutinePool stopping", "wait_for_tasks", waitForTasks)

	if cp.scaleTimer != nil {
		cp.scaleTimer.Stop()
	}

	close(cp.stopCh)

	cp.queueMu.Lock()
	cp.queueCond.Broadcast()
	cp.queueMu.Unlock()

	if waitForTasks {
		for cp.queueSize.Load() > 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	cp.wg.Wait()

	cp.queueMu.Lock()
	cp.highQueue = nil
	cp.mediumQueue = nil
	cp.lowQueue = nil
	cp.queueMu.Unlock()

	cp.state.Store(int32(StateStopped))

	common.LogInfo("CoroutinePool stopped", "metrics", cp.GetMetrics())
}

// GetMetrics 返回协程池运行时指标的快照。
//
// 返回的指标包含各优先级队列大小和累计统计数据。
func (cp *CoroutinePool) GetMetrics() PoolMetrics {
	cp.metricsLock.RLock()
	defer cp.metricsLock.RUnlock()

	m := cp.metrics
	m.TotalTasksSubmitted = cp.atomicMetrics.totalTasksSubmitted.Load()
	m.TotalTasksCompleted = cp.atomicMetrics.totalTasksCompleted.Load()
	m.TotalTasksDropped = cp.atomicMetrics.totalTasksDropped.Load()
	m.TotalTasksTimeout = cp.atomicMetrics.totalTasksTimeout.Load()
	m.QueueFullRejectCount = cp.atomicMetrics.queueFullRejectCount.Load()
	m.ScaleUpCount = cp.atomicMetrics.scaleUpCount.Load()
	m.ScaleDownCount = cp.atomicMetrics.scaleDownCount.Load()
	m.CurrentQueueSize = cp.queueSize.Load()
	m.CurrentActiveWorkers = atomic.LoadInt32(&cp.activeTasks)
	m.CurrentIdleWorkers = atomic.LoadInt32(&cp.idleWorkers)

	cp.queueMu.Lock()
	m.HighQueueSize = int32(len(cp.highQueue))
	m.MediumQueueSize = int32(len(cp.mediumQueue))
	m.LowQueueSize = int32(len(cp.lowQueue))
	cp.queueMu.Unlock()

	return m
}

// GetQueueSize 返回所有队列中的任务总数。
func (cp *CoroutinePool) GetQueueSize() int {
	return int(cp.queueSize.Load())
}

// GetQueueSizeByPriority 返回指定优先级队列中的任务数量。
//
// 参数：
//   - priority: 要查询的优先级级别。
//
// 返回：
//   - int: 指定队列中的任务数量。
func (cp *CoroutinePool) GetQueueSizeByPriority(priority TaskPriority) int {
	cp.queueMu.Lock()
	defer cp.queueMu.Unlock()

	switch priority {
	case PriorityHigh:
		return len(cp.highQueue)
	case PriorityMedium:
		return len(cp.mediumQueue)
	case PriorityLow:
		return len(cp.lowQueue)
	default:
		return len(cp.mediumQueue)
	}
}

// GetWorkerCount 返回当前工作协程数量。
func (cp *CoroutinePool) GetWorkerCount() int {
	return int(atomic.LoadInt32(&cp.workers))
}

// GetActiveTaskCount 返回当前正在执行的任务数量。
func (cp *CoroutinePool) GetActiveTaskCount() int {
	return int(atomic.LoadInt32(&cp.activeTasks))
}

// IsRunning 返回协程池是否正在运行。
func (cp *CoroutinePool) IsRunning() bool {
	return cp.state.Load() == int32(StateRunning)
}

// SetPanicHandler 设置任务panic的自定义处理器。
//
// 当任务发生panic时，处理器会被调用并传入panic值。
// 如果未设置，将使用默认处理器记录panic信息。
func (cp *CoroutinePool) SetPanicHandler(handler func(interface{})) {
	cp.onTaskPanic = handler
}

// GetTaskPriorityByName 将字符串名称转换为 TaskPriority 值。
//
// 支持的名称（区分大小写）："high"、"High"、"HIGH"、"low"、"Low"、"LOW"。
// 其他任何名称返回 PriorityMedium。
//
// 参数：
//   - name: 优先级名称字符串。
//
// 返回：
//   - TaskPriority: 对应的优先级级别。
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

// recordTaskSubmitted 增加已提交任务计数器。
func (cp *CoroutinePool) recordTaskSubmitted() {
	cp.atomicMetrics.totalTasksSubmitted.Add(1)
}

// recordTaskCompletion 更新完成计数器和平均延迟。
func (cp *CoroutinePool) recordTaskCompletion(task *TaskWrapper) {
	completed := cp.atomicMetrics.totalTasksCompleted.Add(1)
	latency := time.Since(task.enqTime).Nanoseconds()
	cp.metricsLock.Lock()
	avg := cp.metrics.AverageTaskLatencyNs
	cp.metrics.AverageTaskLatencyNs = (avg*(completed-1) + uint64(latency)) / completed
	cp.metricsLock.Unlock()
}

// recordTaskDropped 增加已丢弃任务计数器。
func (cp *CoroutinePool) recordTaskDropped() {
	cp.atomicMetrics.totalTasksDropped.Add(1)
}

// recordTaskTimeout 增加超时计数器。
func (cp *CoroutinePool) recordTaskTimeout() {
	cp.atomicMetrics.totalTasksTimeout.Add(1)
}

// recordQueueFull 增加队列满拒绝计数器。
func (cp *CoroutinePool) recordQueueFull() {
	cp.atomicMetrics.queueFullRejectCount.Add(1)
}

// recordTaskPanic 当任务panic时增加丢弃计数器。
func (cp *CoroutinePool) recordTaskPanic() {
	cp.atomicMetrics.totalTasksDropped.Add(1)
}

// recordScaleUp 增加扩容计数器。
func (cp *CoroutinePool) recordScaleUp() {
	cp.atomicMetrics.scaleUpCount.Add(1)
}

// recordScaleDown 增加缩容计数器。
func (cp *CoroutinePool) recordScaleDown() {
	cp.atomicMetrics.scaleDownCount.Add(1)
}

// updatePeakMetrics 更新峰值指标跟踪。
func (cp *CoroutinePool) updatePeakMetrics() {
	cp.metricsLock.Lock()
	active := atomic.LoadInt32(&cp.activeTasks)
	if active > cp.metrics.PeakActiveWorkers {
		cp.metrics.PeakActiveWorkers = active
	}
	queueSize := cp.queueSize.Load()
	if queueSize > cp.metrics.PeakQueueSize {
		cp.metrics.PeakQueueSize = queueSize
	}
	cp.metricsLock.Unlock()
}
