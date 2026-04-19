package main

import "github.com/tyke/tyke/tyke/core"

type TestResponseFilter struct{}

func (f *TestResponseFilter) Before(response *core.TykeResponse) bool {
	return true
}

func (f *TestResponseFilter) After(response *core.TykeResponse) bool {
	return true
}
