package example

import (
	"fmt"

	"github.com/tyke/tyke/pkg/controller"
	"github.com/tyke/tyke/pkg/core"
)

type TestResponseController struct{}

func (c *TestResponseController) RegisterMethod() {
	controller.RegisterResponseMethod("/test/hello", ResponseCallback)
}

func ResponseCallback(resp *core.TykeResponse) {
	_, content := resp.GetContent()
	fmt.Printf("Response callback: %s\n", string(content))
}

func init() {
	controller.RegisterController(&TestResponseController{})
}
