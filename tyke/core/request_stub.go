package core

import (
	"sync"
	"time"

	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/component"
)

type futureEntry struct {
	ch        chan *TykeResponse
	createdAt time.Time
	timeoutMs uint32
}

type funcEntry struct {
	fn        func(*TykeResponse)
	createdAt time.Time
	timeoutMs uint32
}

var (
	uuidFutureMap   map[string]futureEntry
	uuidFutureMapMu sync.Mutex
	uuidFuncMap     map[string]funcEntry
	uuidFuncMapMu   sync.Mutex
)

func init() {
	uuidFutureMap = make(map[string]futureEntry)
	uuidFuncMap = make(map[string]funcEntry)
}

func RequestStubAddFuture(uuid string, ch chan *TykeResponse, timeoutMs uint32) {
	uuidFutureMapMu.Lock()
	defer uuidFutureMapMu.Unlock()
	uuidFutureMap[uuid] = futureEntry{ch: ch, createdAt: time.Now(), timeoutMs: timeoutMs}
	component.GetTimingWheel().AddTask(uuid, timeoutMs, component.TaskTypeFuture)
	common.LogDebug("Future entry added", "uuid", uuid, "timeout", timeoutMs)
}

func RequestStubSetFuture(response *TykeResponse) {
	uuidFutureMapMu.Lock()
	defer uuidFutureMapMu.Unlock()
	if entry, ok := uuidFutureMap[response.GetMsgUuid()]; ok {
		respCopy := *response
		entry.ch <- &respCopy
		delete(uuidFutureMap, response.GetMsgUuid())
		component.GetTimingWheel().RemoveTask(response.GetMsgUuid())
		common.LogDebug("Future result set", "uuid", response.GetMsgUuid())
	} else {
		common.LogWarn("Future entry not found for response", "uuid", response.GetMsgUuid())
	}
}

func RequestStubAddFunc(uuid string, fn func(*TykeResponse), timeoutMs uint32) {
	uuidFuncMapMu.Lock()
	defer uuidFuncMapMu.Unlock()
	uuidFuncMap[uuid] = funcEntry{fn: fn, createdAt: time.Now(), timeoutMs: timeoutMs}
	component.GetTimingWheel().AddTask(uuid, timeoutMs, component.TaskTypeFunc)
	common.LogDebug("Callback entry added", "uuid", uuid, "timeout", timeoutMs)
}

func RequestStubExecFunc(response *TykeResponse) {
	uuidFuncMapMu.Lock()
	entry, ok := uuidFuncMap[response.GetMsgUuid()]
	if ok {
		delete(uuidFuncMap, response.GetMsgUuid())
		component.GetTimingWheel().RemoveTask(response.GetMsgUuid())
	}
	uuidFuncMapMu.Unlock()

	if ok {
		common.LogDebug("Executing callback for response", "uuid", response.GetMsgUuid())
		entry.fn(response)
	} else {
		common.LogWarn("Callback entry not found for response", "uuid", response.GetMsgUuid())
	}
}

func RequestStubCleanupExpiredFuture(uuid string) {
	uuidFutureMapMu.Lock()
	defer uuidFutureMapMu.Unlock()
	if entry, ok := uuidFutureMap[uuid]; ok {
		timeoutResp := NewTykeResponse()
		timeoutResp.SetMsgUuid(uuid)
		timeoutResp.SetResult(-1, "timeout")
		entry.ch <- timeoutResp
		delete(uuidFutureMap, uuid)
		common.LogWarn("Expired future cleaned up", "uuid", uuid)
	}
}

func RequestStubCleanupExpiredFunc(uuid string) {
	uuidFuncMapMu.Lock()
	defer uuidFuncMapMu.Unlock()
	if _, ok := uuidFuncMap[uuid]; ok {
		delete(uuidFuncMap, uuid)
		common.LogWarn("Expired func cleaned up", "uuid", uuid)
	}
}
