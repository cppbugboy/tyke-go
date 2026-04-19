package main

import (
	"fmt"
	"time"

	"github.com/tyke/tyke/tyke/core"
)

func main() {
	startResult := core.App().Start("39649d81-81c5-4f6e-b6a9-e768b55063be")
	if !startResult.HasValue() {
		fmt.Printf("start failed: %s\n", startResult.Err)
		return
	}
	for {
		time.Sleep(time.Second)
	}
}
