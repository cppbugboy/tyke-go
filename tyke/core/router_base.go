// Package core implements the Tyke framework kernel.
//
// This file defines the generic RouterBase type, the core routing table that maps
// full paths to route entries with their handler and filter chain.
package core

import (
	"sync"

	"tyke-go/common"
)

// RouterBase is a generic routing table that maps full route paths to RouteEntry
// values. It is parameterized by filter and handler function types and provides
// a root RouterGroup for building the route tree.
type RouterBase[FilterType any, HandlerFunc any] struct {
	routeTable map[string]RouteEntry[FilterType, HandlerFunc]
	rootGroup  *RouterGroup[FilterType, HandlerFunc]
	mu         sync.RWMutex
}

// NewRouterBase creates a new RouterBase with an initialized route table and root group.
func NewRouterBase[FilterType any, HandlerFunc any]() *RouterBase[FilterType, HandlerFunc] {
	rb := &RouterBase[FilterType, HandlerFunc]{
		routeTable: make(map[string]RouteEntry[FilterType, HandlerFunc]),
	}
	rb.rootGroup = NewRouterGroup[FilterType, HandlerFunc]("", &rb.routeTable, nil)
	common.LogDebug("RouterBase initialized")
	return rb
}

// GetRoot returns the root RouterGroup for building the route tree.
func (rb *RouterBase[FilterType, HandlerFunc]) GetRoot() *RouterGroup[FilterType, HandlerFunc] {
	return rb.rootGroup
}

// GetRouteEntry looks up a route entry by its full path. Returns nil if not found.
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
