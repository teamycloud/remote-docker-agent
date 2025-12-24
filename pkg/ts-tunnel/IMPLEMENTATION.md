# Tstunnel (mTLS TCP Tunnel) 实现方案

## 概述

本文档描述了在 Mutagen 中实现基于 mTLS 的 TCP 隧道传输协议（tstunnel）的完整方案。该实现旨在替代 SSH 作为底层通信机制，用于 remote-docker-agent 项目中的以下场景：

1. Docker Engine API 访问
2. 端口转发（Forwarding）
3. 文件同步（Synchronization）

## 实现思路

### 架构概览

```
┌─────────────┐                  ┌──────────────────┐                  ┌─────────────┐
│   Local     │  mTLS over TCP   │  Tinyscale       │                  │   Remote    │
│   Client    ├─────────────────>│  HTTPS Endpoint  ├─────────────────>│   Host      │
│             │                  │  (SNI Router)    │                  │   (Agent)   │
└─────────────┘                  └──────────────────┘                  └─────────────┘
      │                                   │
      │  1. TLS Handshake (mTLS)         │
      │────────────────────────────────>  │
      │                                   │
      │  2. HTTP UPGRADE Request          │
      │────────────────────────────────>  │
      │                                   │
      │  3. 101 Switching Protocols       │
      │  <────────────────────────────────│
      │                                   │
      │  4. Raw TCP Stream                │
      │  <──────────────────────────────> │
      │     (Bidirectional)               │
```

### 核心组件

实现包含以下核心组件：

1. **Transport 层** (`pkg/agent/transport/tstunnel/`)
   - 实现 `agent.Transport` 接口
   - 负责建立 mTLS 连接
   - 通过 HTTP UPGRADE 升级到原始 TCP 流
   - 提供命令执行和文件拷贝功能

2. **Protocol Handler** (`pkg/forwarding/protocols/tstunnel/` 和 `pkg/synchronization/protocols/tstunnel/`)
   - 实现 `forwarding.ProtocolHandler` 接口
   - 实现 `synchronization.ProtocolHandler` 接口
   - 处理 URL 参数解析和 transport 初始化

3. **TLS 配置工具** (`pkg/agent/transport/tstunnel/tls.go`)
   - 提供便捷的 TLS 配置构建器
   - 支持客户端证书、CA 证书配置

## 已实现的接口

### 1. agent.Transport 接口

位置：`pkg/agent/transport/tstunnel/transport.go`

实现的方法：

```go
type Transport interface {
    // Copy 将本地文件复制到远程
    Copy(localPath, remoteName string) error
    
    // Command 创建一个在远程执行命令的进程
    Command(command string) (*exec.Cmd, error)
    
    // ClassifyError 分类错误，判断是否需要安装 agent
    ClassifyError(processState *os.ProcessState, errorOutput string) (bool, bool, error)
}
```

关键实现细节：

- **Copy**：通过 `/tinyscale/v1/copy/{filename}` 端点上传文件
- **Command**：创建一个本地包装进程，通过 `/tinyscale/v1/command` 端点建立连接
- **ClassifyError**：识别连接错误、agent 未安装等情况

额外提供的方法：

```go
// Dialer 返回可用于建立升级后 TCP 连接的拨号器
func (t *tstunnelTransport) Dialer() func(ctx context.Context) (net.Conn, error)
```

### 2. forwarding.ProtocolHandler 接口

位置：`pkg/forwarding/protocols/tstunnel/protocol.go`

实现 `Connect` 方法，支持端口转发场景。

### 3. synchronization.ProtocolHandler 接口

位置：`pkg/synchronization/protocols/tstunnel/protocol.go`

实现 `Connect` 方法，支持文件同步场景。

## 使用方式

### 配置参数

Tstunnel 协议通过 URL 参数传递配置：

```
tstunnel://hostid?endpoint=containers.tinyscale.net:443&cert=/path/to/client.crt&key=/path/to/client.key&ca=/path/to/ca.crt&upgrade_base=/tinyscale/v1
```

