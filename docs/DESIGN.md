# Tyke Go 设计文档

## 1. 概述

Tyke Go 是 Tyke IPC 框架的 Go 语言实现，提供与 C++ 版本完全兼容的跨平台 IPC 功能。本文档详细阐述 Go 实现的架构设计、核心数据结构和关键算法。

## 2. 系统架构

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                      Application Layer                       │
├─────────────────────────────────────────────────────────────┤
│  Request/Response  │  Controller  │  Dispatcher  │  Router  │
├─────────────────────────────────────────────────────────────┤
│                        IPC Layer                             │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────────┐ │
│  │ IPCServer   │  │ IPCConnection│  │ ConnectionPool      │ │
│  └─────────────┘  └──────────────┘  └─────────────────────┘ │
│  ┌─────────────────────────────────────────────────────────┐│
│  │                  Frame Parser                           ││
│  │      (BuildFrame / ExtractFrame / FragmentReassembly)   ││
│  └─────────────────────────────────────────────────────────┘│
├─────────────────────────────────────────────────────────────┤
│                    Platform Layer                            │
│  ┌─────────────────────┐  ┌─────────────────────────────┐   │
│  │ Windows (go-winio)  │  │ Linux (net.Unix)            │   │
│  │ Named Pipe          │  │ Unix Domain Socket          │   │
│  └─────────────────────┘  └─────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 包结构

| 包 | 路径 | 职责 |
|------|------|------|
| **ipc** | `tyke/ipc/` | 进程间通信核心功能 |
| **core** | `tyke/core/` | 请求/响应处理、路由分发 |
| **component** | `tyke/component/` | 线程池、对象池、时间轮 |
| **common** | `tyke/common/` | 通用定义、工具函数、日志 |

## 3. 核心类型

### 3.1 IPC 包

```go
// 服务端
type IPCServer struct {
    impl Server
}

func NewIPCServer() *IPCServer
func (s *IPCServer) Start(serverName string, callback ServerRecvDataCallback) common.BoolResult
func (s *IPCServer) Stop()
func (s *IPCServer) SendToClient(id ClientId, data []byte) common.BoolResult

// 客户端连接
type IPCConnection struct {
    impl     ClientConnection
    lastUsed time.Time
}

func NewIPCConnection() *IPCConnection
func (c *IPCConnection) Connect(serverName string, timeoutMs uint32) common.BoolResult
func (c *IPCConnection) Write(data []byte, timeoutMs uint32) common.BoolResult
func (c *IPCConnection) ReadLoop(callback ClientRecvDataCallback, timeoutMs uint32) common.BoolResult
func (c *IPCConnection) Close()
func (c *IPCConnection) IsValid() bool

// 便捷方法
func IPCClientSend(serverName string, request []byte, callback ClientRecvDataCallback, timeoutMs ...uint32) common.BoolResult
func IPCClientSendAsync(serverName string, request []byte, timeoutMs ...uint32) common.BoolResult
```

### 3.2 帧解析模块

```go
// 帧常量
const (
    MsgData          byte = 0x03
    MsgDataFragment  byte = 0x04

    MaxFramePayloadLen uint32 = 16 * 1024 * 1024
    FragmentChunkSize  uint32 = 64 * 1024
    FragmentHeaderSize uint32 = 8
)

// 帧操作
func BuildFrame(frameType byte, payload []byte) []byte
func ExtractFrame(buffer *[]byte) (frameType byte, payload []byte, err error)

// 分片重组
type FragmentReassembly struct {
    Buffer   []byte
    Total    uint32
    Received uint32
}
func BuildFragmentPayload(totalSize, offset uint32, chunk []byte) []byte
func ParseFragmentHeader(payload []byte) (totalSize, offset uint32, chunk []byte, err error)
```

`FrameParser` 通过 `BuildFrame` / `ExtractFrame` 完成传输层帧的封包与解包，数据以明文形式直接作为 payload 传输；`FragmentReassembly` 负责将大于 64 KiB 的消息按 `[4B total_size][4B offset][chunk]` 格式重组为完整消息。

### 3.3 连接池

```go
type ConnectionPoolConfig struct {
    MaxConnections     int
    AcquireTimeoutMs   uint32
    ConnectTimeoutMs   uint32
    RwTimeoutMs        uint32
    IdleTimeoutMs      uint32
    CleanupIntervalMs  uint32
}

type ConnectionPool struct { ... }

func NewConnectionPool(serverUuid string, config ConnectionPoolConfig) *ConnectionPool
func (p *ConnectionPool) Acquire() (*IPCConnection, error)
func (p *ConnectionPool) Release(conn *IPCConnection, unhealthy bool)
func (p *ConnectionPool) Stop()

// 全局工厂
type ConnectionPoolFactory struct { ... }
func GetConnectionPoolFactory() *ConnectionPoolFactory
func (f *ConnectionPoolFactory) GetPool(serverUuid string) *ConnectionPool
func (f *ConnectionPoolFactory) RemovePool(serverUuid string)
func (f *ConnectionPoolFactory) Shutdown()
```

## 4. 平台实现

### 4.1 Windows (go-winio)

