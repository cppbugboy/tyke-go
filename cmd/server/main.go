package main

import (
	_ "github.com/tyke/tyke/internal/example"

	"github.com/tyke/tyke/pkg/core"
)

func main() {
	result := core.App().Start("39649d81-81c5-4f6e-b6a9-e768b55063be")
	if result != nil {
		core.LogError("start failed: %v", result)
		return
	}

	select {}
}
