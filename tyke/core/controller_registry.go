package core

var registeredControllers []ControllerBase

func RegisterController(c ControllerBase) {
	registeredControllers = append(registeredControllers, c)
	c.RegisterMethod()
}
