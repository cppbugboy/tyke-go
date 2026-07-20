// Package controllers 提供示例响应路由处理器。
//
// 本文件注册响应处理器，用于处理 ResponseRouter 在异步请求完成时分发的异步响应。
package controllers

import (
	"encoding/json"
	"fmt"
	"time"

	"tyke-go/core"
)

// init 在包被导入时自动注册响应路由。
func init() {
	ExampleResponseRegisterMethod()
}

// ExampleResponseRegisterMethod 在全局 ResponseRouter 上注册响应路由处理器。
func ExampleResponseRegisterMethod() {
	fmt.Println("Registering response route handlers...")

	router := core.GetResponseRouter()
	root := router.GetRoot()

	root.AddSubGroup("/api/async").AddRouteHandler("/callback", HandleAsyncCallback)
	root.AddSubGroup("/api/async").AddRouteHandler("/process", HandleAsyncCallback)
	root.AddSubGroup("/api/async").AddRouteHandler("/notification", HandleAsyncNotification)

	fmt.Println("Response route handlers registered")
}

// logResponse 将格式化的响应摘要打印到标准输出。
func logResponse(response *core.Response, handlerName string) {
	now := time.Now()
	status, reason := response.GetResult()

	fmt.Printf("\n========================================\n")
	fmt.Printf("[%s] 响应处理器: %s\n", now.Format("2006-01-02 15:04:05"), handlerName)
	fmt.Printf("========================================\n")
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
			fmt.Printf("响应内容: %s\n", string(prettyJSON))
		}
	}
	fmt.Printf("========================================\n\n")
}

// HandleAsyncCallback 处理异步回调响应。
func HandleAsyncCallback(response *core.Response) {
	logResponse(response, "HandleAsyncCallback")

	fmt.Println("Processing async callback response...")
	fmt.Println("Executing business logic...")
	fmt.Println("Async callback processing complete")
}

// HandleAsyncNotification 处理异步通知响应。
func HandleAsyncNotification(response *core.Response) {
	logResponse(response, "HandleAsyncNotification")

	fmt.Println("Processing async notification response...")
	fmt.Println("Updating local state...")
	fmt.Println("Async notification processing complete")
}
