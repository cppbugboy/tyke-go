package component

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tyke/tyke/tyke/common"
)

type TaskType int

const (
	TaskTypeFunc TaskType = iota
	TaskTypeFuture
)

type TaskEntry struct {
	Uuid       string
	ExpireTime time.Time
	TimeoutMs  uint32
	Type       TaskType
}

type TimingWheelLevel struct {
	tickIntervalMs uint32
	slotCount      uint32
	currentSlot    uint32
	slots          [][]TaskEntry
}

type TimingWheelLevelConfig struct {
	TickIntervalMs uint32
	SlotCount      uint32
}

type TimingWheelConfig struct {
	Levels []TimingWheelLevelConfig
}

// TimingWheel 是多级时间轮，用于高效的定时任务管理。
type TimingWheel struct {
	levels       []TimingWheelLevel
	stopCh       chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
	taskLocation map[string]string
	stopped      atomic.Bool
	initialized  atomic.Bool

	onExpiredFunc   func(uuid string)
	onExpiredFuture func(uuid string)
}

var (
	timingWheelInstance *TimingWheel
	timingWheelOnce     sync.Once
)

func GetTimingWheel() *TimingWheel {
	timingWheelOnce.Do(func() {
		timingWheelInstance = &TimingWheel{
			taskLocation: make(map[string]string),
			stopCh:       make(chan struct{}),
		}
		common.LogInfo("TimingWheel instance created")
	})
	return timingWheelInstance
}

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

func (tw *TimingWheel) SetExpiredCallbacks(onFunc func(string), onFuture func(string)) {
	tw.onExpiredFunc = onFunc
	tw.onExpiredFuture = onFuture
}

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

func (tw *TimingWheel) RemoveTask(uuid string) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	delete(tw.taskLocation, uuid)
	common.LogDebug("Task removed from timing wheel", "uuid", uuid)
}

func (tw *TimingWheel) Stop() {
	if !tw.initialized.Load() || tw.stopped.Swap(true) {
		return
	}

	close(tw.stopCh)
	tw.wg.Wait()

	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.taskLocation = make(map[string]string)
	tw.levels = nil
	tw.initialized.Store(false)

	common.LogInfo("TimingWheel stopped")
}

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

func (tw *TimingWheel) calculateSlot(levelIndex int, timeoutMs uint32) uint32 {
	level := &tw.levels[levelIndex]
	ticksAhead := (timeoutMs + level.tickIntervalMs - 1) / level.tickIntervalMs
	targetSlot := (level.currentSlot + ticksAhead) % level.slotCount
	return targetSlot
}

func (tw *TimingWheel) tickLoop() {
	defer tw.wg.Done()

	ticker := time.NewTicker(200 * time.Millisecond)
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

func taskLocationKey(levelIndex int, slot uint32) string {
	return fmt.Sprintf("%d:%d", levelIndex, slot)
}
