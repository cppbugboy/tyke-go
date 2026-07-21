# Tyke Go

**Tyke Go** 是一个高性能、跨平台的 Go 语言 IPC（进程间通信）框架。它提供了完整的客户端-服务端通信层，内置路由、过滤器、对象池和异步消息机制，专为 Windows 和 Linux 上的本地进程通信而设计。

> 📘 Tyke 同时提供 [C++ 实现](../tyke-cpp/README.md)，两者共享相同的传输协议，支持跨语言 IPC 通信。

---

## ✨ 特性

- **跨平台 IPC** — Windows（命名管道，基于 `go-winio`）和 Linux（Unix 域套接字）
- **请求/响应模型** — 全双工消息传递，支持同步和异步模式
- **泛型路由** — 基于 Go 1.18+ 泛型的类型安全路由器和路由分组，O(1) 映射查找
- **过滤器链** — 可插拔的请求/响应前后拦截器
- **泛型对象池** — `ObjectPool[T]`，支持容量限制、重置钩子和缓存命中率统计
- **优先级协程池** — 三级优先级调度（高/中/低），支持自动扩缩容
- **层级时间轮** — 4 层时间轮，O(1) 增删，支持一次性定时器和周期任务
- **连接池** — 以服务端 UUID 为键的连接池，支持空闲超时管理与后台清理
- **大消息支持** — 超过 64 KB 的消息自动分片，最大支持 16 MB 载荷
- **高效协议** — 28 字节精简二进制头部，魔数标识（`TYKE`），小端序编码

---

## 📂 项目结构

```
tyke-go/
├── go.mod                      # 模块: tyke-go-project (Go 1.25)
├── go.sum
├── LICENSE                     # MIT 许可证
├── PROTOCOL.md                 # 传输协议规范
├── build.py                    # Python 构建脚本（支持 debug/release/测试/基准/检查/文档）
├── ENCRYPTION_REMOVAL_CHANGELOG.md
├── docs/
│   └── DESIGN.md               # 架构与设计文档
├── tyke/                       # 核心库（模块: tyke-go）
│   ├── go.mod
│   ├── go.sum
│   ├── common/                 # 类型定义、日志、结果类型、工具函数
│   │   ├── tyke_def.go         # 协议头部、状态码、内容/消息类型定义
│   │   ├── tyke_result.go      # 泛型 Result[T]、BoolResult、ByteVecResult
│   │   ├── common_def.go       # 公共常量
│   │   ├── log_def.go          # 结构化日志
│   │   └── tyke_utils.go       # UUID 生成、验证函数
│   ├── core/                   # 框架、路由器、分发器、过滤器
│   │   ├── framework.go        # 应用单例，生命周期管理
│   │   ├── controller.go       # 控制器基础接口
│   │   ├── request.go          # 请求对象，同步/异步发送方法
│   │   ├── response.go         # 响应对象，Send/SendAsync
│   │   ├── request_stub.go     # 异步回调与 Future 存根管理
│   │   ├── dispatcher.go       # 请求/响应分发逻辑
│   │   ├── router_base.go      # 泛型 RouterBase[F, H]
│   │   ├── router_group.go     # 层级路由分组
│   │   ├── request_router.go   # 请求路由器单例
│   │   ├── response_router.go  # 响应路由器单例
│   │   ├── filter_interfaces.go # 请求/响应过滤器接口
│   │   ├── data_handler.go     # IPC 数据回调
│   │   ├── data_proc.go        # 编解码逻辑
│   │   ├── response_future.go  # 异步响应 Future/Promise
│   │   └── log.go              # 日志初始化
│   ├── component/              # 可复用组件
│   │   ├── coroutine_pool.go   # 优先级协程池，支持自动扩缩容
│   │   ├── timing_wheel.go     # 4 层时间轮
│   │   ├── object_pool.go      # 泛型对象池，支持指标统计
│   │   ├── singleton.go        # 单例辅助
│   │   └── context.go          # 请求上下文
│   └── ipc/                    # IPC 传输层
│       ├── ipc_server.go       # 服务端封装
│       ├── ipc_client.go       # 客户端接口与静态辅助函数
│       ├── connection_pool.go  # 连接池，支持空闲清理
│       ├── connection_pool_factory.go
│       ├── ipc_frame.go        # 帧构建与分片重组
│       ├── ipc_types.go        # 类型定义
│       ├── ipc_internal_platform.go  # 平台抽象接口
│       ├── ipc_platform_win.go       # Windows: 命名管道（go-winio）
│       ├── ipc_platform_linux.go     # Linux: Unix 域套接字
│       └── ipc_frame_test.go   # 帧操作单元测试
├── examples/                   # 示例服务端与客户端
│   ├── server/main.go
│   ├── client/main.go
│   └── controllers/            # 示例请求/响应处理器
├── cmd/
│   └── crosslang_test/         # 跨语言 IPC 测试套件
└── build.py                    # 构建自动化脚本
```

