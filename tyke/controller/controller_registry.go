package controller

var registeredControllers []ControllerBase

// RegisterController 注册一个控制器并调用其 RegisterMethod。
func RegisterController(c ControllerBase) {
	registeredControllers = append(registeredControllers, c)
	c.RegisterMethod()
}