参数说明：

| 参数 | 必需 | 说明 | 示例 |
|------|------|------|------|
| `hostid` | 是 | 远程主机标识符，用于 SNI 路由 | `abcdefg` |
| `endpoint` | 是 | HTTPS 端点地址 | `containers.tinyscale.net:443` |
| `cert` | 是 | 客户端证书文件路径 | `/path/to/client.crt` |
| `key` | 是 | 客户端私钥文件路径 | `/path/to/client.key` |
| `ca` | 否 | CA 证书文件路径（用于验证服务器） | `/path/to/ca.crt` |
| `upgrade_base` | 否 | 升级请求的基础路径 | `/tinyscale/v1` |

### 示例 1: 端口转发

```go
import (
    "github.com/mutagen-io/mutagen/pkg/forwarding"
    urlpkg "github.com/mutagen-io/mutagen/pkg/url"
)

// 创建转发 URL
url := &urlpkg.URL{
    Kind:     urlpkg.Kind_Forwarding,
    Protocol: urlpkg.Protocol_Tstunnel, // 需要在 protobuf 中添加
    Host:     "abcdefg",
    Path:     "tcp:localhost:8080",
    Parameters: map[string]string{
        "endpoint": "containers.tinyscale.net:443",
        "cert":     "/path/to/client.crt",
        "key":      "/path/to/client.key",
        "ca":       "/path/to/ca.crt",
    },
}

// 建立连接
endpoint, err := forwarding.Connect(ctx, logger, url, prompter, session, version, config, false)
if err != nil {
    log.Fatal(err)
}
defer endpoint.Shutdown()
```

### 示例 2: 文件同步

```go
import (
    "github.com/mutagen-io/mutagen/pkg/synchronization"
    urlpkg "github.com/mutagen-io/mutagen/pkg/url"
)

// 创建同步 URL
url := &urlpkg.URL{
    Kind:     urlpkg.Kind_Synchronization,
    Protocol: urlpkg.Protocol_Tstunnel, // 需要在 protobuf 中添加
    Host:     "abcdefg",
    Path:     "/path/to/remote/dir",
    Parameters: map[string]string{
        "endpoint": "containers.tinyscale.net:443",
        "cert":     "/path/to/client.crt",
        "key":      "/path/to/client.key",
    },
}

// 建立连接
endpoint, err := synchronization.Connect(ctx, logger, url, prompter, session, version, config, false)
if err != nil {
    log.Fatal(err)
}
defer endpoint.Shutdown()
```

### 示例 3: 直接使用 Transport

```go
import (
    "github.com/mutagen-io/mutagen/pkg/agent"
    "github.com/mutagen-io/mutagen/pkg/agent/transport/tstunnel"
)

// 加载 TLS 配置
tlsConfig, err := tstunnel.LoadTLSConfigFromFiles(
    "/path/to/client.crt",
    "/path/to/client.key",
    "/path/to/ca.crt",
    "abcdefg.containers.tinyscale.net",
)
if err != nil {
    log.Fatal(err)
}

// 创建 transport
transport, err := tstunnel.NewTransport(tstunnel.TransportOptions{
    Endpoint:        "containers.tinyscale.net:443",
    HostID:          "abcdefg",
    TLSConfig:       tlsConfig,
    UpgradeBasePath: "/tinyscale/v1",
    Prompter:        prompter,
})
if err != nil {
    log.Fatal(err)
}

// 拨号连接到 agent
stream, err := agent.Dial(logger, transport, agent.CommandForwarder, prompter)
if err != nil {
    log.Fatal(err)
}
defer stream.Close()
```

## TCP 无法直接提供的能力及解决方案

### 1. 进程 stdin/stdout 附加

**问题**：SSH 可以直接启动远程进程并附加到其 stdin/stdout，而 TCP 连接无法做到这一点。

**解决方案**：

在服务器端维护持久化的 agent 进程池：

