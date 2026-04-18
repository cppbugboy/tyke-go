package core

import (
	"sync"

	"github.com/tyke/tyke/pkg/common"
)

type RequestHandlerFunc func(req *TykeRequest, resp *TykeResponse)
type ResponseHandlerFunc func(resp *TykeResponse)

type RouteEntry[F any, H any] struct {
	Path    string
	Handler H
	Filters []F
}

type RouterGroup[F any, H any] struct {
	prefix       string
	filters      []F
	subGroups    []*RouterGroup[F, H]
	routes       map[string]*RouteEntry[F, H]
	onRouteAdded func(entry *RouteEntry[F, H])
}

func newRouterGroup[F any, H any](prefix string, onRouteAdded func(entry *RouteEntry[F, H])) *RouterGroup[F, H] {
	return &RouterGroup[F, H]{
		prefix:       prefix,
		routes:       make(map[string]*RouteEntry[F, H]),
		onRouteAdded: onRouteAdded,
	}
}

func (g *RouterGroup[F, H]) AddFilter(f F) *RouterGroup[F, H] {
	g.filters = append(g.filters, f)
	return g
}

func (g *RouterGroup[F, H]) AddSubGroup(subPrefix string) *RouterGroup[F, H] {
	sub := newRouterGroup(g.prefix+subPrefix, g.onRouteAdded)
	sub.filters = make([]F, len(g.filters))
	copy(sub.filters, g.filters)
	g.subGroups = append(g.subGroups, sub)
	return sub
}

func (g *RouterGroup[F, H]) AddRouteHandler(path string, handler H) *RouterGroup[F, H] {
	fullPath := g.prefix + path
	allFilters := make([]F, len(g.filters))
	copy(allFilters, g.filters)
	entry := &RouteEntry[F, H]{
		Path:    fullPath,
		Handler: handler,
		Filters: allFilters,
	}
	g.routes[fullPath] = entry
	if g.onRouteAdded != nil {
		g.onRouteAdded(entry)
	}
	return g
}

func (g *RouterGroup[F, H]) findRoute(path string) *RouteEntry[F, H] {
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

type Router[F any, H any] struct {
	root       *RouterGroup[F, H]
	routeTable map[string]*RouteEntry[F, H]
	mu         sync.RWMutex
}

func newRouter[F any, H any]() *Router[F, H] {
	r := &Router[F, H]{
		routeTable: make(map[string]*RouteEntry[F, H]),
	}
	r.root = newRouterGroup[F, H]("", func(entry *RouteEntry[F, H]) {
		r.mu.Lock()
		r.routeTable[entry.Path] = entry
		r.mu.Unlock()
	})
	return r
}

func (r *Router[F, H]) GetRoot() *RouterGroup[F, H] {
	return r.root
}

func (r *Router[F, H]) GetRouteEntry(path string) *RouteEntry[F, H] {
	r.mu.RLock()
	entry, ok := r.routeTable[path]
	r.mu.RUnlock()
	if ok {
		return entry
	}
	return nil
}

type RequestRouteEntry = RouteEntry[RequestFilter, RequestHandlerFunc]
type RequestRouterGroup = RouterGroup[RequestFilter, RequestHandlerFunc]
type RequestRouter = Router[RequestFilter, RequestHandlerFunc]

type ResponseRouteEntry = RouteEntry[ResponseFilter, ResponseHandlerFunc]
type ResponseRouterGroup = RouterGroup[ResponseFilter, ResponseHandlerFunc]
type ResponseRouter = Router[ResponseFilter, ResponseHandlerFunc]

var (
	requestRouter     *RequestRouter
	requestRouterOnce sync.Once
)

func GetRequestRouter() *RequestRouter {
	requestRouterOnce.Do(func() {
		requestRouter = newRouter[RequestFilter, RequestHandlerFunc]()
	})
	return requestRouter
}

var (
	responseRouter     *ResponseRouter
	responseRouterOnce sync.Once
)

func GetResponseRouter() *ResponseRouter {
	responseRouterOnce.Do(func() {
		responseRouter = newRouter[ResponseFilter, ResponseHandlerFunc]()
	})
	return responseRouter
}

func init() {
	_ = common.ProtocolMagic
}
