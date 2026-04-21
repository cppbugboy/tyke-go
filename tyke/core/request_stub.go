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
	uuidFutureMap   = make(map[string]futureEntry)
	uuidFutureMapMu sync.Mutex
	uuidFuncMap     = make(map[string]funcEntry)
	uuidFuncMapMu   sync.Mutex
)

func RequestStubAddFuture(uuid string, ch chan *TykeResponse, timeoutMs uint32) {
	uuidFutureMapMu.Lock()
	uuidFutureMap[uuid] = futureEntry{ch: ch, createdAt: time.Now(), timeoutMs: timeoutMs}
	uuidFutureMapMu.Unlock()
	component.GetTimingWheel().AddTask(uuid, timeoutMs, component.TaskTypeFuture)
	common.LogDebug("Future entry added", "uuid", uuid, "timeout", timeoutMs)
}

func RequestStubSetFuture(response *TykeResponse) {
	var extractedCh chan *TykeResponse
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
		respCopy := *response
		select {
		case extractedCh <- &respCopy:
		default:
			common.LogWarn("Future channel full, dropping response", "uuid", response.GetMsgUUID())
		}
	}
}

func RequestStubAddFunc(uuid string, fn func(*TykeResponse), timeoutMs uint32) {
	uuidFuncMapMu.Lock()
	uuidFuncMap[uuid] = funcEntry{fn: fn, createdAt: time.Now(), timeoutMs: timeoutMs}
	uuidFuncMapMu.Unlock()
	component.GetTimingWheel().AddTask(uuid, timeoutMs, component.TaskTypeFunc)
	common.LogDebug("Callback entry added", "uuid", uuid, "timeout", timeoutMs)
}

func RequestStubExecFunc(response *TykeResponse) {
	var extractedFn func(*TykeResponse)
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
		extractedFn(response)
	}
}

func RequestStubCleanupExpiredFuture(uuid string) {
	var extractedCh chan *TykeResponse
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
		timeoutResp := NewTykeResponse()
		timeoutResp.SetMsgUUID(uuid)
		timeoutResp.SetResult(-1, "timeout")
		select {
		case extractedCh <- timeoutResp:
		default:
			common.LogWarn("Future channel full on timeout, dropping", "uuid", uuid)
		}
	}
}

func RequestStubCleanupExpiredFunc(uuid string) {
	uuidFuncMapMu.Lock()
	if _, ok := uuidFuncMap[uuid]; ok {
		delete(uuidFuncMap, uuid)
		common.LogWarn("Expired func cleaned up", "uuid", uuid)
	}
	uuidFuncMapMu.Unlock()
}

func RequestStubExecFuncOrSetFuture(response *TykeResponse) bool {
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
		respCopy := *response
		select {
		case extractedCh <- &respCopy:
		default:
			common.LogWarn("Fallback future channel full, dropping response", "uuid", uuid)
		}
		return true
	}
	uuidFutureMapMu.Unlock()

	return false
}
