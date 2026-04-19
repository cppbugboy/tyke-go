package main

import "github.com/tyke/tyke/tyke/controller"

type TestResponseController struct{}

func (c *TestResponseController) RegisterMethod() {
}

func init() {
	controller.RegisterController(&TestResponseController{})
}
