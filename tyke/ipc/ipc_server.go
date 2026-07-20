// Package ipc 提供进程间通信层。
//
// 本文件定义了 IPCServer，它包装一个平台特定的 Server 实现
// 并提供 Start/Stop/SendToClient 方法。
package ipc

import "tyke-go/common"

// IPCServer 是 Tyke IPC 服务器。它接受来自 IPC 客户端的连接，
// 将传入的数据分发到回调函数，并可以将数据发送回特定客户端。
type IPCServer struct {
	impl Server
}

// NewIPCServer 使用平台特定的 Server 实现创建一个新的 IPCServer。
func NewIPCServer() *IPCServer {
	common.LogDebug("IPCServer constructed")
	return &IPCServer{impl: createServerImpl()}
}

// Start 启动 IPC 服务器监听指定名称。回调函数在
// 从客户端收到每条完整消息时被调用。
func (s *IPCServer) Start(serverName string, callback ServerRecvDataCallback) common.BoolResult {
	common.LogInfo("IPC server starting", "server_name", serverName)
	result := s.impl.Start(serverName, callback)
	if !result.HasValue() {
		common.LogError("IPC server start failed", "error", result.Err)
		return common.ErrBool("server start failed: " + result.Err)
	}
	common.LogInfo("IPC server started successfully")
	return common.OkBool(true)
}

// Stop 优雅地关闭 IPC 服务器并关闭所有客户端连接。
func (s *IPCServer) Stop() {
	common.LogInfo("IPC server stopping")
	s.impl.Stop()
	common.LogInfo("IPC server stopped")
}

// SendToClient 向由给定 ClientId 标识的客户端发送数据。
func (s *IPCServer) SendToClient(id ClientId, data []byte) common.BoolResult {
	common.LogDebug("SendToClient", "client_id", id, "data_size", len(data))
	result := s.impl.SendToClient(id, data)
	if !result.HasValue() {
		common.LogError("SendToClient failed", "client_id", id, "error", result.Err)
		return common.ErrBool("send to client failed: " + result.Err)
	}
	return common.OkBool(true)
}
