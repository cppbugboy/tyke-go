package core

import "sync"

var (
	requestRouterInstance *RouterBase[RequestFilter, RequestHandlerFunc]
	requestRouterOnce     sync.Once
)

func GetRequestRouter() *RouterBase[RequestFilter, RequestHandlerFunc] {
	requestRouterOnce.Do(func() {
		requestRouterInstance = NewRouterBase[RequestFilter, RequestHandlerFunc]()
	})
	return requestRouterInstance
}
