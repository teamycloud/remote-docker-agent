
Remote Docker Agent: a Docker-aware HTTP Proxy over SSH
==================

## Overview

The project is a tiny agent that proxies local docker API call to an remote SSH host so that we can solve the following two challenges:

It runs a local agent exposing a Docker-compatible HTTP API.

* Automatic port forwarding

* Local → remote file sync

It then proxies requests to a remote Docker daemon over SSH and solve the above two problems by:

1. Automatic port forwarding

    1. Detect -p 8080:80

    2. Create SSH tunnels

    3. Expose local ports

    4. Rewrite the Docker API request to bind remote ports

    5. Keep tunnels alive as long as the container runs


2. Local → remote file sync

    1. Detect -v ./src:/app

    2. Upload files via SFTP or rsync-over-SSH

    3. Rewrite the mount to a remote temp directory

    4. Optionally watch for changes and sync incrementally


This is in development and it is not production-ready yet.

## Architecture

The `pkg/tcp_agent` package provides an HTTP-aware TCP proxy that forwards Docker API traffic through SSH tunnels with selective request interception capabilities.



## Architecture

```
┌─────────────┐       ┌──────────────┐       ┌─────────┐       ┌──────────────┐
│ Docker CLI  │──TCP─→│  TCP Proxy   │──SSH─→│ Remote  │──────→│ Docker       │
│             │       │  (HTTP-Aware)│       │ Host    │       │ Daemon       │
└─────────────┘       └──────────────┘       └─────────┘       └──────────────┘
                            │
                            ├─ Parse HTTP requests
                            ├─ Intercept specific endpoints
                            ├─ Dump request/response
                            ├─ Handle Keep-Alive
                            └─ Detect protocol upgrades
```


Agent listens locally on e.g. localhost:23750.

Docker CLI is pointed at DOCKER_HOST=tcp://localhost:23750.

Agent parses incoming Docker API requests.

For POST /containers/create:

* Parses HostConfig.PortBindings → sets up SSH port forwards.

* Parses HostConfig.Binds → syncs local paths to remote temp dirs and rewrites.

For other endpoints → passes through to remote via Docker API over SSH.

## Features

- **SSH Transport**: Secure Docker API access over SSH tunnels
- **HTTP Keep-Alive Support**: Handles multiple HTTP requests on the same TCP connection
- **Selective Interception**: Parse and intercept specific Docker API calls (e.g., container create)
- **Protocol Upgrade Detection**: Automatically switches to transparent TCP mode for upgraded connections (attach, exec)
- **Transparent Fallback**: Raw TCP proxy for non-HTTP traffic
- **Unix Socket Support**: Connect to Docker via Unix sockets or TCP on remote hosts

## Usage

### Command Line Example

```bash
# Build the example
go build -o tcp-proxy ./cmd/tcp_proxy_example

# Run with SSH transport
./tcp-proxy \
  -listen 127.0.0.1:2375 \
  -ssh-user root \
  -ssh-host remote.example.com:22 \
  -ssh-key ~/.ssh/id_rsa \
  -remote-docker unix:///var/run/docker.sock

# Use Docker CLI with the proxy
export DOCKER_HOST=tcp://127.0.0.1:2375
docker ps
docker run -it hello-world
```

### Config Fields

- **`ListenAddr`**: Local address to listen on (e.g., `"127.0.0.1:2375"`)
- **`SSHUser`**: SSH username for remote connection (e.g., `"root"`)
- **`SSHHost`**: SSH host and port (e.g., `"remote.example.com:22"`)
- **`SSHKeyPath`**: Path to SSH private key (e.g., `"/home/user/.ssh/id_rsa"`)
- **`RemoteDocker`**: Remote Docker socket URL:
    - Unix socket: `"unix:///var/run/docker.sock"`
    - TCP: `"tcp://127.0.0.1:2375"`

You’ll need a remote Docker daemon reachable over SSH, typically:

SSH: user@remote-host

Docker: unix:///var/run/docker.sock on remote


## Roadmap

- Support for WebSocket/hijacked connections (exec, attach, logs -f)
- Automatic port forwarding setup (Full handling of nat.Port types and multi-port mappings.), tracking port forwards per container and closing them on stop.
- Volume path translation for bind mounts with real SFTP sync with incremental updates and ignoring patterns: including Proper local/remote path detection across OSes (Windows vs Linux).
- Robust SSH connection pooling and reuse.
- Multi-tenant mapping: context → remote host → SSH key → Docker daemon.
- Network bridging: access to the whole organization network in target VM underlay
