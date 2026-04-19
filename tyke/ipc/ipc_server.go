package ipc

import "github.com/tyke/tyke/tyke/common"

type IpcServer struct {
	impl IServerImpl
}

func NewIpcServer() *IpcServer {
	common.LogDebug("IpcServer constructed")
	return &IpcServer{impl: createServerImpl()}
}

func (s *IpcServer) Start(serverName string, callback ServerRecvDataCallback) common.BoolResult {
	common.LogInfo("IpcServer starting", "server_name", serverName)
	result := s.impl.Start(serverName, callback)
	if !result.HasValue() {
		common.LogError("IpcServer start failed", "error", result.Err)
		return common.ErrBool("server start failed: " + result.Err)
	}
	common.LogInfo("IpcServer started successfully")
	return common.OkBool(true)
}

func (s *IpcServer) Stop() {
	common.LogInfo("IpcServer stopping")
	s.impl.Stop()
	common.LogInfo("IpcServer stopped")
}

func (s *IpcServer) SendToClient(id ClientId, data []byte) common.BoolResult {
	common.LogDebug("SendToClient", "client_id", id, "data_size", len(data))
	result := s.impl.SendToClient(id, data)
	if !result.HasValue() {
		common.LogError("SendToClient failed", "client_id", id, "error", result.Err)
		return common.ErrBool("send to client failed: " + result.Err)
	}
	return common.OkBool(true)
}
