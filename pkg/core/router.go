package core

import (
	"sync"

	"github.com/tyke/tyke/pkg/common"
)

type RequestHandlerFunc func(req *TykeRequest, resp *TykeResponse)
type ResponseHandlerFunc func(resp *TykeResponse)

type RequestRouteEntry struct {
	Path    string
	Handler RequestHandlerFunc
	Filters []RequestFilter
}

type ResponseRouteEntry struct {
	Path    string
	Handler ResponseHandlerFunc
	Filters []ResponseFilter
}

type RequestRouterGroup struct {
	prefix    string
	filters   []RequestFilter
	subGroups []*RequestRouterGroup
	routes    map[string]*RequestRouteEntry
}

func NewRequestRouterGroup(prefix string) *RequestRouterGroup {
	return &RequestRouterGroup{
		prefix: prefix,
		routes: make(map[string]*RequestRouteEntry),
	}
}

func (g *RequestRouterGroup) AddFilter(f RequestFilter) *RequestRouterGroup {
	g.filters = append(g.filters, f)
	return g
}

func (g *RequestRouterGroup) AddSubGroup(subPrefix string) *RequestRouterGroup {
	sub := NewRequestRouterGroup(g.prefix + subPrefix)
	sub.filters = make([]RequestFilter, len(g.filters))
	copy(sub.filters, g.filters)
	g.subGroups = append(g.subGroups, sub)
	return sub
}

func (g *RequestRouterGroup) AddRouteHandler(path string, handler RequestHandlerFunc) *RequestRouterGroup {
	fullPath := g.prefix + path
	allFilters := make([]RequestFilter, len(g.filters))
	copy(allFilters, g.filters)
	entry := &RequestRouteEntry{
		Path:    fullPath,
		Handler: handler,
		Filters: allFilters,
	}
	g.routes[fullPath] = entry
	return g
}

type RequestRouter struct {
	root *RequestRouterGroup
}

var (
	requestRouter     *RequestRouter
	requestRouterOnce sync.Once
)

func GetRequestRouter() *RequestRouter {
	requestRouterOnce.Do(func() {
		requestRouter = &RequestRouter{
			root: NewRequestRouterGroup(""),
		}
	})
	return requestRouter
}

func (r *RequestRouter) GetRoot() *RequestRouterGroup {
	return r.root
}

func (r *RequestRouter) GetRouteEntry(path string) *RequestRouteEntry {
	if entry, ok := r.root.routes[path]; ok {
		return entry
	}
	for _, sub := range r.root.subGroups {
		if entry := sub.findRoute(path); entry != nil {
			return entry
		}
	}
	return nil
}

func (g *RequestRouterGroup) findRoute(path string) *RequestRouteEntry {
	if entry, ok := g.routes[path]; ok {
		return entry
	}
	for _, sub := range g.subGroups {
		if entry := sub.findRoute(path); entry != nil {
			return entry
		}
	}
	return nil
}

type ResponseRouterGroup struct {
	prefix    string
	filters   []ResponseFilter
	subGroups []*ResponseRouterGroup
	routes    map[string]*ResponseRouteEntry
}

func NewResponseRouterGroup(prefix string) *ResponseRouterGroup {
	return &ResponseRouterGroup{
		prefix: prefix,
		routes: make(map[string]*ResponseRouteEntry),
	}
}

func (g *ResponseRouterGroup) AddFilter(f ResponseFilter) *ResponseRouterGroup {
	g.filters = append(g.filters, f)
	return g
}

func (g *ResponseRouterGroup) AddSubGroup(subPrefix string) *ResponseRouterGroup {
	sub := NewResponseRouterGroup(g.prefix + subPrefix)
	sub.filters = make([]ResponseFilter, len(g.filters))
	copy(sub.filters, g.filters)
	g.subGroups = append(g.subGroups, sub)
	return sub
}

func (g *ResponseRouterGroup) AddRouteHandler(path string, handler ResponseHandlerFunc) *ResponseRouterGroup {
	fullPath := g.prefix + path
	allFilters := make([]ResponseFilter, len(g.filters))
	copy(allFilters, g.filters)
	entry := &ResponseRouteEntry{
		Path:    fullPath,
		Handler: handler,
		Filters: allFilters,
	}
	g.routes[fullPath] = entry
	return g
}

type ResponseRouter struct {
	root *ResponseRouterGroup
}

var (
	responseRouter     *ResponseRouter
	responseRouterOnce sync.Once
)

func GetResponseRouter() *ResponseRouter {
	responseRouterOnce.Do(func() {
		responseRouter = &ResponseRouter{
			root: NewResponseRouterGroup(""),
		}
	})
	return responseRouter
}

func (r *ResponseRouter) GetRoot() *ResponseRouterGroup {
	return r.root
}

func (r *ResponseRouter) GetRouteEntry(path string) *ResponseRouteEntry {
	if entry, ok := r.root.routes[path]; ok {
		return entry
	}
	for _, sub := range r.root.subGroups {
		if entry := sub.findRoute(path); entry != nil {
			return entry
		}
	}
	return nil
}

func (g *ResponseRouterGroup) findRoute(path string) *ResponseRouteEntry {
	if entry, ok := g.routes[path]; ok {
		return entry
	}
	for _, sub := range g.subGroups {
		if entry := sub.findRoute(path); entry != nil {
			return entry
		}
	}
	return nil
}

func init() {
	_ = common.ProtocolMagic
}
