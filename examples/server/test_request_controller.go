package main

import (
	"fmt"

	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/controller"
	"github.com/tyke/tyke/tyke/core"
)

type TestRequestController struct{}

func (c *TestRequestController) RegisterMethod() {
	fmt.Println("TestRequestController::RegisterMethod()")

	rootGroup := core.GetRequestRouterInstance().GetRoot()
	testGroup := rootGroup.AddSubGroup("/test")
	testGroup.AddRouteHandler("/hello", Hello)
}

func Hello(request *core.TykeRequest, response *core.TykeResponse) {
	fmt.Println("TestRequestController::Test()")
	content := "hello world"
	response.SetContent(common.ContentTypeText, []byte(content))
}

func init() {
	controller.RegisterController(&TestRequestController{})
}
