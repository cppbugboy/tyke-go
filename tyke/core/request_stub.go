package core

import (
	"sync"
	"time"

	"github.com/tyke/tyke/tyke/common"
)

type futureEntry struct {
	ch        chan *TykeResponse
	createdAt time.Time
}

type funcEntry struct {
	fn        func(*TykeResponse)
	createdAt time.Time
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

func RequestStubAddFuture(uuid string, ch chan *TykeResponse) {
	uuidFutureMapMu.Lock()
	defer uuidFutureMapMu.Unlock()
	uuidFutureMap[uuid] = futureEntry{ch: ch, createdAt: time.Now()}
	common.LogDebug("Future entry added", "uuid", uuid)
}

func RequestStubSetFuture(response *TykeResponse) {
	uuidFutureMapMu.Lock()
	defer uuidFutureMapMu.Unlock()
	if entry, ok := uuidFutureMap[response.GetMsgUuid()]; ok {
		respCopy := *response
		entry.ch <- &respCopy
		delete(uuidFutureMap, response.GetMsgUuid())
		common.LogDebug("Future result set", "uuid", response.GetMsgUuid())
	} else {
		common.LogWarn("Future entry not found for response", "uuid", response.GetMsgUuid())
	}
}

func RequestStubAddFunc(msgUuid string, fn func(*TykeResponse)) {
	uuidFuncMapMu.Lock()
	defer uuidFuncMapMu.Unlock()
	uuidFuncMap[msgUuid] = funcEntry{fn: fn, createdAt: time.Now()}
	common.LogDebug("Callback entry added", "uuid", msgUuid)
}

func RequestStubExecFunc(response *TykeResponse) {
	uuidFuncMapMu.Lock()
	entry, ok := uuidFuncMap[response.GetMsgUuid()]
	if ok {
		delete(uuidFuncMap, response.GetMsgUuid())
	}
	uuidFuncMapMu.Unlock()

	if ok {
		common.LogDebug("Executing callback for response", "uuid", response.GetMsgUuid())
		entry.fn(response)
	} else {
		common.LogWarn("Callback entry not found for response", "uuid", response.GetMsgUuid())
	}
}