```
┌──────────────────────────────────────────────────────────┐
│  Tinyscale Server                                         │
│                                                           │
│  ┌─────────────┐        ┌──────────────────────────┐    │
│  │ HTTP UPGRADE│        │  Agent Process Pool      │    │
│  │  Handler    ├───────>│                          │    │
│  └─────────────┘        │  ┌────────┐  ┌────────┐  │    │
│         │               │  │ Agent  │  │ Agent  │  │    │
│         │               │  │   1    │  │   2    │  │    │
│    TCP Stream           │  └────────┘  └────────┘  │    │
│         │               └──────────────────────────┘    │
│         v                                                │
│  ┌─────────────┐                                         │
│  │ Connection  │                                         │
│  │   Router    │                                         │
│  └─────────────┘                                         │
└──────────────────────────────────────────────────────────┘
```

服务器端实现要点：

1. **进程池管理**：为每个 hostID 维护一组预启动的 agent 进程
2. **连接路由**：根据升级路径将 TCP 流路由到对应的 agent 进程
3. **协议代理**：在 TCP 流和 agent 进程的 stdio 之间进行双向代理

### 2. 动态命令执行

**问题**：SSH 可以为每个命令创建新的会话，TCP 连接是长连接。

**解决方案**：

使用基于消息的协议层：

```go
// 命令请求格式
type CommandRequest struct {
    Command string
    Args    []string
    Env     map[string]string
}

// 命令响应格式
type CommandResponse struct {
    ExitCode int
    Stdout   []byte
    Stderr   []byte
    Error    string
}
```

服务器端在接收到命令请求后，动态分配 agent 进程执行。

### 3. SCP 文件传输

**问题**：SSH 使用 SCP 协议传输文件，TCP 需要自定义协议。

**解决方案**：

已在 `Copy` 方法中实现，使用专用的上传端点：

- 路径：`/tinyscale/v1/copy/{filename}`
- 方法：POST with streaming body
- 服务器端接收并写入到用户 home 目录

### 4. 主机密钥验证

**问题**：SSH 使用主机密钥验证服务器身份，TCP 需要其他机制。

**解决方案**：

使用 mTLS 双向认证：

1. **服务器验证**：客户端通过 CA 证书验证服务器证书
2. **客户端验证**：服务器验证客户端证书
3. **SNI 路由**：通过 SNI 确定目标主机

### 5. 端口转发

**问题**：SSH 原生支持端口转发（-L, -R），TCP 需要应用层实现。

**解决方案**：

Mutagen 的转发机制本身已经是应用层实现，不依赖 SSH 的端口转发功能。Tstunnel 提供的 `Dialer()` 方法可以被转发和同步模块使用来建立额外的连接。

### 6. 认证和授权

**问题**：SSH 有完整的认证（密码、公钥）和授权机制。

**解决方案**：

使用 mTLS 证书进行认证：

1. 证书中的 CN 或 SAN 标识用户
2. 服务器端根据证书信息进行授权
3. 可以集成外部认证系统（OAuth、LDAP 等）

### 7. 会话管理

**问题**：SSH 支持多路复用（ControlMaster），TCP 需要自己实现。

**解决方案**：

1. 使用 HTTP/2 的多路复用特性（未来增强）
2. 当前实现：每个操作建立新连接（简单但有性能开销）
3. 优化方向：实现连接池和多路复用层

## 服务器端要求

要完整支持 tstunnel，服务器端需要实现以下功能：

### 1. HTTPS 端点和 SNI 路由

```
containers.tinyscale.net
  ├─ SNI: abcdefg.containers.tinyscale.net → Host abcdefg
  ├─ SNI: xyz1234.containers.tinyscale.net → Host xyz1234
  └─ ...
```

### 2. mTLS 配置

- 配置服务器证书和私钥
- 配置 CA 证书用于验证客户端
- 启用客户端证书验证

### 3. HTTP UPGRADE 处理

端点列表：

