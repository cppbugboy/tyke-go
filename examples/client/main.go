// Package main 提供了一个演示用的 Tyke 客户端。
//
// 本示例启动一个 Tyke IPC 客户端，并演示所有四种 IPC 发送模式：
//   - 同步发送 (Send)：阻塞直到服务器响应
//   - 异步发送 (SendAsync)：发后即忘；响应通过 ResponseRouter 路由
//   - 带回调的异步发送 (SendAsyncWithFunc)：收到响应时调用回调函数
//   - 带 Future 的异步发送 (SendAsyncWithFuture)：返回一个可轮询的 Future
//
// 每种模式向示例服务器发送一个请求并打印结果。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tyke-go/common"
	"tyke-go/core"
)

const (
	// serverUuid 是示例服务器监听的众所周知的 UUID。
	serverUuid = "1879b1d8-8ab0-4542-8421-8d845eca6587"
	// clientListenerUuid 是此客户端用于监听异步响应的 UUID。
	clientListenerUuid = "c6ea2fe2-a1d0-4b90-a739-ce78cdaf7b6e"
)

// printRequestHeader 将格式化的请求摘要打印到标准输出。
func printRequestHeader(title string, targetUuid string, request *core.Request) {
	now := time.Now()
	fmt.Printf("\n========================================\n")
	fmt.Printf("[%s] %s\n", now.Format("2006-01-02 15:04:05"), title)
	fmt.Printf("========================================\n")
	fmt.Printf("目标UUID: %s\n", targetUuid)
	fmt.Printf("模块: %s\n", request.GetModule())
	fmt.Printf("路由: %s\n", request.GetRoute())
	if request.GetAsyncUUID() != "" {
		fmt.Printf("异步UUID: %s\n", request.GetAsyncUUID())
	}

	contentType, content := request.GetContent()
	if contentType == "json" && len(content) > 0 {
		var jsonData interface{}
		if err := json.Unmarshal(content, &jsonData); err == nil {
			prettyJSON, _ := json.MarshalIndent(jsonData, "", "  ")
			fmt.Printf("请求头: {}\n")
			fmt.Printf("请求体: %s\n", string(prettyJSON))
		}
	}
}

// printSyncResponse 将格式化的同步响应打印到标准输出。
func printSyncResponse(response *core.Response) {
	now := time.Now()
	status, reason := response.GetResult()

	fmt.Printf("----------------------------------------\n")
	fmt.Printf("[%s] 收到同步响应\n", now.Format("2006-01-02 15:04:05"))
	fmt.Printf("----------------------------------------\n")
	fmt.Printf("状态码: %d\n", status)
	fmt.Printf("原因: %s\n", reason)

	contentType, content := response.GetContent()
	if contentType == "json" && len(content) > 0 {
		var jsonData interface{}
		if err := json.Unmarshal(content, &jsonData); err == nil {
			prettyJSON, _ := json.MarshalIndent(jsonData, "", "  ")
			fmt.Printf("响应头: {}\n")
			fmt.Printf("响应体: %s\n", string(prettyJSON))
		}
	}
	fmt.Printf("========================================\n")
}

// printAsyncResponse 将格式化的异步响应打印到标准输出。
func printAsyncResponse(response *core.Response, methodName string) {
	now := time.Now()
	status, reason := response.GetResult()

	fmt.Printf("----------------------------------------\n")
	fmt.Printf("[%s] 收到异步响应 (%s)\n", now.Format("2006-01-02 15:04:05"), methodName)
	fmt.Printf("----------------------------------------\n")
	fmt.Printf("消息UUID: %s\n", response.GetMsgUUID())
	fmt.Printf("状态码: %d\n", status)
	fmt.Printf("原因: %s\n", reason)
	fmt.Printf("模块: %s\n", response.GetModule())
	fmt.Printf("路由: %s\n", response.GetRoute())

	contentType, content := response.GetContent()
	if contentType == "json" && len(content) > 0 {
		var jsonData interface{}
		if err := json.Unmarshal(content, &jsonData); err == nil {
			prettyJSON, _ := json.MarshalIndent(jsonData, "", "  ")
			fmt.Printf("响应头: {}\n")
			fmt.Printf("响应体: %s\n", string(prettyJSON))
		}
	}
	fmt.Printf("========================================\n")
}

// demoSyncRequest 演示通过 Request.Send 发送同步请求。
// 调用会阻塞直到服务器发送响应或超时到期。
func demoSyncRequest() {
	fmt.Println("\n>>> 1. Sync Request Demo (Send)")

	request := core.AcquireRequest()
	defer core.ReleaseRequest(request)

	request.SetModule("user_service")
	request.SetRoute("/api/user/login")

	loginData := map[string]interface{}{
		"username": "test_user",
		"password": "test_password",
	}
	loginBytes, _ := json.Marshal(loginData)
	request.SetContent(common.ContentTypeJson, loginBytes)

	request.AddMetadata("source", common.NewJsonValue("go_client"))
	request.AddMetadata("version", common.NewJsonValue("1.0"))

	printRequestHeader("发送同步请求", serverUuid, request)

	var response core.Response
	result := request.Send(serverUuid, &response)
	if result.HasValue() {
		printSyncResponse(&response)
	} else {
		fmt.Printf("同步请求失败: %s\n", result.Err)
	}
}

