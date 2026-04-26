package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cppbugboy/tyke-go/examples/controllers"
	"github.com/cppbugboy/tyke-go/tyke/core"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("  Tyke 示例服务端")
	fmt.Println("========================================")
	fmt.Println()

	framework := core.App()
	framework.SetThreadPoolCount(4)
	framework.SetLogConfig("./tyke_server.log", "debug", 1024, 5)

	core.RegisterController(controllers.NewExampleRequestController())
	core.RegisterController(controllers.NewExampleResponseController())

	result := framework.Start("tyke_server_example")
	if !result.HasValue() {
		fmt.Printf("服务端启动失败: %s\n", result.Err)
		os.Exit(1)
	}

	fmt.Println("服务端已启动，监听UUID: tyke_server_example")
	fmt.Println("按 Ctrl+C 停止服务端...")
	fmt.Println()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n正在关闭服务端...")
	framework.Shutdown()
	fmt.Println("服务端已关闭")
}
