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
│  │              Crypto (ECDH + AES-GCM)                    ││
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
func (c *IPCConnection) WriteEncrypted(data []byte, timeoutMs uint32) common.BoolResult
func (c *IPCConnection) ReadLoop(callback ClientRecvDataCallback, timeoutMs uint32) common.BoolResult
func (c *IPCConnection) Close()
func (c *IPCConnection) IsValid() bool

// 便捷方法
func IPCClientSend(serverName string, request []byte, callback ClientRecvDataCallback, timeoutMs ...uint32) common.BoolResult
func IPCClientSendAsync(serverName string, request []byte, timeoutMs ...uint32) common.BoolResult
```

### 3.2 加密模块

```go
// 帧常量
const (
    MsgHandshakeInit byte = 0x01
    MsgHandshakeResp byte = 0x02
    MsgData          byte = 0x03
    MsgDataFragment  byte = 0x04
    
    MaxFramePayloadLen uint32 = 16 * 1024 * 1024
    FragmentChunkSize  uint32 = 64 * 1024
    FragmentHeaderSize uint32 = 8
)

// 帧操作
func BuildFrame(frameType byte, payload []byte) []byte
func ExtractFrame(buffer *[]byte) (frameType byte, payload []byte, err error)

// ECDH 密钥交换
type ECDHKeyExchange struct { ... }
func NewECDHKeyExchange() *ECDHKeyExchange
func (e *ECDHKeyExchange) GenerateKey() common.BoolResult
func (e *ECDHKeyExchange) GetPublicKeyDer() common.ByteVecResult
func (e *ECDHKeyExchange) ComputeSharedSecret(peerPubDer []byte) common.ByteVecResult

// AES-GCM 加密
type AESGCMCipher struct { ... }
func NewAESGCMCipher() *AESGCMCipher
func (c *AESGCMCipher) Init(sharedSecret []byte) common.BoolResult
func (c *AESGCMCipher) Encrypt(plaintext []byte) common.ByteVecResult
func (c *AESGCMCipher) Decrypt(ciphertext []byte) common.ByteVecResult

// 分片重组
type FragmentReassembly struct {
    Buffer   []byte
    Total    uint32
    Received uint32
}
func BuildFragmentPayload(totalSize, offset uint32, encryptedChunk []byte) []byte
func ParseFragmentHeader(payload []byte) (totalSize, offset uint32, encryptedChunk []byte, err error)
```

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

## 4. 关键算法

### 4.1 ECDH 密钥交换 (Go 标准库)

```go
import (
    "crypto/ecdh"
    "crypto/x509"
)

// 1. 生成密钥对 (P-256)
privateKey, _ := ecdh.P256().GenerateKey(rand.Reader)
publicKey := privateKey.PublicKey()

// 2. 导出公钥 (SPKI DER)
pubDer, _ := x509.MarshalPKIXPublicKey(publicKey)

// 3. 计算共享密钥
peerPub, _ := x509.ParsePKIXPublicKey(peerPubDer)
peerEcdhPub := peerPub.(*ecdsa.PublicKey).ECDH()
sharedSecret, _ := privateKey.ECDH(peerEcdhPub)
```

### 4.2 HKDF 密钥派生

```go
import (
    "crypto/hmac"
    "crypto/sha256"
)

const (
    hkdfSalt = "tyke-v1-hkdf-salt"
    hkdfInfo = "tyke-v1-aes256-key"
)

func hkdfDeriveKey(hash func() hash.Hash, salt, ikm, info []byte, length int) ([]byte, error) {
    // Extract: PRK = HMAC-Hash(salt, IKM)
    prk := hmac.New(hash, salt)
    prk.Write(ikm)
    prkBytes := prk.Sum(nil)
    
    // Expand: OKM = HMAC-Hash(PRK, Info || counter)
    var okm []byte
    counter := byte(1)
    for len(okm) < length {
        h := hmac.New(hash, prkBytes)
        h.Write(info)
        h.Write([]byte{counter})
        okm = append(okm, h.Sum(nil)...)
        counter++
    }
    return okm[:length], nil
}
```

### 4.3 AES-GCM 加密

```go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
)

type AESGCMCipher struct {
    aesGcm cipher.AEAD
    ivRand [4]byte
    ivCtr  uint64
}

