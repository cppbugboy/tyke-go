// Package main 演示Tyke客户端同步请求示例
//
// 同步请求的工作流程：
// 1. 从对象池获取请求和响应对象
// 2. 设置请求参数（模块、路由、内容）
// 3. 调用Send方法发送请求并等待响应
// 4. 处理响应结果
package main

import (
	"fmt"

	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/core"
)

// ServerUUID 服务端监听的UUID
// 客户端需要使用此UUID连接到服务端
const ServerUUID = "39649d81-81c5-4f6e-b6a9-e768b55063be"

// SendSyncRequest 发送同步请求的示例函数
//
// 同步请求会阻塞当前goroutine，直到收到服务端的响应或超时。
func SendSyncRequest() {
	fmt.Println("========== 同步请求示例 ==========")

	// 从对象池获取请求和响应对象
	// 使用defer确保对象归还到对象池
	response := core.AcquireResponse()
	defer core.ReleaseResponse(response)
	request := core.AcquireRequest()
	defer core.ReleaseRequest(request)

	// 设置请求内容
	requestContent := "hello from sync client"
	request.SetModule("test").
		SetRoute("/test/hello").
		SetContent(common.ContentTypeText, []byte(requestContent))

	fmt.Printf("发送同步请求: route=%s, content=%s\n",
		request.GetRoute(), requestContent)

	// 发送同步请求
	// Send方法会阻塞当前goroutine，直到收到响应或发生错误
	result := request.Send(ServerUUID, response)
	if !result.HasValue() {
		fmt.Printf("同步请求失败: %s\n", result.Err)
		return
	}

	// 获取响应内容
	contentType, content := response.GetContent()

	// 获取响应状态
	status, reason := response.GetResult()

	fmt.Println("同步响应收到:")
	fmt.Printf("  - 状态码: %d\n", status)
	fmt.Printf("  - 原因: %s\n", reason)
	fmt.Printf("  - 内容类型: %s\n", contentType)
	fmt.Printf("  - 内容: %s\n", string(content))
	fmt.Printf("  - 消息UUID: %s\n", response.GetMsgUuid())
}

func main() {
	fmt.Println("Tyke 同步请求客户端示例")
	fmt.Println("====================================")

	SendSyncRequest()

	fmt.Println("====================================")
	fmt.Println("同步请求示例完成")
}
