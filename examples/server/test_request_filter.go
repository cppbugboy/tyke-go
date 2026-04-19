package main

import (
	"fmt"

	"github.com/tyke/tyke/tyke/core"
)

type TestRequestFilter struct{}

func (f *TestRequestFilter) Before(request *core.TykeRequest, response *core.TykeResponse) bool {
	fmt.Println("before")
	return true
}

func (f *TestRequestFilter) After(request *core.TykeRequest, response *core.TykeResponse) bool {
	fmt.Println("after")
	return true
}
