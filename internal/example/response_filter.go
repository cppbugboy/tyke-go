package example

import (
	"fmt"

	"github.com/tyke/tyke/pkg/core"
)

type TestResponseFilter struct{}

func (f *TestResponseFilter) Before(resp *core.TykeResponse) bool {
	fmt.Printf("ResponseFilter Before: route=%s\n", resp.GetRoute())
	return true
}

func (f *TestResponseFilter) After(resp *core.TykeResponse) bool {
	fmt.Printf("ResponseFilter After: route=%s\n", resp.GetRoute())
	return true
}