// demoSendAsync 演示通过 Request.SendAsync 发送发后即忘的异步请求。
// 响应由服务器的 ResponseRouter 分发到已注册的响应处理器。
func demoSendAsync() {
	fmt.Println("\n>>> 2. Async Request Demo - SendAsync (fire-and-forget)")

	request := core.AcquireRequest()
	defer core.ReleaseRequest(request)

	request.SetModule("data_service")
	request.SetRoute("/api/async/process")
	request.SetAsyncUUID(clientListenerUuid)

	processData := map[string]interface{}{
		"task_type": "background_process",
		"priority":  1,
	}
	processBytes, _ := json.Marshal(processData)
	request.SetContent(common.ContentTypeJson, processBytes)

	printRequestHeader("发送异步请求 (SendAsync)", serverUuid, request)

	result := request.SendAsync(serverUuid)
	if result.HasValue() {
		fmt.Println("异步请求已发送，响应将由 ResponseRouter 分发到响应控制器")
	} else {
		fmt.Printf("异步请求失败: %s\n", result.Err)
	}
}

// demoSendAsyncWithFunc 演示带回调函数的异步请求。
// 提供的回调函数在服务器发送响应时被调用。
func demoSendAsyncWithFunc() {
	fmt.Println("\n>>> 3. Async Request Demo - SendAsyncWithFunc (callback)")

	request := core.AcquireRequest()
	defer core.ReleaseRequest(request)

	request.SetModule("data_service")
	request.SetRoute("/api/async/process")
	request.SetAsyncUUID(clientListenerUuid)

	processData := map[string]interface{}{
		"task_type": "callback_process",
		"priority":  2,
	}
	processBytes, _ := json.Marshal(processData)
	request.SetContent(common.ContentTypeJson, processBytes)

	printRequestHeader("发送异步请求 (SendAsyncWithFunc)", serverUuid, request)

	result := request.SendAsyncWithFunc(serverUuid, func(response *core.Response) {
		printAsyncResponse(response, "SendAsyncWithFunc 回调")
	})

	if result.HasValue() {
		fmt.Println("异步请求已发送，等待回调执行...")
	} else {
		fmt.Printf("异步请求失败: %s\n", result.Err)
	}
}

// demoSendAsyncWithFuture 演示带 Future/Promise 的异步请求。
// 返回的 ResponseFuture 可用于阻塞等待直到响应到达。
func demoSendAsyncWithFuture() {
	fmt.Println("\n>>> 4. Async Request Demo - SendAsyncWithFuture (Future/Promise)")

	request := core.AcquireRequest()
	defer core.ReleaseRequest(request)

	request.SetModule("data_service")
	request.SetRoute("/api/async/process")
	request.SetAsyncUUID(clientListenerUuid)

	processData := map[string]interface{}{
		"task_type": "future_process",
		"priority":  3,
	}
	processBytes, _ := json.Marshal(processData)
	request.SetContent(common.ContentTypeJson, processBytes)

	printRequestHeader("发送异步请求 (SendAsyncWithFuture)", serverUuid, request)

	future, err := request.SendAsyncWithFuture(serverUuid)
	if err != nil {
		fmt.Printf("异步请求失败: %s\n", err)
		return
	}

	fmt.Println("异步请求已发送，等待 Future 结果...")
	response := future.GetResponse()
	printAsyncResponse(response, "SendAsyncWithFuture")
}

func main() {
	fmt.Println("========================================")
	fmt.Println("  Tyke 示例客户端")
	fmt.Println("========================================")
	fmt.Println()

	framework := core.App()
	framework.SetThreadPoolCount(4)
	framework.SetLogConfig("./tyke_client.log", "debug", 1024, 5)

	result := framework.Start(clientListenerUuid)
	if !result.HasValue() {
		fmt.Printf("客户端启动失败: %s\n", result.Err)
		os.Exit(1)
	}

	fmt.Printf("客户端监听已启动，UUID: %s\n\n", clientListenerUuid)

	time.Sleep(500 * time.Millisecond)

	demoSyncRequest()

	time.Sleep(200 * time.Millisecond)

	demoSendAsync()

	time.Sleep(200 * time.Millisecond)

	demoSendAsyncWithFunc()

	time.Sleep(200 * time.Millisecond)

	demoSendAsyncWithFuture()

	fmt.Println("\n等待异步响应处理完成...")
	time.Sleep(3 * time.Second)

	fmt.Println("\n所有示例执行完毕")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n正在关闭客户端...")
	framework.Shutdown()
	fmt.Println("客户端已关闭")
}
