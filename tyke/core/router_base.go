package core

import (
	"sync"

	"github.com/cppbugboy/tyke-go/tyke/common"
)

type RouterBase[FilterType any, HandlerFunc any] struct {
	routeTable map[string]RouteEntry[FilterType, HandlerFunc]
	rootGroup  *RouterGroup[FilterType, HandlerFunc]
	mu         sync.RWMutex
}

func NewRouterBase[FilterType any, HandlerFunc any]() *RouterBase[FilterType, HandlerFunc] {
	rb := &RouterBase[FilterType, HandlerFunc]{
		routeTable: make(map[string]RouteEntry[FilterType, HandlerFunc]),
	}
	rb.rootGroup = NewRouterGroup[FilterType, HandlerFunc]("", &rb.routeTable, nil)
	common.LogDebug("RouterBase initialized")
	return rb
}

func (rb *RouterBase[FilterType, HandlerFunc]) GetRoot() *RouterGroup[FilterType, HandlerFunc] {
	return rb.rootGroup
}

func (rb *RouterBase[FilterType, HandlerFunc]) GetRouteEntry(path string) *RouteEntry[FilterType, HandlerFunc] {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	if entry, ok := rb.routeTable[path]; ok {
		common.LogDebug("Route entry found", "path", path)
		return &entry
	}
	common.LogWarn("Route entry not found", "path", path)
	return nil
}
