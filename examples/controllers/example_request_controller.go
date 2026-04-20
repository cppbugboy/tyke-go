package controllers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/controller"
	"github.com/tyke/tyke/tyke/core"
)

type ExampleRequestController struct{}

func NewExampleRequestController() controller.ControllerBase {
	return &ExampleRequestController{}
}

func (c *ExampleRequestController) RegisterMethod() {
	fmt.Println("注册请求路由处理器...")

	router := core.GetRequestRouter()
	root := router.GetRoot()

	root.Group("/api/user").Route("/login", c.HandleUserLogin)
	root.Group("/api/user").Route("/logout", c.HandleUserLogout)
	root.Group("/api/data").Route("/query", c.HandleDataQuery)
	root.Group("/api/data").Route("/update", c.HandleDataUpdate)
	root.Group("/api/async").Route("/process", c.HandleAsyncProcess)

	fmt.Println("✓ 请求路由处理器注册完成")
}

func (c *ExampleRequestController) logRequest(request *core.TykeRequest, handlerName string) {
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

func (c *ExampleRequestController) logResponse(response *core.TykeResponse, handlerName string) {
	now := time.Now()
	status, reason := response.GetResult()

	fmt.Printf("\n[%s] 响应已构建: %s\n", now.Format("2006-01-02 15:04:05"), handlerName)
	fmt.Printf("状态码: %d\n", status)
	fmt.Printf("原因: %s\n", reason)
	fmt.Printf("========================================\n\n")
}

func (c *ExampleRequestController) validateRequest(request *core.TykeRequest, response *core.TykeResponse, requiredFields []string) bool {
	contentType, content := request.GetContent()

	if contentType != "json" {
		response.SetResult(400, "Content type must be JSON")
		return false
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(content, &jsonData); err != nil {
		response.SetResult(400, "Invalid JSON format")
		return false
	}

	for _, field := range requiredFields {
		if _, exists := jsonData[field]; !exists {
			response.SetResult(400, "Missing required field: "+field)
			return false
		}
	}

	return true
}

func (c *ExampleRequestController) HandleUserLogin(request *core.TykeRequest, response *core.TykeResponse) {
	c.logRequest(request, "HandleUserLogin")

	if !c.validateRequest(request, response, []string{"username", "password"}) {
		c.logResponse(response, "HandleUserLogin")
		return
	}

	_, content := request.GetContent()
	var requestData map[string]interface{}
	json.Unmarshal(content, &requestData)

	username := requestData["username"].(string)
	password := requestData["password"].(string)

	if username == "test_user" && password == "test_password" {
		responseData := map[string]interface{}{
			"success":    true,
			"user_id":    "user_12345",
			"token":      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			"expires_in": 3600,
		}

		responseBytes, _ := json.Marshal(responseData)
		response.SetContent(common.ContentTypeJson, responseBytes)
		response.SetResult(200, "OK")
	} else {
		response.SetResult(401, "Invalid username or password")
	}

	response.SetModule(request.GetModule())
	response.SetRoute(request.GetRoute())
	response.SetMsgUUID(request.GetMsgUUID())

	c.logResponse(response, "HandleUserLogin")
}

func (c *ExampleRequestController) HandleUserLogout(request *core.TykeRequest, response *core.TykeResponse) {
	c.logRequest(request, "HandleUserLogout")

	responseData := map[string]interface{}{
		"success": true,
		"message": "User logged out successfully",
	}

	responseBytes, _ := json.Marshal(responseData)
	response.SetContent(common.ContentTypeJson, responseBytes)
	response.SetResult(200, "OK")
	response.SetModule(request.GetModule())
	response.SetRoute(request.GetRoute())
	response.SetMsgUUID(request.GetMsgUUID())

	c.logResponse(response, "HandleUserLogout")
}

func (c *ExampleRequestController) HandleDataQuery(request *core.TykeRequest, response *core.TykeResponse) {
	c.logRequest(request, "HandleDataQuery")

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
	response.SetResult(200, "OK")
	response.SetModule(request.GetModule())
	response.SetRoute(request.GetRoute())
	response.SetMsgUUID(request.GetMsgUUID())

	c.logResponse(response, "HandleDataQuery")
}

func (c *ExampleRequestController) HandleDataUpdate(request *core.TykeRequest, response *core.TykeResponse) {
	c.logRequest(request, "HandleDataUpdate")

	if !c.validateRequest(request, response, []string{"id", "data"}) {
		c.logResponse(response, "HandleDataUpdate")
		return
	}

	responseData := map[string]interface{}{
		"success":    true,
		"message":    "Data updated successfully",
		"updated_at": time.Now().UnixMilli(),
	}

	responseBytes, _ := json.Marshal(responseData)
	response.SetContent(common.ContentTypeJson, responseBytes)
	response.SetResult(200, "OK")
	response.SetModule(request.GetModule())
	response.SetRoute(request.GetRoute())
	response.SetMsgUUID(request.GetMsgUUID())

	c.logResponse(response, "HandleDataUpdate")
}

func (c *ExampleRequestController) HandleAsyncProcess(request *core.TykeRequest, response *core.TykeResponse) {
	c.logRequest(request, "HandleAsyncProcess")

	responseData := map[string]interface{}{
		"success":    true,
		"task_id":    fmt.Sprintf("task_%d", time.Now().UnixMilli()),
		"status":     "processing",
		"async_uuid": request.GetAsyncUUID(),
	}

	responseBytes, _ := json.Marshal(responseData)
	response.SetContent(common.ContentTypeJson, responseBytes)
	response.SetResult(202, "Accepted")
	response.SetModule(request.GetModule())
	response.SetRoute(request.GetRoute())
	response.SetMsgUUID(request.GetMsgUUID())
	response.SetAsyncUUID(request.GetAsyncUUID())

	c.logResponse(response, "HandleAsyncProcess")
}
