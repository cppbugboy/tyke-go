package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/tyke/tyke/pkg/common"
)

type futureEntry struct {
	ch        chan *TykeResponse
	createdAt time.Time
}

type funcEntry struct {
	fn        func(*TykeResponse)
	createdAt time.Time
}

type RequestStub struct {
	futureMap map[string]*futureEntry
	funcMap   map[string]*funcEntry
	futureMu  sync.RWMutex
	funcMu    sync.RWMutex
}

var (
	globalStub     *RequestStub
	globalStubOnce sync.Once
)

func GetRequestStub() *RequestStub {
	globalStubOnce.Do(func() {
		globalStub = &RequestStub{
			futureMap: make(map[string]*futureEntry),
			funcMap:   make(map[string]*funcEntry),
		}
	})
	return globalStub
}

func (s *RequestStub) AddFuture(uuid string, ch chan *TykeResponse) {
	s.futureMu.Lock()
	defer s.futureMu.Unlock()
	s.futureMap[uuid] = &futureEntry{
		ch:        ch,
		createdAt: time.Now(),
	}
}

func (s *RequestStub) SetFuture(resp *TykeResponse) {
	msgUuid := resp.GetMsgUuid()
	s.futureMu.Lock()
	entry, ok := s.futureMap[msgUuid]
	if ok {
		delete(s.futureMap, msgUuid)
	}
	s.futureMu.Unlock()
	if ok {
		entry.ch <- resp
	}
}

func (s *RequestStub) DelFuture(msgUuid string) {
	s.futureMu.Lock()
	defer s.futureMu.Unlock()
	delete(s.futureMap, msgUuid)
}

func (s *RequestStub) AddFunc(msgUuid string, fn func(*TykeResponse)) {
	s.funcMu.Lock()
	defer s.funcMu.Unlock()
	s.funcMap[msgUuid] = &funcEntry{
		fn:        fn,
		createdAt: time.Now(),
	}
}

func (s *RequestStub) ExecFunc(resp *TykeResponse) {
	msgUuid := resp.GetMsgUuid()
	s.futureMu.Lock()
	entry, ok := s.funcMap[msgUuid]
	if ok {
		delete(s.futureMap, msgUuid)
	}
	s.futureMu.Unlock()
	if ok {
		entry.fn(resp)
	}
}

func (s *RequestStub) DelFunc(msgUuid string) {
	s.funcMu.Lock()
	defer s.funcMu.Unlock()
	delete(s.funcMap, msgUuid)
}

func (s *RequestStub) CleanupExpired(timeoutMs uint32) {
	if timeoutMs == 0 {
		timeoutMs = 30000
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond
	now := time.Now()

	s.futureMu.Lock()
	for uuid, entry := range s.futureMap {
		if now.Sub(entry.createdAt) > timeout {
			delete(s.futureMap, uuid)
		}
	}
	s.futureMu.Unlock()

	s.funcMu.Lock()
	for uuid, entry := range s.funcMap {
		if now.Sub(entry.createdAt) > timeout {
			delete(s.funcMap, uuid)
		}
	}
	s.funcMu.Unlock()
}

func (s *RequestStub) HandleResponse(resp *TykeResponse) {
	msgType := resp.GetMessageType()

	switch msgType {
	case common.MessageTypeResponseAsyncFuture:
		s.SetFuture(resp)
	case common.MessageTypeResponseAsyncFunc:
		s.ExecFunc(resp)
	default:
		common.LogError(fmt.Sprintf("Unknown message type: %s", msgType))
	}
}
