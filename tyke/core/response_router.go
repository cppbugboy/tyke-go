package core

import "sync"

var (
	responseRouterInstance *RouterBase[ResponseFilter, ResponseHandlerFunc]
	responseRouterOnce     sync.Once
)

func GetResponseRouter() *RouterBase[ResponseFilter, ResponseHandlerFunc] {
	responseRouterOnce.Do(func() {
		responseRouterInstance = NewRouterBase[ResponseFilter, ResponseHandlerFunc]()
	})
	return responseRouterInstance
}
