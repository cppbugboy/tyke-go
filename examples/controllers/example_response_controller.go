package controllers

import (
	"encoding/json"
	"fmt"
	"time"

	"tyke-go/core"
)

func init() {
	ExampleResponseRegisterMethod()
}

func ExampleResponseRegisterMethod() {
	fmt.Println("注册响应路由处理器...")

	router := core.GetResponseRouter()
	root := router.GetRoot()

	root.Group("/api/async").Route("/callback", HandleAsyncCallback)
	root.Group("/api/async").Route("/process", HandleAsyncCallback)
	root.Group("/api/async").Route("/notification", HandleAsyncNotification)

	fmt.Println("✓ 响应路由处理器注册完成")
}

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

func HandleAsyncCallback(response *core.Response) {
	logResponse(response, "HandleAsyncCallback")

	fmt.Println("处理异步回调响应...")
	fmt.Println("执行业务逻辑...")
	fmt.Println("✓ 异步回调处理完成")
}

func HandleAsyncNotification(response *core.Response) {
	logResponse(response, "HandleAsyncNotification")

	fmt.Println("处理异步通知响应...")
	fmt.Println("更新本地状态...")
	fmt.Println("✓ 异步通知处理完成")
}
