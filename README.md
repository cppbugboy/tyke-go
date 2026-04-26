# Tyke Go - 高性能跨平台IPC框架

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux-green.svg)]()
[![Go](https://img.shields.io/badge/Go-1.24+-blue.svg)]()

## 概述

Tyke Go �?Tyke IPC 框架�?Go 语言实现，提供与 C++ 版本完全兼容的跨平台 IPC 功能。支�?Windows 命名管道�?Linux Unix 域套接字，内�?ECDH 密钥交换�?AES-256-GCM 加密�?
## 功能特�?
- **跨平台支�?*: Windows (Named Pipe) / Linux (Unix Domain Socket)
- **安全通信**: ECDH P-256 密钥交换 + AES-256-GCM 认证加密
- **高性能**: Goroutine 并发处理，连接池复用
- **大消息支�?*: 应用层分片机制，支持最�?16MB 消息
- **C++ 兼容**: �?C++ 版本协议完全兼容，可跨语言通信

## 环境要求

- Go 1.24+
- Windows 10/11 �?Linux (Debian/Ubuntu 20.04+)

## 依赖

```go
require (
    github.com/Microsoft/go-winio v0.6.2  // Windows 命名管道
    gopkg.in/natefinch/lumberjack.v2 v2.2.1  // 日志轮转
)
```

## 快速开�?
### 安装

```bash
go get github.com/cppbugboy/tyke-go
```

### 服务端示�?
```go
package main

import (
    "fmt"
    
    "github.com/cppbugboy/tyke-go/tyke/component"
    "github.com/cppbugboy/tyke-go/tyke/ipc"
)

func main() {
    // 初始化线程池
    component.GetCoroutinePoolInstance().Init(4)
    
    server := ipc.NewIPCServer()
    
    result := server.Start("my_service", func(cid ipc.ClientId, data []byte, 
        sendCb func(ipc.ClientId, []byte) bool) *uint32 {
        // 处理客户端请�?        response := []byte{0x00, 0x01}
        sendCb(cid, response)
        return nil
    })
    
    if !result.HasValue() {
        fmt.Printf("Server start failed: %s\n", result.Err)
        return
    }
    
    fmt.Println("Server running. Press Enter to stop...")
    fmt.Scanln()
    server.Stop()
}
```

### 客户端示�?
```go
package main

import (
    "fmt"
    
    "github.com/cppbugboy/tyke-go/tyke/ipc"
)

func main() {
    conn := ipc.NewIPCConnection()
    
    result := conn.Connect("my_service", 3000)
    if !result.HasValue() {
        fmt.Printf("Connect failed: %s\n", result.Err)
        return
    }
    defer conn.Close()
    
    // 发送请�?    request := []byte{0xCA, 0xFE}
    conn.WriteEncrypted(request, 3000)
    
    // 接收响应
    conn.ReadLoop(func(data []byte) bool {
        fmt.Printf("Received: %d bytes\n", len(data))
        return true  // 返回 true 停止读取
    }, 3000)
}
```

### 使用便捷方法

```go
package main

import (
    "fmt"
    
    "github.com/cppbugboy/tyke-go/tyke/component"
    "github.com/cppbugboy/tyke-go/tyke/ipc"
)

func main() {
    component.GetCoroutinePoolInstance().Init(4)
    
    request := []byte{0x01, 0x02}
    
    // 同步发送并接收响应
    result := ipc.IPCClientSend("my_service", request, 
        func(data []byte) bool {
            fmt.Printf("Response: %d bytes\n", len(data))
            return true
        }, 3000)
    
    if !result.HasValue() {
        fmt.Printf("Send failed: %s\n", result.Err)
    }
    
    // 异步发送（不等待响应）
    ipc.IPCClientSendAsync("my_service", request, 3000)
}
```

## API 文档

### IPCServer

| 方法 | 说明 |
|------|------|
| `NewIPCServer()` | 创建服务端实�?|
| `Start(name, callback)` | 启动服务端监�?|
| `Stop()` | 停止服务�?|
| `SendToClient(id, data)` | 向指定客户端发送数�?|

### IPCConnection

| 方法 | 说明 |
|------|------|
| `NewIPCConnection()` | 创建连接实例 |
| `Connect(name, timeoutMs)` | 连接到服务端 |
| `WriteEncrypted(data, timeoutMs)` | 发送加密数�?|
| `ReadLoop(callback, timeoutMs)` | 启动读取循环 |
| `Close()` | 关闭连接 |
| `IsValid()` | 检查连接有效�?|

### 便捷方法

| 方法 | 说明 |
|------|------|
| `IPCClientSend(name, request, callback, timeoutMs...)` | 同步发送请�?|
| `IPCClientSendAsync(name, request, timeoutMs...)` | 异步发送请�?|

## 项目结构

```
tyke-go/
├── tyke/
�?  ├── ipc/              # IPC 模块
�?  �?  ├── ipc_server.go
�?  �?  ├── ipc_client.go
�?  �?  ├── ipc_crypto.go
�?  �?  ├── connection_pool.go
�?  �?  ├── ipc_platform_win.go
�?  �?  └── ipc_platform_linux.go
�?  ├── core/             # 核心模块
�?  ├── component/        # 组件
�?  └── common/           # 通用定义
├── examples/             # 示例代码
├── cmd/                  # 命令行工�?└── docs/                 # 文档
```

## 测试

```bash
# 运行所有测�?go test -v ./tyke/...

# 运行 IPC 测试
go test -v ./tyke/ipc/...

# 运行特定测试
go test -v -run TestLargeMessage ./tyke/ipc/...
```

## 跨语言通信

Tyke Go �?Tyke C++ 使用相同的协议，可以互相通信�?
```bash
# Go 服务�?+ C++ 客户�?./bin/crosslang_test go-server-echo cross_go_echo &
./tyke-cpp/build/tests/tyke_crosslang_test cpp-client

# C++ 服务�?+ Go 客户�?./tyke-cpp/build/tests/tyke_crosslang_test cpp-server &
./bin/crosslang_test go-client cross_cpp_echo
```

## 性能指标

| 指标 | Windows | Linux |
|------|---------|-------|
| 大消息吞吐量 (16MB) | ~900 MB/s | ~800 MB/s |
| Ping-Pong 延迟 (P50) | ~26 μs | ~30 μs |
| 并发连接成功�?| 100% | 100% |

## 常见问题

### Q: 连接超时怎么办？

A: 检查服务端是否已启动，服务名称是否一致。确保已初始化线程池 `component.GetCoroutinePoolInstance().Init(4)`�?
### Q: 大消息发送失败？

A: Tyke 支持 64KB 分片，最大消�?16MB。超过此限制需要应用层自行分片�?
### Q: 如何实现跨语言通信�?
A: Tyke Go �?C++ 版本使用相同的协议，可以直接互连。注�?Linux 上使用相同的 abstract namespace 格式�?
## 贡献指南

1. Fork 本仓�?2. 创建特性分�?(`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 许可�?
本项目采�?MIT 许可�?- 详见 [LICENSE](LICENSE) 文件�?
## 联系方式

- 作�? Nick
- 日期: 2026-04-26
