package core

import "sync"

var (
	responseRouterInstance *RouterBase[ResponseFilter, ResponseHandlerFunc]
	responseRouterOnce     sync.Once
)

func GetResponseRouterInstance() *RouterBase[ResponseFilter, ResponseHandlerFunc] {
	responseRouterOnce.Do(func() {
		responseRouterInstance = NewRouterBase[ResponseFilter, ResponseHandlerFunc]()
	})
	return responseRouterInstance
}

func GetResponseRouter() *RouterBase[ResponseFilter, ResponseHandlerFunc] {
	return GetResponseRouterInstance()
}
