# Tstunnel Quick Start Guide

## 简介

Tstunnel 是一个基于 mTLS 的 TCP 隧道传输协议，用于替代 SSH 在 Mutagen 中的底层通信机制。

## 快速开始

### 1. 准备 TLS 证书

首先需要准备以下证书文件：

- `client.crt`: 客户端证书
- `client.key`: 客户端私钥
- `ca.crt`: CA 证书（用于验证服务器）

#### 生成示例证书（仅用于测试）

```bash
# 生成 CA
openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
  -keyout ca.key -out ca.crt \
  -subj "/CN=TinyscaleCA"

# 生成服务器证书
openssl req -newkey rsa:4096 -nodes \
  -keyout server.key -out server.csr \
  -subj "/CN=containers.tinyscale.net"

openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt -days 365 \
  -extfile <(echo "subjectAltName=DNS:containers.tinyscale.net,DNS:*.containers.tinyscale.net")

# 生成客户端证书
openssl req -newkey rsa:4096 -nodes \
  -keyout client.key -out client.csr \
  -subj "/CN=client"

openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out client.crt -days 365
```

### 2. 使用 TLS 配置构建器

```go
package main

import (
    "log"
    "github.com/mutagen-io/mutagen/pkg/agent/transport/tstunnel"
)

func main() {
    // 方式 1: 使用构建器
    tlsConfig, err := tstunnel.NewTLSConfigBuilder().
        WithClientCertificate("client.crt", "client.key").
        WithCACertificate("ca.crt").
        WithServerName("abcdefg.containers.tinyscale.net").
        Build()
    if err != nil {
        log.Fatal(err)
    }
    
    // 方式 2: 使用便捷函数
    tlsConfig, err = tstunnel.LoadTLSConfigFromFiles(
        "client.crt",
        "client.key",
        "ca.crt",
        "abcdefg.containers.tinyscale.net",
    )
    if err != nil {
        log.Fatal(err)
    }
}
```

### 3. 创建 Tstunnel Transport

```go
package main

import (
    "log"
    "github.com/mutagen-io/mutagen/pkg/agent/transport/tstunnel"
)

func main() {
    tlsConfig, _ := tstunnel.LoadTLSConfigFromFiles(
        "client.crt", "client.key", "ca.crt",
        "abcdefg.containers.tinyscale.net",
    )
    
    transport, err := tstunnel.NewTransport(tstunnel.TransportOptions{
        Endpoint:        "containers.tinyscale.net:443",
        HostID:          "abcdefg",
        TLSConfig:       tlsConfig,
        UpgradeBasePath: "/tinyscale/v1",
        Prompter:        "",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // 使用 transport...
}
```

### 4. 在 remote-docker-agent 中使用

#### 配置示例

```yaml
# config.yaml
endpoint: "containers.tinyscale.net:443"
host_id: "my-container-host"
tls:
  cert: "/path/to/client.crt"
  key: "/path/to/client.key"
  ca: "/path/to/ca.crt"
upgrade_base_path: "/tinyscale/v1"
```

#### 代码示例

```go
package main

import (
    "context"
    "log"
    
    "github.com/mutagen-io/mutagen/pkg/agent"
    "github.com/mutagen-io/mutagen/pkg/agent/transport/tstunnel"
    "github.com/mutagen-io/mutagen/pkg/forwarding"
    "github.com/mutagen-io/mutagen/pkg/logging"
)

func main() {
    // 加载配置
    cfg := loadConfig("config.yaml")
    
    // 创建 logger
    logger := logging.NewLogger(logging.LevelInfo, "")
    
    // 创建 TLS 配置
    tlsConfig, err := tstunnel.LoadTLSConfigFromFiles(
        cfg.TLS.Cert,
        cfg.TLS.Key,
        cfg.TLS.CA,
        fmt.Sprintf("%s.%s", cfg.HostID, strings.Split(cfg.Endpoint, ":")[0]),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // 创建 transport
    transport, err := tstunnel.NewTransport(tstunnel.TransportOptions{
        Endpoint:        cfg.Endpoint,
        HostID:          cfg.HostID,
        TLSConfig:       tlsConfig,
        UpgradeBasePath: cfg.UpgradeBasePath,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // 连接到远程 agent
    stream, err := agent.Dial(logger, transport, agent.CommandForwarder, "")
    if err != nil {
        log.Fatal(err)
    }
    defer stream.Close()
    
    // 使用 stream 进行通信...
}
```

### 5. 端口转发示例

