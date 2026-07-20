// Package core 实现 Tyke 框架内核：生命周期管理、
// IPC 消息路由、请求/响应处理和元数据序列化。
//
// 本文件定义了 Framework 单例，负责引导 IPC 服务器、
// 线程池、时间轮、日志和存根清理任务。
package core

import (
	"runtime"
	"sync"

	"tyke-go/common"
	"tyke-go/component"
	"tyke-go/ipc"
)

// Framework 是 Tyke 框架的主入口点，管理生命周期和配置。
// 它是一个单例，通过 App() 访问。
type Framework struct {
	threadPoolCount uint32
	logPath         string
	logLevel        string
	fileSizeMb      uint32
	fileCount       uint32
	ipcServer       *ipc.IPCServer
	cleanupTimerId  component.TimerId
}

var (
	frameworkInstance *Framework
	frameworkOnce     sync.Once
)

// App 返回单例 Framework 实例，首次调用时创建。
func App() *Framework {
	frameworkOnce.Do(func() {
		frameworkInstance = &Framework{
			threadPoolCount: 4,
			logLevel:        "info",
			fileSizeMb:      1024,
			fileCount:       5,
			ipcServer:       ipc.NewIPCServer(),
		}
	})
	return frameworkInstance
}

// SetThreadPoolCount 设置协程池中工作协程的数量。
// 返回 Framework 以支持链式调用。
func (f *Framework) SetThreadPoolCount(count uint32) *Framework {
	f.threadPoolCount = count
	return f
}

// SetLogConfig 配置日志系统。如果日志实例尚未初始化，
// 则立即初始化它。返回 Framework 以支持链式调用。
func (f *Framework) SetLogConfig(logPath string, logLevel string, fileSizeMb uint32, fileCount uint32) *Framework {
	f.logPath = logPath
	f.logLevel = logLevel
	f.fileSizeMb = fileSizeMb
	f.fileCount = fileCount
	logInstance := GetTykeLogInstance()
	if result := logInstance.Init(logPath, logLevel, fileSizeMb, fileCount); !result.HasValue() {
		common.LogError("Tyke framework initialization failed", "log_path", logPath)
	}
	return f
}

// Start 引导框架：验证 UUID、初始化日志、
// 启动协程池和时间轮、注册存根清理任务、
// 并启动 IPC 服务器监听指定的 UUID。
func (f *Framework) Start(listenUuid string) common.BoolResult {

	if !common.IsValidUUID(listenUuid) {
		return common.ErrBool("uuid is invalid")
	}

	// 如果尚未设置，则初始化日志。
	logInstance := GetTykeLogInstance()
	if !logInstance.IsInitialized() {
		logPath := f.logPath
		if logPath == "" {
			logPath = common.GetTempDir() + "/tyke.log"
		}
		if result := logInstance.Init(logPath, f.logLevel, f.fileSizeMb, f.fileCount); !result.HasValue() {
			common.LogError("Tyke framework start failed", "log_path", logPath)
			return common.ErrBool("log init failed")
		}
	}

	common.LogInfo("Tyke framework starting", "listen_uuid", listenUuid)

	threadPoolCount := f.threadPoolCount
	if threadPoolCount == 0 {
		threadPoolCount = uint32(runtime.NumCPU())
	}
	if threadPoolCount == 0 {
		threadPoolCount = 4
	}

	// 初始化全局协程池。
	tp := component.GetCoroutinePoolInstance()
	tp.Init(int(threadPoolCount))
	common.LogDebug("Thread pool initialized", "threads", threadPoolCount)

	// 初始化时间轮并注册清理回调。
	tw := component.GetTimingWheel()
	tw.SetExpiredCallbacks(RequestStubCleanupExpiredFunc, RequestStubCleanupExpiredFuture)
	tw.Init()
	common.LogDebug("TimingWheel initialized")

	// 定期清理过期的请求存根。
	cleanupIntervalMs := uint32(common.DefaultStubTimeoutMs / 4)
	f.cleanupTimerId = tw.AddRepeatedTask(cleanupIntervalMs, cleanupIntervalMs, func() {
		RequestStubCleanupExpiredFuncs()
		RequestStubCleanupExpiredFutures()
	})
	common.LogDebug("Stub cleanup task registered", "interval_ms", cleanupIntervalMs, "timer_id", f.cleanupTimerId)

	if f.ipcServer == nil {
		common.LogError("IPC server is not initialized")
		return common.ErrBool("ipc server is not initialized")
	}

	// 启动 IPC 服务器。回调函数将 IPC 发送回调适配为 DataCallback 使用。
	startResult := f.ipcServer.Start(listenUuid, func(clientId ipc.ClientId, data []byte, sendCb ipc.ServerSendDataCallback) *uint32 {
		handler := func(cid ipc.ClientId, d []byte) bool {
			return sendCb(cid, d)
		}
		return DataCallback(clientId, data, handler)
	})
	if !startResult.HasValue() {
		common.LogError("IPC server start failed", "error", startResult.Err)
		return common.ErrBool("ipc server start failed: " + startResult.Err)
	}

	common.LogInfo("Tyke framework started successfully")
	return common.OkBool(true)
}

// SetModuleName 设置全局模块名称，作为出站请求中的默认模块名称使用。
func SetModuleName(moduleName string) {
	common.ModuleName = moduleName
}

// GetRequestRouter 返回全局请求路由器。
func (f *Framework) GetRequestRouter() *RouterBase[RequestFilter, RequestHandlerFunc] {
	return GetRequestRouter()
}

// GetResponseRouter 返回全局响应路由器。
func (f *Framework) GetResponseRouter() *RouterBase[ResponseFilter, ResponseHandlerFunc] {
	return GetResponseRouter()
}

// Shutdown 优雅地停止框架：取消清理定时器、
// 停止 IPC 服务器、时间轮、协程池和日志。
func (f *Framework) Shutdown() {
	common.LogInfo("Tyke framework shutting down")

	if f.cleanupTimerId != component.InvalidTimerId {
		component.GetTimingWheel().CancelTask(f.cleanupTimerId)
		f.cleanupTimerId = component.InvalidTimerId
	}

	if f.ipcServer != nil {
		f.ipcServer.Stop()
	}

	component.GetTimingWheel().Stop()
	component.GetCoroutinePoolInstance().Stop(true)
	GetTykeLogInstance().Stop()
}
