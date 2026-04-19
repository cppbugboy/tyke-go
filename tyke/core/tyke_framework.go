package core

import (
	"fmt"
	"sync"

	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/component"
	"github.com/tyke/tyke/tyke/ipc"
)

type TykeFramework struct {
	threadPoolCount uint32
	logPath         string
	logLevel        string
	fileSizeMb      uint32
	fileCount       uint32
	ipcServer       *ipc.IpcServer
}

var (
	frameworkInstance *TykeFramework
	frameworkOnce     sync.Once
)

func App() *TykeFramework {
	frameworkOnce.Do(func() {
		frameworkInstance = &TykeFramework{
			threadPoolCount: 4,
			logLevel:        "info",
			fileSizeMb:      1024,
			fileCount:       5,
			ipcServer:       ipc.NewIpcServer(),
		}
	})
	return frameworkInstance
}

func (f *TykeFramework) SetThreadPoolCount(count uint32) *TykeFramework {
	f.threadPoolCount = count
	return f
}

func (f *TykeFramework) SetLogConfig(logPath string, logLevel string, fileSizeMb uint32, fileCount uint32) *TykeFramework {
	f.logPath = logPath
	f.logLevel = logLevel
	f.fileSizeMb = fileSizeMb
	f.fileCount = fileCount
	logInstance := GetTykeLogInstance()
	if result := logInstance.Init(logPath, logLevel, fileSizeMb, fileCount); !result.HasValue() {
		fmt.Printf("Tyke framework initialization failed: %s\n", logPath)
	}
	return f
}

func (f *TykeFramework) Start(listenUuid string) common.BoolResult {
	logInstance := GetTykeLogInstance()
	if !logInstance.IsInitialized() {
		logPath := f.logPath
		if logPath == "" {
			logPath = common.GetTempDir() + "/tyke.log"
		}
		if result := logInstance.Init(logPath, f.logLevel, f.fileSizeMb, f.fileCount); !result.HasValue() {
			fmt.Printf("Tyke framework start failed: %s\n", logPath)
			return common.ErrBool("log init failed")
		}
	}

	common.LogInfo("Tyke framework starting", "listen_uuid", listenUuid)

	tp := component.GetThreadPoolInstance()
	tp.Init(int(f.threadPoolCount))
	common.LogDebug("Thread pool initialized", "threads", f.threadPoolCount)

	if f.ipcServer == nil {
		common.LogError("IPC server is not initialized")
		return common.ErrBool("ipc server is not initialized")
	}

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

func (f *TykeFramework) GetRequestRouter() *RouterBase[RequestFilter, RequestHandlerFunc] {
	return GetRequestRouterInstance()
}

func (f *TykeFramework) GetResponseRouter() *RouterBase[ResponseFilter, ResponseHandlerFunc] {
	return GetResponseRouterInstance()
}