#### `/tinyscale/v1/command`
- 命令执行连接
- 升级到原始 TCP，连接到 agent 的 forwarder 或 synchronizer 模式

#### `/tinyscale/v1/copy/{filename}`
- 文件上传
- 接收文件内容并写入到目标主机的用户 home 目录

### 4. Agent 进程管理

服务器端需要：

1. 在目标容器/主机上安装 mutagen agent
2. 维护 agent 进程池或按需启动
3. 将升级后的 TCP 流连接到 agent 的 stdin/stdout
4. 处理 agent 进程的生命周期

### 5. 示例服务器伪代码

```go
func handleUpgrade(w http.ResponseWriter, r *http.Request) {
    // 验证 TLS 客户端证书
    if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
        http.Error(w, "Client certificate required", http.StatusUnauthorized)
        return
    }
    
    // 从 SNI 提取 hostID
    hostID := extractHostIDFromSNI(r.TLS.ServerName)
    
    // 检查 UPGRADE 头
    if r.Header.Get("Connection") != "Upgrade" || 
       r.Header.Get("Upgrade") != "tcp" {
        http.Error(w, "Invalid upgrade request", http.StatusBadRequest)
        return
    }
    
    // 发送 101 响应
    w.WriteHeader(http.StatusSwitchingProtocols)
    w.Header().Set("Upgrade", "tcp")
    w.Header().Set("Connection", "Upgrade")
    
    // 劫持连接
    hijacker := w.(http.Hijacker)
    conn, bufrw, err := hijacker.Hijack()
    if err != nil {
        return
    }
    defer conn.Close()
    
    // 根据路径路由
    switch {
    case strings.HasPrefix(r.URL.Path, "/tinyscale/v1/command"):
        handleCommandStream(conn, hostID)
    case strings.HasPrefix(r.URL.Path, "/tinyscale/v1/copy/"):
        handleFileCopy(conn, hostID, r.URL.Path)
    case strings.HasPrefix(r.URL.Path, "/tinyscale/v1/stream"):
        handleGenericStream(conn, hostID)
    }
}

func handleCommandStream(conn net.Conn, hostID string) {
    // 获取或启动 agent 进程
    agent := getOrStartAgent(hostID, "forwarder")
    
    // 双向代理 TCP 流和 agent stdin/stdout
    go io.Copy(conn, agent.Stdout)
    io.Copy(agent.Stdin, conn)
}
```

## 集成到 Mutagen 主线的步骤

要将 tstunnel 完全集成到 Mutagen，需要以下额外步骤：

### 1. 更新 Protobuf 定义

编辑 `pkg/url/url.proto`，添加新的协议类型：

```protobuf
enum Protocol {
    Local = 0;
    SSH = 1;
    Docker = 11;
    Tstunnel = 12;  // 添加这一行
}
```

重新生成 Go 代码：

```bash
cd pkg/url
protoc --go_out=. --go_opt=paths=source_relative url.proto
```

### 2. 更新 URL 解析逻辑

在 `pkg/url/url.go` 中添加对 tstunnel 协议的支持：

```go
func (p Protocol) MarshalText() ([]byte, error) {
    // ... existing cases ...
    case Protocol_Tstunnel:
        result = "tstunnel"
    // ...
}

func (p *Protocol) UnmarshalText(textBytes []byte) error {
    // ... existing cases ...
    case "tstunnel":
        *p = Protocol_Tstunnel
    // ...
}
```

### 3. 注册 Protocol Handlers

取消注释以下代码中的 init() 函数：

- `pkg/forwarding/protocols/tstunnel/protocol.go`
- `pkg/synchronization/protocols/tstunnel/protocol.go`

```go
func init() {
    forwarding.ProtocolHandlers[urlpkg.Protocol_Tstunnel] = &protocolHandler{}
}
```

### 4. 添加 URL 解析支持

创建 `pkg/url/parse_tstunnel.go` 文件来处理 tstunnel URL 的解析逻辑（参考 `parse_ssh.go`）。

