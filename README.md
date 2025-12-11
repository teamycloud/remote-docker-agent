
DOCKER CLI AGNET
==================

aiming at something pretty ambitious here, so I’ll give you a minimal but realistic Go project layout that:

Runs a local agent exposing a Docker-compatible HTTP API

Proxies requests to a remote Docker daemon over SSH

Adds:

Automatic port forwarding for -p published ports

Local → remote file sync for bind mounts like -v ./src:/app

This won’t be production-ready, but it’s a solid starting skeleton you can extend.

I’ll structure it as:

Project layout

Key concepts

Core code files (simplified but working‑ish)

Notes on what’s stubbed / to be extended

2. High-level behavior
Agent listens locally on e.g. localhost:23750.

Docker CLI is pointed at DOCKER_HOST=tcp://localhost:23750.

Agent:

Parses incoming Docker API requests.

For POST /containers/create:

Parses HostConfig.PortBindings → sets up SSH port forwards.

Parses HostConfig.Binds → syncs local paths to remote temp dirs and rewrites.

For other endpoints → passes through to remote via Docker API over SSH.

You’ll need a remote Docker daemon reachable over SSH, typically:

SSH: user@remote-host

Docker: unix:///var/run/docker.sock on remote


This skeleton skips a bunch of hard parts you can fill in:

Robust SSH connection pooling and reuse.

Proper Docker API compatibility (streaming responses, hijacked connections, logs, exec, attach).

Full handling of nat.Port types and multi-port mappings.

Proper local/remote path detection across OSes (Windows vs Linux).

Real SFTP sync with incremental updates and ignoring patterns.

Tracking port forwards per container and closing them on stop.

Mapping container IDs/events from remote back to local semantics.

Security hardening: host key verification, restricted remotes, auth.

Multi-tenant mapping: context → remote host → SSH key → Docker daemon.

But the structure above is enough to:

Compile.

Serve as a base to iterate on.

Let you plug in your actual overlay logic, sync implementation, and policies.