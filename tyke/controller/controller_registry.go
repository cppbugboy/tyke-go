package controller

var registeredControllers []ControllerBase

func RegisterController(c ControllerBase) {
	registeredControllers = append(registeredControllers, c)
	c.RegisterMethod()
}

func GetRegisteredControllers() []ControllerBase {
	return registeredControllers
}
