// Package component 提供tyke框架的核心可复用组件。
//
// 本文件实现了一个分层时间轮，用于高效的定时器管理。
// 时间轮支持一次性定时器和重复定时器，添加/删除操作的复杂度为O(1)。
//
// # 主要特性
//
// - 多级分层设计，支持宽时间范围
// - O(1)的添加和删除操作
// - 支持一次性定时器和重复定时器
// - 基于回调的到期处理
// - 线程安全操作
//
// # 架构
//
// 时间轮使用多个不同精度的层级：
//   - 第0层: 200ms刻度, 50个槽位 (覆盖0-10秒)
//   - 第1层: 1s刻度, 60个槽位 (覆盖10-70秒)
//   - 第2层: 10s刻度, 6个槽位 (覆盖70-130秒)
//   - 第3层: 60s刻度, 60个槽位 (覆盖130秒以上)
//
// # 使用示例
//
//	tw := component.GetTimingWheel()
//	tw.Init()
//	defer tw.Stop()
//
//	// 添加一次性定时器
//	id := tw.AddTaskAt(time.Now().Add(5*time.Second), func() {
//	    fmt.Println("Timer fired!")
//	})
//
//	// 添加重复定时器
//	tw.AddRepeatedTask(1000, 500, func() {
//	    fmt.Println("Repeating!")
//	})
//
// # 作者
//
// Nick
package component

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cppbugboy/tyke-go/tyke/common"
)

// TaskType 区分不同类型的定时任务。
type TaskType int

const (
	// TaskTypeFunc 表示函数回调任务。
	TaskTypeFunc TaskType = iota
	// TaskTypeFuture 表示Future/Promise任务。
	TaskTypeFuture
)

// TimerId 是定时器任务的唯一标识符。
type TimerId uint64

// InvalidTimerId 表示无效或未设置的定时器ID。
const InvalidTimerId TimerId = 0

// TimerTask 表示一个已调度的定时器，可选支持重复行为。
type TimerTask struct {
	Id          TimerId   // 唯一标识符
	Callback    func()    // 到期时调用的函数
	ExpireTime  time.Time // 定时器应该触发的时间
	IntervalMs  uint32    // 重复间隔（0表示一次性）
	IsRepeating bool      // 是否为重复定时器
	Cancelled   bool      // 定时器是否已取消
}

// TaskEntry 表示时间轮槽位中的任务。
type TaskEntry struct {
	Uuid       string    // 任务的唯一标识符
	ExpireTime time.Time // 任务到期时间
	TimeoutMs  uint32    // 原始超时时间（毫秒）
	Type       TaskType  // 任务类型
}

// TimingWheelLevel 表示分层时间轮中的单个层级。
type TimingWheelLevel struct {
	tickIntervalMs uint32        // 每刻度的毫秒数
	slotCount      uint32        // 此层级的槽位数量
	currentSlot    uint32        // 当前槽位位置
	slots          [][]TaskEntry // 每个槽位的任务条目
}

// TimingWheelLevelConfig 配置时间轮的单个层级。
type TimingWheelLevelConfig struct {
	TickIntervalMs uint32 // 每刻度的毫秒数
	SlotCount      uint32 // 槽位数量
}

// TimingWheelConfig 包含所有时间轮层级的配置。
type TimingWheelConfig struct {
	Levels []TimingWheelLevelConfig
}

// TimingWheel 是一个具有多种精度的分层定时器管理器。
//
// 它提供高效的O(1)添加和删除定时器操作，
// 适用于高吞吐量的定时器管理场景。
type TimingWheel struct {
	levels       []TimingWheelLevel // 分层时间轮层级
	stopCh       chan struct{}      // 用于停止刻度循环的通道
	wg           sync.WaitGroup     // 用于优雅关闭的等待组
	mu           sync.Mutex         // 保护所有可变状态
	taskLocation map[string]string  // 将任务UUID映射到位置(level:slot)
	stopped      atomic.Bool        // 时间轮是否已停止
	initialized  atomic.Bool        // 时间轮是否已初始化

	onExpiredFunc   func(uuid string) // 函数任务到期回调
	onExpiredFuture func(uuid string) // Future任务到期回调

	timerTasks  map[TimerId]*TimerTask // 按ID索引的定时器任务
	nextTimerId atomic.Uint64          // 下一个定时器ID
}

var (
	timingWheelInstance *TimingWheel
	timingWheelOnce     sync.Once
)

