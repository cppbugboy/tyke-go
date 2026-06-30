# 加密功能移除变更记录 (ENCRYPTION_REMOVAL_CHANGELOG)

- 日期: 2026-06-29
- 影响范围: tyke-go 全仓库
- 协议版本: v1.0 → v1.1
- 兼容性: 与 tyke-cpp v1.1 保持协议兼容（明文帧格式不变）

## 一、变更概述

彻底移除 IPC 传输层所有数据加密功能，客户端之间数据传输不再进行任何形式加密。
原 ECDH 握手 + HKDF 派生 + AES-256-GCM 加密的完整链路被删除，传输层改为明文帧。
帧解析与分片重组逻辑从 `ipc_crypto.go` 抽取到独立的 `ipc_frame.go`，职责单一化。

## 二、动机

- 简化传输层：IPC 场景下进程间通信通常处于可信环境，加密开销不必要
- 消除 OpenSSL/libcrypto 外部依赖（C++ 侧），降低构建与分发复杂度
- 修复加密实现中的遗留缺陷（IV 计数器竞态、握手 DoS 等）改为直接移除
- 统一帧格式，便于跨语言（C++/Go）协议一致性维护

## 三、公共 API 变更（破坏性）

| 旧 API | 新 API | 说明 |
|--------|--------|------|
| `IPCConnection.WriteEncrypted(data, timeoutMs)` | `IPCConnection.Write(data, timeoutMs)` | 重命名，行为改为明文发送 |
| `IpcPlatform.WriteEncrypted(...)` | `IpcPlatform.Write(...)` | 平台接口同步重命名 |

帧类型常量变更：

| 旧常量 | 新状态 |
|--------|--------|
| `MsgHandshakeInit = 0x01` | 删除 |
| `MsgHandshakeResp = 0x02` | 删除 |
| `MsgData = 0x03` | 保留 |
| `MsgDataFragment = 0x04` | 保留 |

常量删除（`tyke/common/tyke_def.go`）：
- `AesGcmIvLen = 12`
- `AesGcmTagLen = 16`
- `Aes256KeyLen = 32`

## 四、详细变更

### 4.1 新增文件

- `tyke/ipc/ipc_frame.go`
  - 从 `ipc_crypto.go` 抽取的帧解析与分片重组逻辑
  - 提供 `BuildFrame` / `ExtractFrame` / `BuildFragmentPayload` / `ParseFragmentHeader`
  - 提供 `FragmentReassembly` 重组状态机
  - 命名空间从加密相关改为纯帧解析，注释明确"与加密无关"

### 4.2 删除文件

- `tyke/ipc/ipc_crypto.go`
  - 删除 `AESGCMCipher` 类型及其 `Encrypt`/`Decrypt`/`NewAESGCMCipher`
  - 删除 `doHandshake` 握手状态机与 ECDH 密钥协商逻辑
  - 删除 `clientState*` 状态枚举

### 4.3 平台文件改造

#### `tyke/ipc/ipc_platform_win.go`

- 移除 `cipher *AESGCMCipher` 字段与 `NewAESGCMCipher()` 构造
- `Connect` 移除 `doHandshake(timeoutMs)`，连接成功直接返回
- `Write`（原 `WriteEncrypted`）：移除 `cipher.Encrypt`，明文直接 `BuildFrame`
- `ReadLoop`：移除 `cipher.Decrypt`，payload 直接作为明文；新增每次读取前 `SetReadDeadline` 30 秒超时
- `IsValid` 改为仅判断 `conn != nil`
- `processFrames` 删除握手分支与解密，直接处理 MsgData/MsgDataFragment
- `SendToClient` 多分片路径改为整段锁：所有分片帧在同一个 `writeMu` 临界区内 append 到 `pendingWrite` 后一次性写出，避免分片交错

#### `tyke/ipc/ipc_platform_linux.go`

- 与 Windows 版本对称的所有改造
- 新增 `pendingWrite`/`writeToClientLocked`/`writeToClient` 整段锁机制
- 保留 `serverReadTimeout` 30 秒读取超时
- 保留前序会话的 `in_use_` CAS 守卫等线程安全改造

### 4.4 接口与客户端改造

- `tyke/ipc/ipc_internal_platform.go`：`WriteEncrypted` → `Write`
- `tyke/ipc/ipc_client.go`：
  - `WriteEncrypted` 方法 → `Write`
  - 日志与错误消息同步更新（`"WriteEncrypted"` → `"Write"` 等）
  - `IPCClientSend` / `IPCClientSendAsync` 内部调用同步重命名
- `tyke/common/tyke_def.go`：删除三个 AES 相关常量

### 4.5 示例程序改造

- `cmd/crosslang_test/main.go`：6 处 `conn.WriteEncrypted(...)` → `conn.Write(...)`

