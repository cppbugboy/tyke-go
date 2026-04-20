package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tyke/tyke/examples/controllers"
	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/controller"
	"github.com/tyke/tyke/tyke/core"
)

const (
	serverUuid         = "tyke_server_example"
	clientListenerUuid = "tyke_client_listener_go"
)

func printRequestHeader(title string, targetUuid string, request *core.TykeRequest) {
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

func printSyncResponse(response *core.TykeResponse) {
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

func printAsyncResponse(response *core.TykeResponse, methodName string) {
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

func demoSyncRequest() {
	fmt.Println("\n>>> 1. 同步请求示例 (Send)")

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

	printRequestHeader("发送同步请求", serverUuid, request)

	var response core.TykeResponse
	result := request.Send(serverUuid, &response)
	if result.HasValue() {
		printSyncResponse(&response)
	} else {
		fmt.Printf("同步请求失败: %s\n", result.Err)
	}
}

func demoSendAsync() {
	fmt.Println("\n>>> 2. 异步请求示例 - SendAsync (即发即弃)")

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

func demoSendAsyncWithFunc() {
	fmt.Println("\n>>> 3. 异步请求示例 - SendAsyncWithFunc (回调函数)")

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

	result := request.SendAsyncWithFunc(serverUuid, func(response *core.TykeResponse) {
		printAsyncResponse(response, "SendAsyncWithFunc 回调")
	})

	if result.HasValue() {
		fmt.Println("异步请求已发送，等待回调执行...")
	} else {
		fmt.Printf("异步请求失败: %s\n", result.Err)
	}
}

func demoSendAsyncWithFuture() {
	fmt.Println("\n>>> 4. 异步请求示例 - SendAsyncWithFuture (Future/Promise)")

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

	controller.RegisterController(controllers.NewExampleResponseController())

	result := framework.Start(clientListenerUuid)
	if !result.HasValue() {
		fmt.Printf("客户端启动失败: %s\n", result.Err)
		os.Exit(1)
	}

	fmt.Printf("客户端监听已启动，UUID: %s\n\n", clientListenerUuid)

	time.Sleep(500 * time.Millisecond)

	//demoSyncRequest()

	time.Sleep(200 * time.Millisecond)

	//demoSendAsync()

	time.Sleep(200 * time.Millisecond)

	//demoSendAsyncWithFunc()

	time.Sleep(200 * time.Millisecond)

	demoSendAsyncWithFuture()

	fmt.Println("\n等待异步响应处理完成...")
	time.Sleep(3 * time.Second)

	fmt.Println("\n所有示例执行完毕")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}