func (c *AESGCMCipher) Encrypt(plaintext []byte) ([]byte, error) {
    // IV: [4B 随机][8B 大端计数器]
    iv := make([]byte, 12)
    copy(iv[:4], c.ivRand[:])
    binary.BigEndian.PutUint64(iv[4:], atomic.AddUint64(&c.ivCtr, 1))
    
    ciphertext := c.aesGcm.Seal(nil, iv, plaintext, nil)
    return append(iv, ciphertext...), nil  // IV + Ciphertext + Tag
}

func (c *AESGCMCipher) Decrypt(ciphertext []byte) ([]byte, error) {
    iv := ciphertext[:12]
    encrypted := ciphertext[12:]
    return c.aesGcm.Open(nil, iv, encrypted, nil)
}
```

## 5. 平台实现

### 5.1 Windows (go-winio)

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

### 5.2 Linux (Unix Domain Socket)

```go
import "net"

// 服务端 (abstract namespace)
addr := &net.UnixAddr{Name: "\x00tyke_<name>", Net: "unix"}
listener, _ := net.ListenUnix("unix", addr)

// 客户端
conn, _ := net.DialUnix("unix", nil, addr)
```

## 6. 并发模型

### 6.1 Goroutine 模型

```
┌─────────────────────────────────────────────────────────────┐
│                      Server Goroutines                       │
├─────────────────────────────────────────────────────────────┤
│  Accept Loop      │  Handle Client Goroutines               │
│  - Accept         │  - Read frames                          │
│    connections    │  - Decrypt data                         │
│  - Spawn          │  - Dispatch to thread pool              │
│    handlers       │                                         │
├───────────────────┼─────────────────────────────────────────┤
│  Thread Pool      │  User Callbacks                         │
│  - Process        │  - Business logic                       │
│    requests       │  - Send responses                       │
└───────────────────┴─────────────────────────────────────────┘
```

### 6.2 连接池并发

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

## 7. 与 C++ 版本的兼容性

### 7.1 协议兼容

| 特性 | C++ | Go | 兼容性 |
|------|-----|-----|--------|
| 帧格式 | [4B len][1B type][payload] | 相同 | ✅ |
| ECDH | OpenSSL P-256 | crypto/ecdh P-256 | ✅ |
| HKDF | OpenSSL HKDF | 手动实现 | ✅ |
| AES-GCM | OpenSSL EVP_aes_256_gcm | crypto/aes + cipher | ✅ |
| 公钥格式 | X.509 SPKI DER | 相同 | ✅ |
| 分片 | MsgDataFragment=0x04 | 相同 | ✅ |

### 7.2 平台兼容

| 平台 | C++ | Go | 互操作性 |
|------|-----|-----|----------|
| Windows | Named Pipe `\\.\pipe\<name>` | 相同 | ✅ |
| Linux | UDS `@tyke_<name>` | 相同 | ✅ |

## 8. 性能优化

### 8.1 内存优化

- 使用 `sync.Pool` 复用对象
- 连接池复用 IPC 连接
- 避免不必要的内存拷贝

### 8.2 I/O 优化

- 增大读取缓冲区至 128KB
- 使用非阻塞 I/O
- Goroutine 并发处理

### 8.3 加密优化

- 复用 AES-GCM 上下文
- IV 计数器避免随机数生成
- 批量加密减少函数调用

## 9. 错误处理

### 9.1 Result 类型

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

### 9.2 错误传播

- 所有可能失败的操作返回 `Result` 类型
- 错误信息包含上下文
- 日志记录关键错误

## 10. 测试

### 10.1 单元测试

```bash
go test -v ./tyke/ipc/... -run TestECDHKeyExchange
go test -v ./tyke/ipc/... -run TestAESGCMCipher
go test -v ./tyke/ipc/... -run TestBuildExtractFrame
```

### 10.2 集成测试

```bash
go test -v ./tyke/ipc/... -run TestServerStartStop
go test -v ./tyke/ipc/... -run TestLargeMessage
go test -v ./tyke/ipc/... -run TestConcurrentConnections
```

### 10.3 跨语言测试

```bash
# Go 服务端 + C++ 客户端
./bin/crosslang_test go-server-echo cross_go_echo &
./tyke-cpp/build/tests/tyke_crosslang_test cpp-client

# C++ 服务端 + Go 客户端
./tyke-cpp/build/tests/tyke_crosslang_test cpp-server &
./bin/crosslang_test go-client cross_cpp_echo
```

## 11. 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0 | 2026-04-26 | 初始版本，支持 Windows/Linux 跨平台 IPC，与 C++ 版本完全兼容 |