// GetTimingWheel 返回全局单例TimingWheel实例。
//
// 单例在首次调用时延迟初始化。
// 使用时间轮前请调用Init()。
//
// 返回：
//   - *TimingWheel: 全局时间轮实例。
func GetTimingWheel() *TimingWheel {
	timingWheelOnce.Do(func() {
		timingWheelInstance = &TimingWheel{
			taskLocation: make(map[string]string),
			timerTasks:   make(map[TimerId]*TimerTask),
			stopCh:       make(chan struct{}),
		}
		common.LogInfo("TimingWheel instance created")
	})
	return timingWheelInstance
}

// DefaultTimingWheelConfig 返回具有合理默认值的配置。
//
// 默认配置使用4个层级：
//   - 第0层: 200ms刻度, 50个槽位 (覆盖0-10秒)
//   - 第1层: 1s刻度, 60个槽位 (覆盖10-70秒)
//   - 第2层: 10s刻度, 6个槽位 (覆盖70-130秒)
//   - 第3层: 60s刻度, 60个槽位 (覆盖130秒以上)
func DefaultTimingWheelConfig() TimingWheelConfig {
	return TimingWheelConfig{
		Levels: []TimingWheelLevelConfig{
			{TickIntervalMs: 200, SlotCount: 50},
			{TickIntervalMs: 1000, SlotCount: 60},
			{TickIntervalMs: 10000, SlotCount: 6},
			{TickIntervalMs: 60000, SlotCount: 60},
		},
	}
}

// SetExpiredCallbacks 设置到期任务的回调函数。
//
// 这些回调在时间轮中的任务到期时调用。
// 使用此函数实现自定义到期处理。
//
// 参数：
//   - onFunc: TaskTypeFunc任务到期时的回调。
//   - onFuture: TaskTypeFuture任务到期时的回调。
func (tw *TimingWheel) SetExpiredCallbacks(onFunc func(string), onFuture func(string)) {
	tw.onExpiredFunc = onFunc
	tw.onExpiredFuture = onFuture
}

// Init 使用可选的自定义配置初始化时间轮。
//
// 如果未提供配置，则使用DefaultTimingWheelConfig()。
// 此方法只能调用一次；后续调用将被忽略。
//
// 参数：
//   - config: 可选的自定义配置。
func (tw *TimingWheel) Init(config ...TimingWheelConfig) {
	if tw.initialized.Load() {
		common.LogWarn("TimingWheel already initialized")
		return
	}

	cfg := DefaultTimingWheelConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	tw.mu.Lock()
	defer tw.mu.Unlock()

	tw.levels = nil
	for _, lc := range cfg.Levels {
		level := TimingWheelLevel{
			tickIntervalMs: lc.TickIntervalMs,
			slotCount:      lc.SlotCount,
			currentSlot:    0,
			slots:          make([][]TaskEntry, lc.SlotCount),
		}
		for i := range level.slots {
			level.slots[i] = make([]TaskEntry, 0)
		}
		tw.levels = append(tw.levels, level)
	}

	tw.taskLocation = make(map[string]string)
	tw.stopCh = make(chan struct{})
	tw.stopped.Store(false)
	tw.initialized.Store(true)

	tw.wg.Add(1)
	go tw.tickLoop()

	common.LogInfo("TimingWheel initialized", "levels", len(tw.levels))
}

// AddTask 将任务添加到具有超时时间的时间轮中。
//
// 任务根据超时时间放置在适当的层级和槽位中。
// 当任务到期时，调用已注册的回调。
//
// 参数：
//   - uuid: 任务的唯一标识符。
//   - timeoutMs: 超时时间（毫秒）。
//   - taskType: 任务类型（TaskTypeFunc或TaskTypeFuture）。
func (tw *TimingWheel) AddTask(uuid string, timeoutMs uint32, taskType TaskType) {
	if tw.stopped.Load() || len(tw.levels) == 0 {
		common.LogWarn("TimingWheel not running, cannot add task", "uuid", uuid)
		return
	}

	tw.mu.Lock()
	defer tw.mu.Unlock()

	levelIndex := tw.selectLevel(timeoutMs)
	slot := tw.calculateSlot(levelIndex, timeoutMs)

	entry := TaskEntry{
		Uuid:       uuid,
		ExpireTime: time.Now().Add(time.Duration(timeoutMs) * time.Millisecond),
		TimeoutMs:  timeoutMs,
		Type:       taskType,
	}

	tw.levels[levelIndex].slots[slot] = append(tw.levels[levelIndex].slots[slot], entry)
	tw.taskLocation[uuid] = taskLocationKey(levelIndex, slot)

	common.LogDebug("Task added to timing wheel", "uuid", uuid, "timeout", timeoutMs, "level", levelIndex, "slot", slot)
}

