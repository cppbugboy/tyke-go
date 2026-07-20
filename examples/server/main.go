// Package main 提供了一个演示用的 Tyke 服务器。
//
// 本示例启动一个 Tyke IPC 服务器，监听在众所周知的 UUID
// "1879b1d8-8ab0-4542-8421-8d845eca6587" 上。服务器通过 example controllers 包
// （通过其 init() 副作用导入）注册请求路由处理器，并处理传入的同步/异步请求，
// 直到被 SIGINT 或 SIGTERM 中断。
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "tyke-go-project/examples/controllers"
	"tyke-go/core"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("  Tyke Example Server")
	fmt.Println("========================================")
	fmt.Println()

	framework := core.App()
	framework.SetThreadPoolCount(4)
	framework.SetLogConfig("./tyke_server.log", "debug", 1024, 5)

	// 在众所周知的服务器 UUID 上启动 IPC 服务器。
	result := framework.Start("1879b1d8-8ab0-4542-8421-8d845eca6587")
	if !result.HasValue() {
		fmt.Printf("Server start failed: %s\n", result.Err)
		os.Exit(1)
	}

	fmt.Println("Server started, listening on UUID: 1879b1d8-8ab0-4542-8421-8d845eca6587")
	fmt.Println("Press Ctrl+C to stop the server...")
	fmt.Println()

	// 阻塞直到收到 SIGINT/SIGTERM。
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down server...")
	framework.Shutdown()
	fmt.Println("Server stopped")
}
