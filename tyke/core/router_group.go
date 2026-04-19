package core

type RouteEntry[FilterType any, HandlerFunc any] struct {
	Handler     HandlerFunc
	FilterChain []FilterType
}

type RouterGroup[FilterType any, HandlerFunc any] struct {
	prefix         string
	filterChain    []FilterType
	parent         *RouterGroup[FilterType, HandlerFunc]
	globalRegistry *map[string]RouteEntry[FilterType, HandlerFunc]
}

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

func (g *RouterGroup[FilterType, HandlerFunc]) AddFilter(filter FilterType) *RouterGroup[FilterType, HandlerFunc] {
	g.filterChain = append(g.filterChain, filter)
	return g
}

func (g *RouterGroup[FilterType, HandlerFunc]) AddSubGroup(subPrefix string) *RouterGroup[FilterType, HandlerFunc] {
	return NewRouterGroup(g.prefix+subPrefix, g.globalRegistry, g)
}

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

func (g *RouterGroup[FilterType, HandlerFunc]) CollectFilters(chain *[]FilterType) {
	if g.parent != nil {
		g.parent.CollectFilters(chain)
	}
	*chain = append(*chain, g.filterChain...)
}