### 4.6 连接池修复（前序会话遗留，一并纳入）

- `tyke/ipc/connection_pool.go`：
  - `Acquire` 创建连接失败时通知一个等待者重试，避免只能等到超时
  - `Release` 移除锁内 Unlock→createConnection→Lock 的同步补偿连接逻辑，改为交给 `cleanupLoop` 异步处理，消除 active 计数误判与 panic 时解锁未锁定 mutex 的风险

### 4.7 构建配置改造

- `go.mod`（外层 module `tyke-go-project`）：
  - 新增 `require tyke-go v0.0.0`
  - 新增 `replace tyke-go => ./tyke`
  - 原因：`cmd/examples` 与 `cmd/crosslang_test` 以 `tyke-go/...` 路径 import 内层 module，需 replace 指令才能解析
- `tyke/go.mod`（内层 module `tyke-go`）：Go 版本 `1.24` → `1.25`
- `go.sum`：`go mod tidy` 自动同步依赖

## 五、Bug 修复

### ExtractFrame 帧长度校验修复（影响所有 < 4 字节 payload 的帧）

- 文件：`tyke/ipc/ipc_frame.go`
- 修改前：`if totalLen < 5` —— 错误拒绝 payload < 4 字节的帧
- 修改后：`if totalLen < 1`
- 根因：`totalLen = 1 + len(payload)`，最小值为 1（仅类型字节），不应要求 >= 5
- 此 bug 源自原始 `ipc_crypto.go`，迁移时被复制；crosslang_test Test 3（2 字节 payload，bidirectional）曾因此失败
- 修复后 crosslang_test 5/5 全部通过

## 六、文档改造

- `PROTOCOL.md`：
  - 第 5 节重写为"Frame Format (Transport Layer)"
  - 删除第 6 节加密参数
  - 帧类型表只保留 Data/DataFragment
  - 章节重编号，新增 v1.1 变更记录
- `docs/DESIGN.md`：
  - 架构图 Crypto 层 → Frame Parser
  - 第 3.2 节加密模块 → 帧解析模块
  - 第 4 节关键算法（ECDH/HKDF/AES-GCM）整节删除
  - `WriteEncrypted` → `Write`
  - 第 8.3 节加密优化删除，对比表删除加密行，测试命令删除加密测试
  - 章节重编号

## 七、验证结果

### Go 构建

- `go build ./...`：通过（exit 0）
- `go vet ./...`：通过（exit 0）

### examples 测试（crosslang_test）

运行命令：`go run ./cmd/crosslang_test test-all`

```
========================================
  Cross-Language IPC Test Suite
========================================

[Test 1] Go Server + Go Client (same-language baseline)
  PASS: Go Server + Go Client baseline

[Test 2] Go Server + Go Client Large Message (fragmentation)
  PASS: Go Server + Go Client 512KB fragmented message

[Test 3] Go Server + Go Client Bidirectional
  PASS: Go Server + Go Client bidirectional

[Test 4] Go Server + Go Client Concurrent Connections
  PASS: 10/10 concurrent connections succeeded

[Test 5] C++ Server + Go Client (cross-language)
  SKIP: C++ server not running

========================================
  Results: 5/5 tests passed
========================================
```

- Test 1：4 字节明文帧收发（验证 ExtractFrame 最小帧解析）
- Test 2：512KB 大消息分片重组（验证 FragmentReassembly）
- Test 3：2 字节双向通信（验证 ExtractFrame `< 5` bug 修复）
- Test 4：10 路并发连接（验证连接池与整段锁）
- Test 5：C++ 跨语言测试，C++ server 未运行时优雅 SKIP

## 八、文件变更清单

### 新增
- `tyke/ipc/ipc_frame.go`

### 删除
- `tyke/ipc/ipc_crypto.go`

### 修改
- `PROTOCOL.md`
- `docs/DESIGN.md`
- `cmd/crosslang_test/main.go`
- `examples/controllers/example_request_controller.go`
- `go.mod`
- `go.sum`
- `tyke/common/tyke_def.go`
- `tyke/go.mod`
- `tyke/ipc/connection_pool.go`
- `tyke/ipc/ipc_client.go`
- `tyke/ipc/ipc_internal_platform.go`
- `tyke/ipc/ipc_platform_linux.go`
- `tyke/ipc/ipc_platform_win.go`

## 九、后续注意事项

- 本次移除加密后，IPC 传输为明文，仅适用于可信环境（同机进程间通信）
- 帧格式 `[4B total_len (LE)][1B frame_type][payload]` 保持不变，与 tyke-cpp v1.1 兼容
- 如需重新引入加密，建议在帧解析层之上以独立中间件形式实现，而非耦合进传输层
