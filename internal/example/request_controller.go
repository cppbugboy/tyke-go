package example

import (
	"fmt"

	"github.com/tyke/tyke/pkg/common"
	"github.com/tyke/tyke/pkg/controller"
	"github.com/tyke/tyke/pkg/core"
)

type TestRequestController struct{}

func (c *TestRequestController) RegisterMethod() {
	controller.RegisterRequestMethod("/test/hello", Hello)
}

func Hello(req *core.TykeRequest, resp *core.TykeResponse) {
	fmt.Println("TestRequestController::Hello()")
	content := "hello world"
	resp.SetResult(200, "OK")
	resp.SetContent(common.ContentTypeText, []byte(content))
}

func init() {
	controller.RegisterController(&TestRequestController{})
}
