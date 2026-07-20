// Package core implements the Tyke framework kernel.
//
// This file defines the generic RouteEntry and RouterGroup types used to build
// the route table tree with prefix-based grouping and filter chains.
package core

// RouteEntry associates a handler function with its filter chain for a given route.
type RouteEntry[FilterType any, HandlerFunc any] struct {
	Handler     HandlerFunc
	FilterChain []FilterType
}

// RouterGroup represents a group of routes sharing a common path prefix and filter chain.
type RouterGroup[FilterType any, HandlerFunc any] struct {
	prefix         string
	filterChain    []FilterType
	parent         *RouterGroup[FilterType, HandlerFunc]
	globalRegistry *map[string]RouteEntry[FilterType, HandlerFunc]
}

// NewRouterGroup creates a new RouterGroup with the given prefix and global registry reference.
func NewRouterGroup[FilterType any, HandlerFunc any](
	prefix string,
	globalRegistry *map[string]RouteEntry[FilterType, HandlerFunc],
	parent *RouterGroup[FilterType, HandlerFunc],
) *RouterGroup[FilterType, HandlerFunc] {
	return &RouterGroup[FilterType, HandlerFunc]{
		prefix:         prefix,
		globalRegistry: globalRegistry,
		parent:         parent,
	}
}

// AddFilter appends a filter to this group's filter chain. Returns the group for chaining.
func (g *RouterGroup[FilterType, HandlerFunc]) AddFilter(filter FilterType) *RouterGroup[FilterType, HandlerFunc] {
	g.filterChain = append(g.filterChain, filter)
	return g
}

// AddSubGroup creates a child RouterGroup with the combined prefix (parent + subPrefix).
func (g *RouterGroup[FilterType, HandlerFunc]) AddSubGroup(subPrefix string) *RouterGroup[FilterType, HandlerFunc] {
	return NewRouterGroup(g.prefix+subPrefix, g.globalRegistry, g)
}

// AddRouteHandler registers a handler for the given relative path. The full path is
// formed by concatenating the group prefix and path. The handler is stored with the
// accumulated filter chain from all ancestor groups.
func (g *RouterGroup[FilterType, HandlerFunc]) AddRouteHandler(path string, handler HandlerFunc) {
	fullPath := g.prefix + path
	var fullChain []FilterType
	g.CollectFilters(&fullChain)
	if g.globalRegistry != nil {
		(*g.globalRegistry)[fullPath] = RouteEntry[FilterType, HandlerFunc]{
			Handler:     handler,
			FilterChain: fullChain,
		}
	}
}

// CollectFilters walks up the ancestry tree and accumulates all filter chains into the provided slice.
func (g *RouterGroup[FilterType, HandlerFunc]) CollectFilters(chain *[]FilterType) {
	if g.parent != nil {
		g.parent.CollectFilters(chain)
	}
	*chain = append(*chain, g.filterChain...)
}