---

## 🚀 快速开始

### 环境要求

- **Go** ≥ 1.25
- **Python** ≥ 3.7（构建脚本可选，直接使用 `go build` 亦可）

### 构建

**使用构建脚本:**
```bash
cd tyke-go

# 发布构建（优化、去除调试信息）
python build.py --release

# 调试构建
python build.py --debug

# 构建并运行测试
python build.py --test

# 构建并生成覆盖率报告
python build.py --test --cover

# 强制重新构建（忽略缓存）
python build.py --force

# 运行代码检查
python build.py --lint

# 生成接口文档
python build.py --doc

# 清理所有构建产物
python build.py --clean
```

**直接使用 Go 命令:**
```bash
cd tyke-go
go build ./...
```

### 运行示例

在一个终端启动服务端：

```bash
go run ./examples/server
```

在另一个终端运行客户端：

```bash
go run ./examples/client
```

客户端依次演示四种请求模式：同步请求、即发即弃异步、回调式异步、Future 异步。

---

## 💻 使用指南

### 1. 引入包

```go
import (
    "tyke-go/common"
    "tyke-go/core"
)
```

### 2. 创建服务端

```go
package main

import (
    "fmt"
    "os"
    "os/signal"
    "syscall"

    _ "your-project/controllers"  // 通过 init() 自动注册
    "tyke-go/core"
)

func main() {
    framework := core.App()
    framework.SetThreadPoolCount(4)
    framework.SetLogConfig("./server.log", "debug", 1024, 5)

    result := framework.Start("1879b1d8-8ab0-4542-8421-8d845eca6587")
    if !result.HasValue() {
        fmt.Printf("服务端启动失败: %s\n", result.Err)
        os.Exit(1)
    }

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    framework.Shutdown()
}
```

### 3. 注册请求处理器

```go
package controllers

import (
    "tyke-go/common"
    "tyke-go/core"
)

func init() {
    ExampleRegisterMethod()
}

func ExampleRegisterMethod() {
    router := core.GetRequestRouter()
    root := router.GetRoot()

    root.AddSubGroup("/api/user").AddRouteHandler("/login", HandleUserLogin)
    root.AddSubGroup("/api/user").AddRouteHandler("/logout", HandleUserLogout)
    root.AddSubGroup("/api/data").AddRouteHandler("/query", HandleDataQuery)
}

func HandleUserLogin(request *core.Request, response *core.Response) {
    // 解析 JSON 请求体...
    response.SetContent(common.ContentTypeJson, responseBytes)
    response.SetResult(int(common.StatusSuccess), "OK")
    response.SetModule(request.GetModule())
    response.SetRoute(request.GetRoute())
}
```

### 4. 发送请求（客户端）

**同步模式** — 阻塞直到收到响应：
```go
req := core.AcquireRequest()
defer core.ReleaseRequest(req)

req.SetModule("my_module")
req.SetRoute("/api/data/query")
req.SetContent(common.ContentTypeJson, jsonBytes)

var resp core.Response
result := req.Send("服务端UUID", &resp)
```

**回调式异步** — 通过回调函数接收响应：
```go
req.SendAsyncWithFunc("服务端UUID", func(response *core.Response) {
    // 处理异步响应...
})
```

**Future 异步** — 后续通过 `ResponseFuture` 阻塞获取：
```go
future, err := req.SendAsyncWithFuture("服务端UUID")
if err == nil {
    response := future.GetResponse()  // 阻塞直到响应到达
}
```

**即发即弃** — 不处理响应：
```go
req.SendAsync("服务端UUID")
```

### 5. 注册响应处理器

```go
func ExampleResponseRegisterMethod() {
    router := core.GetResponseRouter()
    root := router.GetRoot()

    root.AddSubGroup("/api/async").
        AddRouteHandler("/callback", HandleAsyncCallback).
        AddRouteHandler("/notification", HandleAsyncNotification)
}

func HandleAsyncCallback(response *core.Response) {
    status, reason := response.GetResult()
    // 处理异步回调...
}
```

