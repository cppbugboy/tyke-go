package example

import (
	"fmt"

	"github.com/tyke/tyke/pkg/core"
)

type TestRequestFilter struct{}

func (f *TestRequestFilter) Before(req *core.TykeRequest, resp *core.TykeResponse) bool {
	fmt.Printf("RequestFilter Before: route=%s\n", req.GetRoute())
	return true
}

func (f *TestRequestFilter) After(req *core.TykeRequest, resp *core.TykeResponse) bool {
	fmt.Printf("RequestFilter After: route=%s\n", req.GetRoute())
	return true
}