### 5. 更新文档

更新 Mutagen 的用户文档，说明如何使用 tstunnel 协议。

## 限制和注意事项

### 当前实现的限制

1. **Command 方法的实现**
   - 当前的 `Command()` 方法返回一个占位符 `exec.Cmd`
   - 需要修改 `pkg/agent/dial.go` 中的 agent 拨号逻辑来支持非传统进程
   - 或者，实现一个自定义的 `io.ReadWriteCloser` 来封装 TCP 连接

2. **连接效率**
   - 每个操作都建立新的 TLS 连接
   - 没有连接复用或连接池
   - 对于高频操作可能有性能开销

3. **错误处理**
   - 需要更详细的错误分类
   - 需要更好的网络错误恢复机制

### 安全考虑

1. **证书管理**
   - 需要安全存储客户端私钥
   - 需要定期轮换证书
   - 需要证书撤销机制

2. **连接安全**
   - 必须使用强密码套件（TLS 1.2+）
   - 建议启用证书固定（Certificate Pinning）
   - 需要审计日志

### 性能考虑

1. **延迟**
   - TLS 握手增加连接建立时间
   - HTTP UPGRADE 增加额外的往返时间（RTT）

2. **吞吐量**
   - TLS 加密/解密有 CPU 开销
   - 建议使用硬件加速（AES-NI）

3. **优化建议**
   - 实现连接池
   - 使用 HTTP/2 多路复用
   - 考虑使用 QUIC 协议

## 测试建议

### 单元测试

创建以下测试文件：

1. `pkg/agent/transport/tstunnel/transport_test.go`
2. `pkg/agent/transport/tstunnel/tls_test.go`
3. `pkg/forwarding/protocols/tstunnel/protocol_test.go`
4. `pkg/synchronization/protocols/tstunnel/protocol_test.go`

### 集成测试

1. **Mock 服务器测试**
   - 实现一个简单的 mock HTTPS 服务器
   - 测试 HTTP UPGRADE 流程
   - 测试文件传输

2. **端到端测试**
   - 使用真实的 mTLS 配置
   - 测试完整的转发和同步场景
   - 测试错误恢复和重连

### 性能测试

1. 连接建立时间
2. 文件传输速度
3. 并发连接处理能力
4. 资源使用（CPU、内存、网络）

## 下一步行动

### 必需的改进

1. **修复 Command 实现**
   - 当前的 `Command()` 实现是占位符
   - 需要实现真正的连接包装器
   - 或扩展 `agent.Dial()` 以支持非进程 transport

2. **完成 Protobuf 集成**
   - 添加 `Protocol_Tstunnel` 枚举值
   - 更新 URL 解析和序列化逻辑

3. **实现服务器端组件**
   - 创建示例服务器实现
   - 实现 agent 进程管理
   - 实现连接路由逻辑

### 可选的增强

1. **连接池**：减少 TLS 握手开销
2. **HTTP/2 支持**：提供多路复用
3. **更好的错误处理**：重试、超时、断线重连
4. **监控和日志**：连接指标、性能跟踪
5. **配置文件支持**：简化证书和端点配置

## 总结

本实现提供了一个完整的 mTLS TCP 隧道传输层，可以替代 SSH 用于 Mutagen 的所有主要功能。关键优势包括：

1. ✅ **不需要 SSH 服务器**：减少攻击面
2. ✅ **基于证书的认证**：更好的自动化和安全性
3. ✅ **SNI 路由**：支持多主机场景
4. ✅ **可扩展架构**：服务器端可以添加额外的逻辑层

主要挑战在于：

1. ⚠️ **服务器端实现复杂度**：需要 agent 进程管理
2. ⚠️ **Command 方法的进程模拟**：需要额外的适配层
3. ⚠️ **性能优化**：需要连接池和多路复用

总体而言，这个实现提供了一个坚实的基础，可以在此基础上进行进一步的优化和增强。
