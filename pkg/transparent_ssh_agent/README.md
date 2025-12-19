# TCP Agent - TCP-based Docker API Proxy

The `tcp_agent` package provides TCP-based proxies for Docker API over SSH tunnels.

## Features

- **Transparent TCP Proxy**: Forward raw TCP connections through SSH tunnels
- **Docker-aware HTTP Proxy**: Intercept and modify Docker API HTTP requests/responses
- **SSH Tunneling**: Secure connection to remote Docker daemons
- **Extensible Hooks**: Add custom logic before requests and after responses

## Architecture

```
┌─────────────┐       ┌──────────────┐       ┌─────────┐       ┌──────────────┐
│ Docker CLI  │──TCP─→│  TCP Proxy   │──SSH─→│ Remote  │──────→│ Docker       │
│             │       │  (Local)     │       │ Host    │       │ Daemon       │
└─────────────┘       └──────────────┘       └─────────┘       └──────────────┘
```

## Components

### 1. Transparent TCP Proxy (`tcp_proxy.go`)

A simple transparent proxy that forwards all TCP traffic through an SSH tunnel without inspecting the content.

**Use case**: When you just need to forward Docker API over SSH without any modifications.

```go
cfg := tcp_agent.Config{
    ListenAddr:      "127.0.0.1:2375",
    SSHUser:         "root",
    SSHHost:         "remote.example.com:22",
    SSHKeyPath:      "/home/user/.ssh/id_rsa",
    RemoteDockerURL: "unix:///var/run/docker.sock",
}

proxy, err := tcp_agent.NewTCPProxy(cfg)
if err != nil {
    log.Fatal(err)
}
defer proxy.Close()

if err := proxy.ListenAndServe(); err != nil {
    log.Fatal(err)
}
```

### 2. Docker-aware TCP Proxy (`docker_tcp_proxy.go`)

An HTTP-aware proxy that can parse Docker API requests and responses, allowing interception and modification.

**Use case**: When you need to intercept container creation, modify port mappings, adjust volumes, or add custom logic to Docker API calls.

```go
cfg := tcp_agent.Config{
    ListenAddr:      "127.0.0.1:2375",
    SSHUser:         "root",
    SSHHost:         "remote.example.com:22",
    SSHKeyPath:      "/home/user/.ssh/id_rsa",
    RemoteDockerURL: "unix:///var/run/docker.sock",
}

proxy, err := tcp_agent.NewDockerTCPProxy(cfg)
if err != nil {
    log.Fatal(err)
}
defer proxy.Close()

// Enable container creation interception
proxy.InterceptCreateContainer()

// Add custom request hook
proxy.SetBeforeRequestHook(func(req *http.Request) error {
    log.Printf("Intercepting: %s %s", req.Method, req.URL.Path)
    // Modify request here
    return nil
})

// Add custom response hook
proxy.SetAfterResponseHook(func(resp *http.Response) error {
    log.Printf("Response status: %d", resp.StatusCode)
    // Process response here
    return nil
})

if err := proxy.ListenAndServe(); err != nil {
    log.Fatal(err)
}
```

## Configuration

```go
type Config struct {
    ListenAddr      string // Local address to listen on (e.g., "127.0.0.1:2375")
    SSHUser         string // SSH username
    SSHHost         string // SSH host:port (e.g., "remote.example.com:22")
    SSHKeyPath      string // Path to SSH private key
    RemoteDockerURL string // Remote Docker socket (e.g., "unix:///var/run/docker.sock" or "tcp://localhost:2376")
}
```

## Usage with Docker CLI

Once the proxy is running, configure your Docker CLI to use it:

```bash
export DOCKER_HOST=tcp://127.0.0.1:2375
docker ps
docker run -it ubuntu bash
```

## Key Differences from HTTP Agent

| Feature | HTTP Agent (`pkg/agent`) | TCP Agent (`pkg/tcp_agent`) |
|---------|-------------------------|----------------------------|
| Protocol | HTTP only | TCP (transparent) |
| Interception | HTTP handlers | Stream parsing |
| Flexibility | More structured | More transparent |
| Overhead | Higher (HTTP routing) | Lower (direct TCP) |
| Use Case | Complex API modifications | Simple forwarding or lightweight interception |

## Implementation Details

### Transparent Proxy

- Accepts TCP connections on local port
- Establishes SSH connection to remote host
- Dials remote Docker socket through SSH tunnel
- Bidirectional `io.Copy` between client and remote

### Docker-aware Proxy

- Same as transparent proxy, but with HTTP parsing
- Uses `bufio.Reader` to parse HTTP requests/responses
- Calls hooks before/after forwarding
- Falls back to transparent mode if HTTP parsing fails

## Future Enhancements

- [ ] Connection pooling for better performance
- [ ] TLS support for local connections
- [ ] Metrics and monitoring hooks
- [ ] Request/response logging options
- [ ] Support for WebSocket/hijacked connections (exec, attach, logs -f)
- [ ] Automatic port forwarding setup
- [ ] Volume path translation for bind mounts

## See Also

- `pkg/agent` - HTTP-based Docker API proxy with more sophisticated features
- `ssh_client.go` - SSH client implementation
