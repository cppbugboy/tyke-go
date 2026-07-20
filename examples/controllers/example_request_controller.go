// Package controllers 为 Tyke 演示提供示例请求和响应路由处理器。
//
// 本文件注册用户服务和数据服务的请求处理器，并演示
// 同步和异步请求处理模式。
package controllers

import (
	"encoding/json"
	"fmt"
	"time"

	"tyke-go/common"
	"tyke-go/core"
)

// init 在包被导入时自动注册请求路由。
func init() {
	ExampleRequestRegisterMethod()
}

// ExampleRequestRegisterMethod 在全局 RequestRouter 上注册请求路由处理器。
func ExampleRequestRegisterMethod() {
	fmt.Println("注册请求路由处理器...")

	router := core.GetRequestRouter()
	root := router.GetRoot()

	root.AddSubGroup("/api/user").AddRouteHandler("/login", HandleUserLogin)
	root.AddSubGroup("/api/user").AddRouteHandler("/logout", HandleUserLogout)
	root.AddSubGroup("/api/data").AddRouteHandler("/query", HandleDataQuery)
	root.AddSubGroup("/api/data").AddRouteHandler("/update", HandleDataUpdate)
	root.AddSubGroup("/api/async").AddRouteHandler("/process", HandleAsyncProcess)

	fmt.Println("✓ 请求路由处理器注册完成")
}

// logRequest 将格式化的请求摘要打印到标准输出。
func logRequest(request *core.Request, handlerName string) {
	now := time.Now()
	fmt.Printf("\n========================================\n")
	fmt.Printf("[%s] 请求处理器: %s\n", now.Format("2006-01-02 15:04:05"), handlerName)
	fmt.Printf("========================================\n")
	fmt.Printf("消息UUID: %s\n", request.GetMsgUUID())
	fmt.Printf("模块: %s\n", request.GetModule())
	fmt.Printf("路由: %s\n", request.GetRoute())

	contentType, content := request.GetContent()
	if contentType == "json" && len(content) > 0 {
		var jsonData interface{}
		if err := json.Unmarshal(content, &jsonData); err == nil {
			prettyJSON, _ := json.MarshalIndent(jsonData, "", "  ")
			fmt.Printf("请求内容: %s\n", string(prettyJSON))
		}
	}
}

// validateRequest 验证请求体是否为合法的 JSON 且包含所有必填字段。
// 验证失败时在响应上设置适当的错误状态码和原因。
func validateRequest(request *core.Request, response *core.Response, requiredFields []string) bool {
	contentType, content := request.GetContent()

	if contentType != "json" {
		response.SetResult(int(common.StatusContentError), "Content type must be JSON")
		return false
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(content, &jsonData); err != nil {
		response.SetResult(int(common.StatusContentError), "Invalid JSON format")
		return false
	}

	for _, field := range requiredFields {
		if _, exists := jsonData[field]; !exists {
			response.SetResult(int(common.StatusContentError), "Missing required field: "+field)
			return false
		}
	}

	return true
}

// HandleUserLogin 处理用户登录请求。验证凭据并在成功时返回认证令牌。
func HandleUserLogin(request *core.Request, response *core.Response) {
	logRequest(request, "HandleUserLogin")

	if !validateRequest(request, response, []string{"username", "password"}) {
		logResponse(response, "HandleUserLogin")
		return
	}

	_, content := request.GetContent()
	var requestData map[string]interface{}
	json.Unmarshal(content, &requestData)

	username, _ := requestData["username"].(string)
	password, _ := requestData["password"].(string)

	if username == "test_user" && password == "test_password" {
		responseData := map[string]interface{}{
			"success":    true,
			"user_id":    "user_12345",
			"token":      "<GENERATED_TOKEN_PLACEHOLDER>",
			"expires_in": 3600,
		}

		responseBytes, _ := json.Marshal(responseData)
		response.SetContent(common.ContentTypeJson, responseBytes)
		response.SetResult(int(common.StatusSuccess), "OK")
	} else {
		response.SetResult(int(common.StatusContentError), "Invalid username or password")
	}

	response.SetModule(request.GetModule())
	response.SetRoute(request.GetRoute())
	response.SetMsgUUID(request.GetMsgUUID())

	logResponse(response, "HandleUserLogin")
}

// HandleUserLogout 处理用户登出请求。
func HandleUserLogout(request *core.Request, response *core.Response) {
	logRequest(request, "HandleUserLogout")

	responseData := map[string]interface{}{
		"success": true,
		"message": "User logged out successfully",
	}

	responseBytes, _ := json.Marshal(responseData)
	response.SetContent(common.ContentTypeJson, responseBytes)
	response.SetResult(int(common.StatusSuccess), "OK")
	response.SetModule(request.GetModule())
	response.SetRoute(request.GetRoute())
	response.SetMsgUUID(request.GetMsgUUID())

	logResponse(response, "HandleUserLogout")
}

// HandleDataQuery 处理数据查询请求，返回模拟数据集。
func HandleDataQuery(request *core.Request, response *core.Response) {
	logRequest(request, "HandleDataQuery")

	responseData := map[string]interface{}{
		"success": true,
		"total":   100,
		"data": []map[string]interface{}{
			{"id": 1, "name": "Item 1", "status": "active"},
			{"id": 2, "name": "Item 2", "status": "inactive"},
			{"id": 3, "name": "Item 3", "status": "active"},
		},
	}

	responseBytes, _ := json.Marshal(responseData)
	response.SetContent(common.ContentTypeJson, responseBytes)
	response.SetResult(int(common.StatusSuccess), "OK")
	response.SetModule(request.GetModule())
	response.SetRoute(request.GetRoute())
	response.SetMsgUUID(request.GetMsgUUID())

	logResponse(response, "HandleDataQuery")
}

// HandleDataUpdate 处理数据更新请求，在处理前验证必填字段。
func HandleDataUpdate(request *core.Request, response *core.Response) {
	logRequest(request, "HandleDataUpdate")

	if !validateRequest(request, response, []string{"id", "data"}) {
		logResponse(response, "HandleDataUpdate")
		return
	}

	responseData := map[string]interface{}{
		"success":    true,
		"message":    "Data updated successfully",
		"updated_at": time.Now().UnixMilli(),
	}

	responseBytes, _ := json.Marshal(responseData)
	response.SetContent(common.ContentTypeJson, responseBytes)
	response.SetResult(int(common.StatusSuccess), "OK")
	response.SetModule(request.GetModule())
	response.SetRoute(request.GetRoute())
	response.SetMsgUUID(request.GetMsgUUID())

	logResponse(response, "HandleDataUpdate")
}

// HandleAsyncProcess 处理异步处理请求。创建模拟任务并回显异步 UUID。
func HandleAsyncProcess(request *core.Request, response *core.Response) {
	logRequest(request, "HandleAsyncProcess")

	responseData := map[string]interface{}{
		"success":    true,
		"task_id":    fmt.Sprintf("task_%d", time.Now().UnixMilli()),
		"status":     "processing",
		"async_uuid": request.GetAsyncUUID(),
	}

	responseBytes, _ := json.Marshal(responseData)
	response.SetContent(common.ContentTypeJson, responseBytes)
	response.SetResult(int(common.StatusSuccess), "Accepted")
	response.SetModule(request.GetModule())
	response.SetRoute(request.GetRoute())
	response.SetMsgUUID(request.GetMsgUUID())
	response.SetAsyncUUID(request.GetAsyncUUID())

	logResponse(response, "HandleAsyncProcess")
}
