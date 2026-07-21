// Package core 实现 Tyke 框架内核。
//
// 本文件管理请求存根条目：以消息 UUID 为键的待处理异步回调 (funcEntry) 和
// Future (futureEntry)。提供了数据处理器和时间轮使用的添加/设置/执行/清理操作。
package core

import (
	"sync"
	"time"

	"tyke-go/common"
	"tyke-go/component"
)

// futureEntry 跟踪一个等待中的 Future 响应通道。
type futureEntry struct {
	ch        chan *Response
	createdAt time.Time
	timeoutMs uint32
}

// funcEntry 跟踪一个等待中的异步回调函数。
type funcEntry struct {
	fn        func(*Response)
	createdAt time.Time
	timeoutMs uint32
}

var (
	uuidFutureMap   = make(map[string]futureEntry)
	uuidFutureMapMu sync.Mutex
	uuidFuncMap     = make(map[string]funcEntry)
	uuidFuncMapMu   sync.Mutex
)

// RequestStubAddFuture 为指定的消息 UUID 注册一个 Future 通道。
// 添加一个时间轮任务以便在条目过期时进行清理。
func RequestStubAddFuture(uuid string, ch chan *Response, timeoutMs uint32) {
	uuidFutureMapMu.Lock()
	uuidFutureMap[uuid] = futureEntry{ch: ch, createdAt: time.Now(), timeoutMs: timeoutMs}
	uuidFutureMapMu.Unlock()
	component.GetTimingWheel().AddTask(uuid, timeoutMs, component.TaskTypeFuture)
	common.LogDebug("Future entry added", "uuid", uuid, "timeout", timeoutMs)
}

// RequestStubSetFuture 通过在其通道上发送响应来解析等待中的 Future。
// 解析后移除时间轮任务。
func RequestStubSetFuture(response *Response) {
	var extractedCh chan *Response
	found := false

	uuidFutureMapMu.Lock()
	if entry, ok := uuidFutureMap[response.GetMsgUUID()]; ok {
		extractedCh = entry.ch
		delete(uuidFutureMap, response.GetMsgUUID())
		found = true
		common.LogDebug("Future result set", "uuid", response.GetMsgUUID())
	} else {
		common.LogWarn("Future entry not found for response", "uuid", response.GetMsgUUID())
	}
	uuidFutureMapMu.Unlock()

	if found {
		component.GetTimingWheel().RemoveTask(response.GetMsgUUID())
		select {
		case extractedCh <- response:
		default:
			common.LogWarn("Future channel full, dropping response", "uuid", response.GetMsgUUID())
			ReleaseResponse(response)
		}
	}
}

// RequestStubAddFunc 为指定的消息 UUID 注册一个回调函数。
// 添加一个时间轮任务以便在条目过期时进行清理。
func RequestStubAddFunc(uuid string, fn func(*Response), timeoutMs uint32) {
	uuidFuncMapMu.Lock()
	uuidFuncMap[uuid] = funcEntry{fn: fn, createdAt: time.Now(), timeoutMs: timeoutMs}
	uuidFuncMapMu.Unlock()
	component.GetTimingWheel().AddTask(uuid, timeoutMs, component.TaskTypeFunc)
	common.LogDebug("Callback entry added", "uuid", uuid, "timeout", timeoutMs)
}

// RequestStubExecFunc 为响应的消息 UUID 执行已注册的回调函数。
// 回调函数获得响应的所有权，并负责释放它。
func RequestStubExecFunc(response *Response) {
	var extractedFn func(*Response)
	found := false

	uuidFuncMapMu.Lock()
	if entry, ok := uuidFuncMap[response.GetMsgUUID()]; ok {
		extractedFn = entry.fn
		delete(uuidFuncMap, response.GetMsgUUID())
		found = true
	} else {
		common.LogWarn("Callback entry not found for response", "uuid", response.GetMsgUUID())
	}
	uuidFuncMapMu.Unlock()

	if found {
		component.GetTimingWheel().RemoveTask(response.GetMsgUUID())
		common.LogDebug("Executing callback for response", "uuid", response.GetMsgUUID())
		defer ReleaseResponse(response)
		extractedFn(response)
	}
}