```go
import "github.com/Microsoft/go-winio"

// 服务端
listener, _ := winio.ListenPipe(
    &winio.PipeConfig{
        Name:           `\\.\pipe\<name>`,
        InputBufferSize:  262144,  // 256KB
        OutputBufferSize: 262144,
    },
)

// 客户端
conn, _ := winio.DialPipe(`\\.\pipe\<name>`, nil)
```

### 4.2 Linux (Unix Domain Socket)

```go
import "net"

// 服务端 (abstract namespace)
addr := &net.UnixAddr{Name: "\x00tyke_<name>", Net: "unix"}
listener, _ := net.ListenUnix("unix", addr)

// 客户端
conn, _ := net.DialUnix("unix", nil, addr)
```

## 5. 并发模型

### 5.1 Goroutine 模型

```
┌─────────────────────────────────────────────────────────────┐
│                      Server Goroutines                       │
├─────────────────────────────────────────────────────────────┤
│  Accept Loop      │  Handle Client Goroutines               │
│  - Accept         │  - Read frames                          │
│    connections    │  - Process plaintext data               │
│  - Spawn          │  - Dispatch to thread pool              │
│    handlers       │                                         │
├───────────────────┼─────────────────────────────────┤
│  Thread Pool      │  User Callbacks                         │
│  - Process        │  - Business logic                       │
│    requests       │  - Send responses                       │
└───────────────────┴─────────────────────────────────┘
```

### 5.2 连接池并发

```go
func (p *ConnectionPool) Acquire() (*IPCConnection, error) {
    p.mu.Lock()
    
    // 1. 检查 idle 队列
    if len(p.idle) > 0 {
        conn := p.idle[0]
        p.idle = p.idle[1:]
        p.mu.Unlock()
        return conn, nil
    }
    
    // 2. 检查是否可以创建新连接
    canCreate := atomic.LoadInt32(&p.active) < p.config.MaxConnections
    if canCreate {
        atomic.AddInt32(&p.active, 1)  // 先递增，避免竞态
    }
    p.mu.Unlock()
    
    if canCreate {
        conn := p.createConnection()
        if conn != nil {
            return conn, nil
        }
        atomic.AddInt32(&p.active, -1)  // 创建失败，回退
    }
    
    // 3. 等待可用连接
    select {
    case conn := <-p.available:
        return conn, nil
    case <-time.After(timeout):
        return nil, errors.New("timeout")
    }
}
```

## 6. 与 C++ 版本的兼容性

### 6.1 协议兼容

| 特性 | C++ | Go | 兼容性 |
|------|-----|-----|--------|
| 帧格式 | [4B len][1B type][payload] | 相同 | ✅ |
| 分片 | MsgDataFragment=0x04 | 相同 | ✅ |

### 6.2 平台兼容

| 平台 | C++ | Go | 互操作性 |
|------|-----|-----|----------|
| Windows | Named Pipe `\\.\pipe\<name>` | 相同 | ✅ |
| Linux | UDS `@tyke_<name>` | 相同 | ✅ |

## 7. 性能优化

### 7.1 内存优化

- 使用 `sync.Pool` 复用对象
- 连接池复用 IPC 连接
- 避免不必要的内存拷贝

### 7.2 I/O 优化

- 增大读取缓冲区至 128KB
- 使用非阻塞 I/O
- Goroutine 并发处理

## 8. 错误处理

### 8.1 Result 类型

```go
type Result[T any] struct {
    Value T
    Has   bool
    Err   string
}

type BoolResult = Result[bool]
type ByteVecResult = Result[[]byte]

func Ok[T any](value T) Result[T]
func Err[T any](msg string) Result[T]
```

### 8.2 错误传播

- 所有可能失败的操作返回 `Result` 类型
- 错误信息包含上下文
- 日志记录关键错误

## 9. 测试（待实现）

> **注意**：本节描述的测试用例尚未实现。当前仓库不含 `*_test.go` 文件，
> 以下命令暂不可运行。测试用例的实现列为独立后续任务。

### 9.1 单元测试

```bash
go test -v ./tyke/ipc/... -run TestBuildExtractFrame
go test -v ./tyke/ipc/... -run TestFragmentReassembly
```

### 9.2 集成测试

```bash
go test -v ./tyke/ipc/... -run TestServerStartStop
go test -v ./tyke/ipc/... -run TestLargeMessage
go test -v ./tyke/ipc/... -run TestConcurrentConnections
```

### 9.3 跨语言测试

```bash
# Go 服务端 + C++ 客户端
./bin/crosslang_test go-server-echo cross_go_echo &
./tyke-cpp/build/tests/tyke_crosslang_test cpp-client

# C++ 服务端 + Go 客户端
./tyke-cpp/build/tests/tyke_crosslang_test cpp-server &
./bin/crosslang_test go-client cross_cpp_echo
```

## 10. 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0 | 2026-04-26 | 初始版本，支持 Windows/Linux 跨平台 IPC，与 C++ 版本完全兼容 |
| 1.1 | 2026-06-29 | 迁移至明文数据传输；帧解析改由 `ipc_frame` 提供 |
