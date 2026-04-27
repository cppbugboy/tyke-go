package controllers

import (
	"encoding/json"
	"fmt"
	"time"

	"tyke-go/common"
	"tyke-go/core"
)

func init() {
	ExampleRequestRegisterMethod()
}

func ExampleRequestRegisterMethod() {
	fmt.Println("注册请求路由处理器...")

	router := core.GetRequestRouter()
	root := router.GetRoot()

	root.Group("/api/user").Route("/login", HandleUserLogin)
	root.Group("/api/user").Route("/logout", HandleUserLogout)
	root.Group("/api/data").Route("/query", HandleDataQuery)
	root.Group("/api/data").Route("/update", HandleDataUpdate)
	root.Group("/api/async").Route("/process", HandleAsyncProcess)

	fmt.Println("✓ 请求路由处理器注册完成")
}

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