```go
package main

import (
    "context"
    "log"
    
    "github.com/mutagen-io/mutagen/pkg/forwarding"
    "github.com/mutagen-io/mutagen/pkg/logging"
    urlpkg "github.com/mutagen-io/mutagen/pkg/url"
)

func main() {
    logger := logging.NewLogger(logging.LevelInfo, "")
    ctx := context.Background()
    
    // 创建转发 URL
    // 注意: 这需要在 url.proto 中添加 Protocol_Tstunnel 后才能使用
    url := &urlpkg.URL{
        Kind:     urlpkg.Kind_Forwarding,
        Protocol: urlpkg.Protocol_Tstunnel,
        Host:     "my-container-host",
        Path:     "tcp:localhost:8080",  // 远程目标
        Parameters: map[string]string{
            "endpoint":     "containers.tinyscale.net:443",
            "cert":         "/path/to/client.crt",
            "key":          "/path/to/client.key",
            "ca":           "/path/to/ca.crt",
            "upgrade_base": "/tinyscale/v1",
        },
    }
    
    // 创建本地监听端点
    localEndpoint, err := forwarding.NewListenerEndpoint(
        logger,
        forwarding.Version_Version1,
        &forwarding.Configuration{},
        "tcp",
        "localhost:3000",  // 本地监听地址
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // 连接到远程端点
    remoteEndpoint, err := forwarding.Connect(
        ctx,
        logger,
        url,
        "",  // prompter
        "",  // session
        forwarding.Version_Version1,
        &forwarding.Configuration{},
        false,
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // 创建转发会话
    // (这里需要使用 forwarding.Manager 或直接使用底层 API)
    
    log.Println("Port forwarding established: localhost:3000 -> remote:8080")
}
```

### 6. 文件同步示例

```go
package main

import (
    "context"
    "log"
    
    "github.com/mutagen-io/mutagen/pkg/logging"
    "github.com/mutagen-io/mutagen/pkg/synchronization"
    urlpkg "github.com/mutagen-io/mutagen/pkg/url"
)

func main() {
    logger := logging.NewLogger(logging.LevelInfo, "")
    ctx := context.Background()
    
    // 创建同步 URL
    url := &urlpkg.URL{
        Kind:     urlpkg.Kind_Synchronization,
        Protocol: urlpkg.Protocol_Tstunnel,
        Host:     "my-container-host",
        Path:     "/app/data",  // 远程路径
        Parameters: map[string]string{
            "endpoint": "containers.tinyscale.net:443",
            "cert":     "/path/to/client.crt",
            "key":      "/path/to/client.key",
            "ca":       "/path/to/ca.crt",
        },
    }
    
    // 连接到远程端点
    endpoint, err := synchronization.Connect(
        ctx,
        logger,
        url,
        "",  // prompter
        "",  // session
        synchronization.Version_Version1,
        &synchronization.Configuration{},
        false,
    )
    if err != nil {
        log.Fatal(err)
    }
    defer endpoint.Shutdown()
    
    // 使用 synchronization.Manager 进行同步...
    
    log.Println("Synchronization endpoint connected")
}
```

## URL 格式

### Forwarding URL

```
tstunnel://host-id/protocol:address?endpoint=...&cert=...&key=...
```

示例：

```
tstunnel://my-host/tcp:localhost:8080?endpoint=containers.tinyscale.net:443&cert=/certs/client.crt&key=/certs/client.key&ca=/certs/ca.crt
```

### Synchronization URL

```
tstunnel://host-id/path?endpoint=...&cert=...&key=...
```

示例：

```
tstunnel://my-host/app/data?endpoint=containers.tinyscale.net:443&cert=/certs/client.crt&key=/certs/client.key
```

## 环境变量

可以通过环境变量设置默认值：

```bash
export TSTUNNEL_ENDPOINT="containers.tinyscale.net:443"
export TSTUNNEL_CERT="/path/to/client.crt"
export TSTUNNEL_KEY="/path/to/client.key"
export TSTUNNEL_CA="/path/to/ca.crt"
export TSTUNNEL_UPGRADE_BASE="/tinyscale/v1"
```

## 故障排查

### 连接失败

```
Error: failed to establish TLS connection: x509: certificate signed by unknown authority
```

**解决方案**：确保 CA 证书正确，或使用 `WithInsecureSkipVerify()` (仅用于测试)

### 升级失败

```
Error: server did not accept upgrade: 400 Bad Request
```

**解决方案**：
1. 检查服务器是否支持 HTTP UPGRADE
2. 确认 `upgrade_base` 路径正确
3. 查看服务器日志

### Agent 连接失败

```
Error: unable to dial agent endpoint: connection timeout
```

**解决方案**：
1. 确认服务器端 agent 已启动
2. 检查防火墙规则
3. 验证 hostID 正确

### 证书验证失败

```
Error: x509: certificate is valid for containers.tinyscale.net, not abcdefg.containers.tinyscale.net
```

**解决方案**：确保服务器证书包含正确的 SAN (Subject Alternative Name)，例如 `*.containers.tinyscale.net`

## 性能优化建议

1. **重用连接**：在应用层实现连接池
2. **调整超时**：根据网络延迟调整 `upgradeTimeout` 和 `commandTimeout`
3. **使用本地 CA**：将 CA 证书添加到系统信任存储，减少验证开销
4. **监控指标**：记录连接建立时间、传输速度等指标

## 安全最佳实践

1. ✅ **使用强密钥**：至少 2048 位 RSA 或 256 位 ECDSA
2. ✅ **定期轮换证书**：建议每 90 天轮换一次
3. ✅ **保护私钥**：使用适当的文件权限 (chmod 600)
4. ✅ **启用证书撤销检查**：在生产环境中配置 CRL 或 OCSP
5. ✅ **审计日志**：记录所有连接和操作

## 更多资源

- [完整实现文档](IMPLEMENTATION.md)
- [服务器端实现指南](./SERVER.md) (待创建)
- [TLS 证书管理](./TLS_MANAGEMENT.md) (待创建)
