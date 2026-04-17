# Tyke Go IPC Framework

## 项目概述

Tyke 是一个高性能的跨平台 IPC（进程间通信）框架，提供安全、可靠的进程间通信能力。

### 核心特性

- **跨平台支持**：Windows（命名管道）和 Linux（Unix 域套接字）
- **安全通信**：ECDH 密钥交换 + AES-GCM 加密
- **高性能**：基于 goroutine 的并发处理
- **WorkerPool**：内置协程池支持异步任务处理
- **协议封装**：28 字节协议头 + JSON 元数据 + 二进制内容

### 目录结构

```
golang/
├── cmd/                   # 应用入口
│   ├── server/           # 服务端程序
│   └── client/           # 客户端程序
├── pkg/                   # 公共包
│   ├── common/           # 通用定义
│   ├── core/             # 核心模块
│   ├── ipc/              # IPC 模块
│   ├── component/        # 组件
│   └── controller/       # 控制器
├── internal/              # 内部模块
├── go.mod                 # 模块定义
└── build.py               # 构建脚本
```

## 快速开始

### 构建项目

```bash
# Release 模式构建（默认）
python build.py

# Debug 模式构建
python build.py --debug

# 运行测试
python build.py --test

# 运行测试并生成覆盖率报告
python build.py --test --cover

# 清理构建产物
python build.py --clean
```

### 使用示例

```go
package main

import (
    "github.com/tyke/tyke/pkg/core"
    "github.com/tyke/tyke/pkg/controller"
)

// MyController 自定义控制器
type MyController struct {
    controller.RequestController
}

// RegisterMethod 注册请求处理方法
func (c *MyController) RegisterMethod() {
    controller.RegisterRequestMethod("/hello", func(req *core.TykeRequest, resp *core.TykeResponse) {
        resp.SetContent(common.ContentTypeText, []byte("Hello, World!"))
        resp.Send()
    })
}

func main() {
    // 注册控制器
    controller.RegisterController(&MyController{})
    
    // 启动框架
    app := core.App()
    app.SetThreadPoolCount(4).
        SetLogConfig("/var/log/tyke.log", "info", 10, 5)
    
    if err := app.Start("my-server-uuid"); err != nil {
        panic(err)
    }
    defer app.Stop()
    
    // 等待退出信号
    select {}
}
```

## 构建指南

### 系统要求

- Go 1.21+

### Windows 构建

```powershell
python build.py --release
```

### Linux 构建

```bash
python build.py --release
```

### 依赖管理

```bash
# 下载依赖
go mod download

# 更新依赖
go mod tidy
```

## API 参考

详细 API 文档请参阅 [API 参考](api-reference.md)。

## 包说明

### pkg/core

核心包，包含框架的主要功能：

- `Framework`: 应用框架
- `TykeRequest`: 请求对象
- `TykeResponse`: 响应对象
- `RequestRouter`: 请求路由
- `ResponseRouter`: 响应路由
- `TykeLog`: 日志系统

### pkg/ipc

IPC 包，提供进程间通信功能：

- `IpcServer`: IPC 服务器
- `IpcConnection`: IPC 连接
- `AesGcmCipher`: AES-GCM 加密
- `EcdhKeyExchange`: ECDH 密钥交换

### pkg/component

组件包，提供通用组件：

- `WorkerPool`: 协程池
- `Pool`: 对象池

### pkg/controller

控制器包，提供控制器管理：

- `RequestController`: 请求控制器基类
- `ResponseController`: 响应控制器基类
- `RegisterController`: 注册控制器

## 常见问题

### Q: 如何处理粘包问题？

A: Tyke 使用帧协议，每个帧包含完整的消息。`FrameParser.ExtractFrame` 会自动处理帧边界。

### Q: 如何实现异步请求？

A: 使用 `TykeRequest.SendAsync` 或 `TykeRequest.SendAsyncWithFuture`。

### Q: 如何自定义日志输出？

A: 调用 `Framework.SetLogConfig` 配置日志路径和级别。

### Q: 如何调整 WorkerPool 大小？

A: 调用 `Framework.SetThreadPoolCount` 设置工作协程数量。
