// Package main 演示Tyke客户端异步请求示例
//
// 演示三种异步请求方式：
// 1. SendAsync - 通过ResponseRouter路由分发处理响应
// 2. SendAsyncWithFunc - 通过回调函数处理响应
// 3. SendAsyncWithFuture - 通过Future/Promise机制等待响应
//
// 异步请求的核心机制：
// - 客户端启动独立的IPC Server监听异步响应
// - 发送请求时通过SetAsyncUuid设置监听UUID
// - 服务端根据async_uuid将响应发送到客户端监听服务器
package main

import (
	"fmt"
	"sync"

	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/core"
	"github.com/tyke/tyke/tyke/ipc"
)

// ServerUUID 服务端监听的UUID
const ServerUUID = "39649d81-81c5-4f6e-b6a9-e768b55063be"

// TotalAsyncRequests 异步请求总数
const TotalAsyncRequests = 3

// 全局等待组，用于等待所有异步响应
var wg sync.WaitGroup

// OnAsyncResponse 响应处理函数（用于SendAsync方式）
//
// 此函数通过ResponseRouter注册，当收到kResponseAsync类型的响应时被调用。
func OnAsyncResponse(response *core.TykeResponse) {
	contentType, content := response.GetContent()
	status, reason := response.GetResult()

	fmt.Println("[SendAsync响应] 收到异步响应:")
	fmt.Printf("  - 路由: %s\n", response.GetRoute())
	fmt.Printf("  - 状态码: %d\n", status)
	fmt.Printf("  - 原因: %s\n", reason)
	fmt.Printf("  - 内容类型: %s\n", contentType)
	fmt.Printf("  - 内容: %s\n", string(content))

	wg.Done()
}

// RegisterResponseHandlers 注册响应路由处理器
//
// 对于SendAsync方式，需要在客户端注册响应路由处理器。
// 当服务端返回响应时，ResponseRouter会根据路由路径找到对应的处理器。
func RegisterResponseHandlers() {
	// 获取响应路由器的根分组
	rootGroup := core.GetResponseRouterInstance().GetRoot()

	// 注册 /test/hello 路由的响应处理器
	rootGroup.AddRouteHandler("/test/hello", OnAsyncResponse)

	fmt.Println("响应路由处理器已注册")
}

// SendAsyncRequest 演示SendAsync方式发送异步请求
//
// SendAsync方式的特点：
// - 发送请求后立即返回，不阻塞当前goroutine
// - 响应通过ResponseRouter路由分发到对应的处理器
// - 适合需要统一响应处理逻辑的场景
func SendAsyncRequest(listenUuid string) {
	fmt.Println("\n========== SendAsync 方式 ==========")

	request := core.AcquireRequest()
	defer core.ReleaseRequest(request)

	requestContent := "hello from SendAsync"
	request.SetModule("test").
		SetRoute("/test/hello").
		SetContent(common.ContentTypeText, []byte(requestContent))

	// 通过SetAsyncUuid设置async_uuid
	// 服务端会将响应发送到此UUID对应的IPC服务器
	request.SetAsyncUuid(listenUuid)

	fmt.Printf("发送SendAsync请求: route=%s, async_uuid=%s\n",
		request.GetRoute(), listenUuid)

	// SendAsync第二个参数是recv_uuid，用于指定响应接收地址
	result := request.SendAsync(ServerUUID)
	if !result.HasValue() {
		fmt.Printf("SendAsync请求失败: %s\n", result.Err)
		wg.Done()
		return
	}

	fmt.Println("SendAsync请求已发送，等待响应...")
}

// SendAsyncWithFuncRequest 演示SendAsyncWithFunc方式发送异步请求
//
// SendAsyncWithFunc方式的特点：
// - 发送请求时注册回调函数
// - 响应到达时自动调用回调函数
// - 适合简单场景，直接处理响应
func SendAsyncWithFuncRequest(listenUuid string) {
	fmt.Println("\n========== SendAsyncWithFunc 方式 ==========")

	request := core.AcquireRequest()
	defer core.ReleaseRequest(request)

	requestContent := "hello from SendAsyncWithFunc"
	request.SetModule("test").
		SetRoute("/test/hello").
		SetContent(common.ContentTypeText, []byte(requestContent))

	// 通过SetAsyncUuid设置async_uuid
	request.SetAsyncUuid(listenUuid)

	fmt.Printf("发送SendAsyncWithFunc请求: route=%s, async_uuid=%s\n",
		request.GetRoute(), listenUuid)

	// 发送请求并注册回调函数
	// 回调函数会在响应到达时被调用
	result := request.SendAsyncWithFunc(ServerUUID, func(response *core.TykeResponse) {
		contentType, content := response.GetContent()
		status, reason := response.GetResult()

		fmt.Println("[SendAsyncWithFunc响应] 收到异步响应:")
		fmt.Printf("  - 路由: %s\n", response.GetRoute())
		fmt.Printf("  - 状态码: %d\n", status)
		fmt.Printf("  - 原因: %s\n", reason)
		fmt.Printf("  - 内容类型: %s\n", contentType)
		fmt.Printf("  - 内容: %s\n", string(content))

		wg.Done()
	})

	if !result.HasValue() {
		fmt.Printf("SendAsyncWithFunc请求失败: %s\n", result.Err)
		wg.Done()
		return
	}

	fmt.Println("SendAsyncWithFunc请求已发送，等待回调...")
}