// RemoveTask 通过UUID从时间轮中移除任务。
//
// 参数：
//   - uuid: 要移除的任务的唯一标识符。
func (tw *TimingWheel) RemoveTask(uuid string) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	delete(tw.taskLocation, uuid)
	common.LogDebug("Task removed from timing wheel", "uuid", uuid)
}

// AddTaskAt 调度一个回调在特定截止时间执行。
//
// 如果截止时间已过，回调会立即在新的goroutine中执行。
//
// 参数：
//   - deadline: 执行回调的时间。
//   - cb: 要执行的回调函数。
//
// 返回：
//   - TimerId: 用于取消的ID，如果已过期则返回InvalidTimerId。
//
// 示例：
//
//	id := tw.AddTaskAt(time.Now().Add(5*time.Second), func() {
//	    fmt.Println("Timer fired!")
//	})
func (tw *TimingWheel) AddTaskAt(deadline time.Time, cb func()) TimerId {
	if tw.stopped.Load() {
		common.LogWarn("TimingWheel not running, cannot add task at")
		return InvalidTimerId
	}

	delay := time.Until(deadline)
	if delay <= 0 {
		go cb()
		return InvalidTimerId
	}

	id := TimerId(tw.nextTimerId.Add(1))
	task := &TimerTask{
		Id:         id,
		Callback:   cb,
		ExpireTime: deadline,
	}

	tw.mu.Lock()
	tw.timerTasks[id] = task
	tw.mu.Unlock()

	go func() {
		select {
		case <-time.After(delay):
			tw.mu.Lock()
			t, exists := tw.timerTasks[id]
			if exists && !t.Cancelled {
				delete(tw.timerTasks, id)
				tw.mu.Unlock()
				cb()
			} else {
				tw.mu.Unlock()
			}
		case <-tw.stopCh:
			return
		}
	}()

	common.LogDebug("AddTaskAt registered", "timer_id", id, "delay_ms", delay.Milliseconds())
	return id
}

// CancelTask 取消先前调度的定时器。
//
// 参数：
//   - id: 从AddTaskAt或AddRepeatedTask返回的定时器ID。
//
// 返回：
//   - bool: 如果找到并取消定时器返回true，否则返回false。
func (tw *TimingWheel) CancelTask(id TimerId) bool {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if task, exists := tw.timerTasks[id]; exists {
		task.Cancelled = true
		delete(tw.timerTasks, id)
		common.LogDebug("Timer task cancelled", "timer_id", id)
		return true
	}
	return false
}

// AddRepeatedTask 调度一个回调以固定间隔重复执行。
//
// 回调首先在initialDelayMs后执行，然后每隔intervalMs执行一次。
// 定时器持续运行，直到通过CancelTask取消或时间轮停止。
//
// 参数：
//   - initialDelayMs: 首次执行前的延迟（毫秒）。
//   - intervalMs: 执行间隔（毫秒）。
//   - cb: 要执行的回调函数。
//
// 返回：
//   - TimerId: 用于取消的ID。
//
// 示例：
//
//	id := tw.AddRepeatedTask(1000, 5000, func() {
//	    fmt.Println("Every 5 seconds after 1 second delay")
//	})
//	defer tw.CancelTask(id)
func (tw *TimingWheel) AddRepeatedTask(initialDelayMs uint32, intervalMs uint32, cb func()) TimerId {
	if tw.stopped.Load() {
		common.LogWarn("TimingWheel not running, cannot add repeated task")
		return InvalidTimerId
	}

	id := TimerId(tw.nextTimerId.Add(1))
	task := &TimerTask{
		Id:          id,
		Callback:    cb,
		IntervalMs:  intervalMs,
		IsRepeating: true,
	}

	tw.mu.Lock()
	tw.timerTasks[id] = task
	tw.mu.Unlock()

	go func() {
		if initialDelayMs > 0 {
			select {
			case <-time.After(time.Duration(initialDelayMs) * time.Millisecond):
			case <-tw.stopCh:
				return
			}
		}

		for {
			tw.mu.Lock()
			t, exists := tw.timerTasks[id]
			if !exists || t.Cancelled {
				tw.mu.Unlock()
				return
			}
			tw.mu.Unlock()

			cb()

			select {
			case <-time.After(time.Duration(intervalMs) * time.Millisecond):
			case <-tw.stopCh:
				return
			}
		}
	}()

	common.LogDebug("AddRepeatedTask registered", "timer_id", id, "initial_delay", initialDelayMs, "interval", intervalMs)
	return id
}

