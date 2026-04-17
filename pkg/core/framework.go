// Package core provides the core functionality of the Tyke IPC framework.
//
// This package contains the main framework, request/response handling,
// routing, dispatching, and logging functionality.
package core

import (
	"fmt"
	"sync"

	"github.com/tyke/tyke/pkg/common"
	"github.com/tyke/tyke/pkg/component"
	"github.com/tyke/tyke/pkg/ipc"
)

// RunControllersFunc is a function that registers all controllers.
// It should be set by the controller package during initialization.
var RunControllersFunc func()

// Framework is the main application framework that manages the IPC server,
// worker pool, and logging configuration.
//
// Framework follows the singleton pattern and should be accessed via App().
//
// Example:
//
//	app := App()
//	app.SetThreadPoolCount(8).
//	    SetLogConfig("/var/log/tyke.log", "info", 10, 5).
//	    Start("my-server-uuid")
//	defer app.Stop()
type Framework struct {
	threadPoolCount int
	logPath         string
	logLevel        string
	logFileSizeMB   uint32
	logFileCount    uint32
	ipcServer       *ipc.IpcServer
	started         bool
}

var (
	framework     *Framework
	frameworkOnce sync.Once
)

// App returns the singleton Framework instance.
//
// The framework is lazily initialized with default values:
//   - ThreadPoolCount: common.DefaultWorkerPoolSize (4)
//   - LogLevel: "info"
//   - LogFileSizeMB: 10
//   - LogFileCount: 5
//
// Returns:
//   - *Framework: The singleton framework instance
func App() *Framework {
	frameworkOnce.Do(func() {
		framework = &Framework{
			threadPoolCount: common.DefaultWorkerPoolSize,
			logLevel:       "info",
			logFileSizeMB:  10,
			logFileCount:   5,
		}
	})
	return framework
}

// SetThreadPoolCount sets the number of worker goroutines in the pool.
//
// Parameters:
//   - count: Number of workers (must be > 0)
//
// Returns:
//   - *Framework: The framework instance for method chaining
//
// Note: This method must be called before Start().
func (f *Framework) SetThreadPoolCount(count int) *Framework {
	f.threadPoolCount = count
	return f
}

// SetLogConfig configures the logging system.
//
// Parameters:
//   - logPath: Path to the log file (empty string for stdout)
//   - logLevel: Log level ("debug", "info", "warn", "error")
//   - fileSizeMB: Maximum size of each log file in MB
//   - fileCount: Number of log files to keep
//
// Returns:
//   - *Framework: The framework instance for method chaining
//
// Note: This method must be called before Start().
func (f *Framework) SetLogConfig(logPath, logLevel string, fileSizeMB, fileCount uint32) *Framework {
	f.logPath = logPath
	f.logLevel = logLevel
	f.logFileSizeMB = fileSizeMB
	f.logFileCount = fileCount
	return f
}

// Start initializes and starts the framework.
//
// This method:
//  1. Initializes the logging system (if logPath is set)
//  2. Initializes the worker pool
//  3. Registers all controllers
//  4. Starts the IPC server
//
// Parameters:
//   - listenUUID: UUID for the IPC server endpoint
//
// Returns:
//   - error: nil on success, or an error on failure
//
// Example:
//
//	err := App().Start("my-server-uuid")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (f *Framework) Start(listenUUID string) error {
	if f.started {
		return fmt.Errorf("framework already started")
	}

	if f.logPath != "" {
		if err := InitLog(f.logPath, f.logLevel, f.logFileSizeMB, f.logFileCount); err != nil {
			return fmt.Errorf("log init failed: %w", err)
		}
	}

	LogInfo("Tyke framework starting, listen_uuid=%s", listenUUID)

	component.InitWorkerPool(f.threadPoolCount)

	if RunControllersFunc != nil {
		RunControllersFunc()
	}

	f.ipcServer = ipc.NewIpcServer()
	dataHandler := NewDataHandler()

	callback := func(id ipc.ClientId, data []byte, sendCb ipc.ServerSendDataCallback) uint32 {
		responseData := dataHandler.OnRequestData(data, func(encoded []byte) bool {
			return sendCb(id, encoded)
		})
		if responseData != nil {
			sendCb(id, responseData)
		}
		return uint32(len(data))
	}

	if err := f.ipcServer.Start(listenUUID, callback); err != nil {
		return fmt.Errorf("IPC server start failed: %w", err)
	}

	f.started = true
	LogInfo("Tyke framework started successfully")
	return nil
}

// GetRequestRouter returns the global request router.
//
// The request router is used to register request handlers.
//
// Returns:
//   - *RequestRouter: The global request router instance
func (f *Framework) GetRequestRouter() *RequestRouter {
	return GetRequestRouter()
}

// GetResponseRouter returns the global response router.
//
// The response router is used to register response handlers.
//
// Returns:
//   - *ResponseRouter: The global response router instance
func (f *Framework) GetResponseRouter() *ResponseRouter {
	return GetResponseRouter()
}

// Stop gracefully stops the framework.
//
// This method:
//  1. Stops the IPC server
//  2. Stops the worker pool (waits for pending tasks)
//  3. Cleans up resources
//
// Note: Stop is idempotent and can be called multiple times safely.
func (f *Framework) Stop() {
	if !f.started {
		return
	}

	LogInfo("Tyke framework stopping")

	if f.ipcServer != nil {
		f.ipcServer.Stop()
	}

	component.StopWorkerPool(true)

	f.started = false
	LogInfo("Tyke framework stopped")
}