// SendAsyncWithFutureRequest 演示SendAsyncWithFuture方式发送异步请求
//
// SendAsyncWithFuture方式的特点：
// - 返回ResponseFuture对象
// - 可以在需要时调用GetResponse()阻塞等待响应
// - 适合需要同步等待异步结果的场景
func SendAsyncWithFutureRequest(listenUuid string) {
	fmt.Println("\n========== SendAsyncWithFuture 方式 ==========")

	request := core.AcquireRequest()
	defer core.ReleaseRequest(request)

	requestContent := "hello from SendAsyncWithFuture"
	request.SetModule("test").
		SetRoute("/test/hello").
		SetContent(common.ContentTypeText, []byte(requestContent))

	// 通过SetAsyncUuid设置async_uuid
	request.SetAsyncUuid(listenUuid)

	fmt.Printf("发送SendAsyncWithFuture请求: route=%s, async_uuid=%s\n",
		request.GetRoute(), listenUuid)

	// 发送请求并获取Future对象
	future, err := request.SendAsyncWithFuture(ServerUUID)
	if err != nil {
		fmt.Printf("SendAsyncWithFuture请求失败: %s\n", err.Error())
		wg.Done()
		return
	}

	fmt.Println("SendAsyncWithFuture请求已发送，在goroutine中等待Future...")

	// 在单独的goroutine中等待Future结果
	go func() {
		// GetResponse()会阻塞直到收到响应或超时
		response := future.GetResponse()

		contentType, content := response.GetContent()
		status, reason := response.GetResult()

		fmt.Println("[SendAsyncWithFuture响应] 收到异步响应:")
		fmt.Printf("  - 路由: %s\n", response.GetRoute())
		fmt.Printf("  - 状态码: %d\n", status)
		fmt.Printf("  - 原因: %s\n", reason)
		fmt.Printf("  - 内容类型: %s\n", contentType)
		fmt.Printf("  - 内容: %s\n", string(content))

		wg.Done()
	}()
}

// ClientDataCallback 客户端数据回调函数
//
// 当监听IPC Server收到数据时，此回调函数被调用。
// 这里使用框架提供的DataCallback处理数据，它会根据消息类型
// 自动分发到相应的处理器（ResponseRouter、RequestStub等）。
func ClientDataCallback(clientId ipc.ClientId, dataVec []byte, sendCb ipc.ServerSendDataCallback) *uint32 {
	handler := func(cid ipc.ClientId, d []byte) bool {
		return sendCb(cid, d)
	}
	return core.DataCallback(clientId, dataVec, handler)
}

func main() {
	fmt.Println("Tyke 异步请求客户端示例")
	fmt.Println("====================================")

	// 生成客户端监听服务器的UUID
	// 服务端会将异步响应发送到此UUID
	listenUuid := common.GenerateUUID()
	fmt.Printf("客户端监听UUID: %s\n", listenUuid)

	// 注册响应路由处理器（用于SendAsync方式）
	RegisterResponseHandlers()

	// 创建并启动客户端监听IPC Server
	// 这是异步请求的关键：客户端需要有自己的IPC Server来接收异步响应
	listenServer := ipc.NewIpcServer()
	startResult := listenServer.Start(listenUuid, ClientDataCallback)
	if !startResult.HasValue() {
		fmt.Printf("启动监听服务器失败: %s\n", startResult.Err)
		return
	}

	fmt.Println("客户端监听服务器已启动")

	// 设置等待组
	wg.Add(TotalAsyncRequests)

	// 发送三种异步请求
	SendAsyncRequest(listenUuid)
	SendAsyncWithFuncRequest(listenUuid)
	SendAsyncWithFutureRequest(listenUuid)

	fmt.Println("\n等待所有异步响应...")

	// 等待所有响应到达
	wg.Wait()

	fmt.Println("\n所有异步响应已收到")

	// 停止监听服务器
	listenServer.Stop()
	fmt.Println("客户端监听服务器已停止")

	fmt.Println("====================================")
	fmt.Println("异步请求示例完成")
}