// Stop 优雅地关闭时间轮。
//
// 所有定时器被取消，刻度循环停止。
// 时间轮可以在停止后重新初始化。
func (tw *TimingWheel) Stop() {
	if !tw.initialized.Load() || tw.stopped.Swap(true) {
		return
	}

	close(tw.stopCh)
	tw.wg.Wait()

	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.taskLocation = make(map[string]string)
	tw.timerTasks = make(map[TimerId]*TimerTask)
	tw.levels = nil
	tw.initialized.Store(false)

	common.LogInfo("TimingWheel stopped")
}

// selectLevel 为给定的超时时间选择适当的时间轮层级。
//
// 较短的超时进入较低（更精确）的层级。
// 较长的超时进入较高（较不精确）的层级。
func (tw *TimingWheel) selectLevel(timeoutMs uint32) int {
	maxRangeMs := uint32(0)
	for i, level := range tw.levels {
		maxRangeMs += level.tickIntervalMs * level.slotCount
		if timeoutMs <= maxRangeMs {
			return i
		}
	}
	return len(tw.levels) - 1
}

// calculateSlot 确定在层级内的哪个槽位放置任务。
func (tw *TimingWheel) calculateSlot(levelIndex int, timeoutMs uint32) uint32 {
	level := &tw.levels[levelIndex]
	ticksAhead := (timeoutMs + level.tickIntervalMs - 1) / level.tickIntervalMs
	targetSlot := (level.currentSlot + ticksAhead) % level.slotCount
	return targetSlot
}

// tickLoop 是推进时间轮的主循环。
func (tw *TimingWheel) tickLoop() {
	defer tw.wg.Done()

	tickInterval := time.Duration(tw.levels[0].tickIntervalMs) * time.Millisecond
	if tickInterval <= 0 {
		tickInterval = 200 * time.Millisecond
	}
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	common.LogInfo("TimingWheel tick loop started")

	for {
		select {
		case <-tw.stopCh:
			return
		case <-ticker.C:
			if tw.stopped.Load() {
				return
			}
			tw.tick()
		}
	}
}

// tick 将时间轮推进一个刻度并处理到期任务。
func (tw *TimingWheel) tick() {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	for i := range tw.levels {
		level := &tw.levels[i]
		level.currentSlot = (level.currentSlot + 1) % level.slotCount

		tasks := level.slots[level.currentSlot]
		level.slots[level.currentSlot] = make([]TaskEntry, 0)

		if len(tasks) > 0 {
			tw.processExpiredTasks(tasks)
		}

		if level.currentSlot != 0 {
			break
		}
	}
}

// processExpiredTasks 处理已到达其槽位的任务。
//
// 尚未到达精确到期时间的任务会被重新调度到更精确的层级。
func (tw *TimingWheel) processExpiredTasks(tasks []TaskEntry) {
	now := time.Now()

	for _, entry := range tasks {
		if _, exists := tw.taskLocation[entry.Uuid]; !exists {
			continue
		}

		if entry.ExpireTime.After(now) {
			remaining := uint32(entry.ExpireTime.Sub(now).Milliseconds())
			if remaining == 0 {
				remaining = 1
			}
			levelIndex := tw.selectLevel(remaining)
			slot := tw.calculateSlot(levelIndex, entry.TimeoutMs)
			tw.levels[levelIndex].slots[slot] = append(tw.levels[levelIndex].slots[slot], entry)
			tw.taskLocation[entry.Uuid] = taskLocationKey(levelIndex, slot)
			continue
		}

		delete(tw.taskLocation, entry.Uuid)

		if entry.Type == TaskTypeFuture {
			common.LogWarn("TimingWheel: future task expired", "uuid", entry.Uuid, "timeout", entry.TimeoutMs)
			if tw.onExpiredFuture != nil {
				tw.onExpiredFuture(entry.Uuid)
			}
		} else {
			common.LogWarn("TimingWheel: func task expired", "uuid", entry.Uuid, "timeout", entry.TimeoutMs)
			if tw.onExpiredFunc != nil {
				tw.onExpiredFunc(entry.Uuid)
			}
		}
	}
}

// taskLocationKey 创建用于任务位置查找的字符串键。
func taskLocationKey(levelIndex int, slot uint32) string {
	return fmt.Sprintf("%d:%d", levelIndex, slot)
}
