package controller

import (
	"sync"

	"github.com/tyke/tyke/pkg/core"
)

type ControllerBase interface {
	RegisterMethod()
}

type ControllerRegistry struct {
	controllers []ControllerBase
}

var (
	registry     *ControllerRegistry
	registryOnce sync.Once
)

func init() {
	core.RunControllersFunc = RunAllControllers
}

func GetRegistry() *ControllerRegistry {
	registryOnce.Do(func() {
		registry = &ControllerRegistry{
			controllers: make([]ControllerBase, 0),
		}
	})
	return registry
}

func RegisterController(c ControllerBase) {
	r := GetRegistry()
	r.controllers = append(r.controllers, c)
}

func RunAllControllers() {
	r := GetRegistry()
	for _, c := range r.controllers {
		c.RegisterMethod()
	}
}

type RequestController struct{}

func (rc *RequestController) RegisterMethod() {}

type ResponseController struct{}

func (rc *ResponseController) RegisterMethod() {}

func RegisterRequestMethod(path string, handler core.RequestHandlerFunc) {
	core.GetRequestRouter().GetRoot().AddRouteHandler(path, handler)
}

func RegisterResponseMethod(path string, handler core.ResponseHandlerFunc) {
	core.GetResponseRouter().GetRoot().AddRouteHandler(path, handler)
}