### 6. 添加过滤器

```go
type AuthFilter struct{}

func (f *AuthFilter) Before(request *core.Request, response *core.Response) bool {
    // 验证令牌，返回 false 则拒绝请求
    return true
}
func (f *AuthFilter) After(request *core.Request, response *core.Response) bool {
    // 后处理响应
    return true
}

// 挂载到路由分组
root.AddSubGroup("/api/admin").
    AddFilter(&AuthFilter{}).
    AddRouteHandler("/dashboard", handler)
```

---

## 🧱 架构

```
┌─────────────────────────────────────────────────┐
│                    应用程序                       │
├─────────────────────────────────────────────────┤
│   控制器 (Controllers) │ 过滤器 (Filters) │ 路由器  │
├─────────────────────────────────────────────────┤
│   请求/响应  │  分发器 (Dispatcher)  │  存根管理   │
├─────────────────────────────────────────────────┤
│   IPC 服务端  │  IPC 客户端  │  连接池             │
├─────────────────────────────────────────────────┤
│   帧解析器  │  协议层 (28字节头部)                 │
├─────────────────────────────────────────────────┤
│   Windows 命名管道 (go-winio)                    │
│   Linux Unix 域套接字 (net)                      │
└─────────────────────────────────────────────────┘
```

| 组件           | 说明                                                                           |
| -------------- | ------------------------------------------------------------------------------ |
| **Framework**  | 应用程序生命周期：初始化、启动、关闭。通过 `core.App()` 获取线程安全单例。        |
| **Router**     | 泛型 `RouterBase[FilterType, HandlerFunc]`。O(1) 映射查找。层级分组，过滤器链可继承。 |
| **Dispatcher** | 将收到的请求/响应通过过滤器链（前置 → 处理器 → 后置）路由。未匹配的异步响应回退到存根表。 |
| **IPC 服务端**  | 封装平台特定的 Server 接口。接受连接，将数据分发到回调。                          |
| **IPC 客户端**  | `IPCClientSend`（同步）和 `IPCClientSendAsync`（即发即弃）。通过 `ConnectionPoolFactory` 池化连接。 |
| **连接池**     | 以服务端 UUID 为键的池，支持获取/归还语义。后台清理协程移除空闲连接。             |
| **帧解析器**   | `[4B 长度][1B 类型][载荷]`。大于 64 KB 的载荷按 64 KB 分片并重组。               |
| **协程池**     | 三级优先级队列（高 → 中 → 低），基于负载因子自动扩缩容，指标统计，异常恢复。      |
| **时间轮**     | 4 层（200ms、1s、10s、60s）。O(1) 增删。用于请求存根的超时清理。                  |
| **对象池**     | `ObjectPool[T]`，支持工厂函数、重置钩子、容量限制和缓存命中率统计。               |

---

## 📦 依赖项

| 库                                                      | 用途               | 许可证 |
| ------------------------------------------------------- | ------------------ | ------ |
| [go-winio](https://github.com/Microsoft/go-winio)       | Windows 命名管道传输 | MIT    |
| [lumberjack](https://github.com/natefinch/lumberjack)   | 日志文件滚动        | MIT    |

所有依赖在 `go.mod` 中声明，由 Go 模块自动管理。

---

## 🔌 协议

传输协议详见 `PROTOCOL.md`。核心要点：

- **魔数**: `TYKE`（4 字节）
- **头部**: 28 字节固定长度，1 字节对齐
- **消息类型**: 请求、响应及各类异步变体（回调、Future）
- **内容类型**: 文本、JSON、二进制
- **帧格式**: `[4B 总长度 (小端序)][1B 类型][载荷]`
- **分片**: 超过 64 KB 的消息按 64 KB 分片，每片带 8 字节分片头
- **最大载荷**: 每帧 16 MB

该协议与 [C++ 实现](../tyke-cpp/README.md) 共享，支持跨语言 IPC 通信。

---

## 🛠 开发命令

```bash
# 运行测试
go test ./...

# 竞态检测
go test -race ./...

# 运行基准测试
go test -bench=. -benchmem ./...

# 格式化代码
gofmt -w .

# 静态检查
go vet ./...
```

---

## 📄 许可证

MIT 许可证。详见 [LICENSE](LICENSE)。
