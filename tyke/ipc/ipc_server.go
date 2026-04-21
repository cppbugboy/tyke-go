package ipc

import "github.com/tyke/tyke/tyke/common"

type IPCServer struct {
	impl Server
}

func NewIPCServer() *IPCServer {
	common.LogDebug("IPCServer constructed")
	return &IPCServer{impl: createServerImpl()}
}

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

func (s *IPCServer) Stop() {
	common.LogInfo("IPC server stopping")
	s.impl.Stop()
	common.LogInfo("IPC server stopped")
}

func (s *IPCServer) SendToClient(id ClientId, data []byte) common.BoolResult {
	common.LogDebug("SendToClient", "client_id", id, "data_size", len(data))
	result := s.impl.SendToClient(id, data)
	if !result.HasValue() {
		common.LogError("SendToClient failed", "client_id", id, "error", result.Err)
		return common.ErrBool("send to client failed: " + result.Err)
	}
	return common.OkBool(true)
}