// RequestStubCleanupExpiredFuture 通过在其通道上发送超时响应
// 来清理过期的 Future。
func RequestStubCleanupExpiredFuture(uuid string) {
	var extractedCh chan *Response
	found := false

	uuidFutureMapMu.Lock()
	if entry, ok := uuidFutureMap[uuid]; ok {
		extractedCh = entry.ch
		delete(uuidFutureMap, uuid)
		found = true
		common.LogWarn("Expired future cleaned up", "uuid", uuid)
	}
	uuidFutureMapMu.Unlock()

	if found {
		timeoutResp := AcquireResponse()
		timeoutResp.SetMsgUUID(uuid)
		timeoutResp.SetResult(int(common.StatusTimeout), "future timeout")
		select {
		case extractedCh <- timeoutResp:
		default:
			common.LogWarn("Future channel full on timeout, dropping", "uuid", uuid)
			ReleaseResponse(timeoutResp)
		}
	}
}

// RequestStubCleanupExpiredFunc 通过用超时响应调用回调函数
// 来清理过期的回调。
func RequestStubCleanupExpiredFunc(uuid string) {
	var extractedFn func(*Response)
	found := false

	uuidFuncMapMu.Lock()
	if entry, ok := uuidFuncMap[uuid]; ok {
		extractedFn = entry.fn
		delete(uuidFuncMap, uuid)
		found = true
		common.LogWarn("Expired func cleaned up, notifying with timeout", "uuid", uuid)
	}
	uuidFuncMapMu.Unlock()

	if found {
		timeoutResp := AcquireResponse()
		timeoutResp.SetMsgUUID(uuid)
		timeoutResp.SetResult(int(common.StatusTimeout), "func callback timeout")
		defer ReleaseResponse(timeoutResp)
		extractedFn(timeoutResp)
	}
}

// RequestStubCleanupExpiredFutures 扫描所有 Future 条目并清理过期的条目。
func RequestStubCleanupExpiredFutures() {
	now := time.Now()
	var expired []string

	uuidFutureMapMu.Lock()
	for uuid, entry := range uuidFutureMap {
		if now.Sub(entry.createdAt) >= time.Duration(entry.timeoutMs)*time.Millisecond {
			expired = append(expired, uuid)
		}
	}
	uuidFutureMapMu.Unlock()

	for _, uuid := range expired {
		RequestStubCleanupExpiredFuture(uuid)
	}
}

// RequestStubCleanupExpiredFuncs 扫描所有回调条目并清理过期的条目。
func RequestStubCleanupExpiredFuncs() {
	now := time.Now()
	var expired []string

	uuidFuncMapMu.Lock()
	for uuid, entry := range uuidFuncMap {
		if now.Sub(entry.createdAt) >= time.Duration(entry.timeoutMs)*time.Millisecond {
			expired = append(expired, uuid)
		}
	}
	uuidFuncMapMu.Unlock()

	for _, uuid := range expired {
		RequestStubCleanupExpiredFunc(uuid)
	}
}

// RequestStubExecFuncOrSetFuture 依次尝试在回调表和 Future 表中
// 匹配响应的 UUID。如果找到匹配则返回 true。
// 在响应路由器没有匹配路由时作为回退方案使用。
func RequestStubExecFuncOrSetFuture(response *Response) bool {
	uuid := response.GetMsgUUID()

	uuidFuncMapMu.Lock()
	if entry, ok := uuidFuncMap[uuid]; ok {
		extractedFn := entry.fn
		delete(uuidFuncMap, uuid)
		uuidFuncMapMu.Unlock()

		component.GetTimingWheel().RemoveTask(uuid)
		common.LogDebug("Executing fallback func for response", "uuid", uuid)
		extractedFn(response)
		return true
	}
	uuidFuncMapMu.Unlock()

	uuidFutureMapMu.Lock()
	if entry, ok := uuidFutureMap[uuid]; ok {
		extractedCh := entry.ch
		delete(uuidFutureMap, uuid)
		uuidFutureMapMu.Unlock()

		component.GetTimingWheel().RemoveTask(uuid)
		common.LogDebug("Setting fallback future for response", "uuid", uuid)
		select {
		case extractedCh <- response:
		default:
			common.LogWarn("Fallback future channel full, dropping response", "uuid", uuid)
			ReleaseResponse(response)
		}
		return true
	}
	uuidFutureMapMu.Unlock()

	return false
}
