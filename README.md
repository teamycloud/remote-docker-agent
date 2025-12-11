
Remote Docker Agent
==================

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

Agent listens locally on e.g. localhost:23750.

Docker CLI is pointed at DOCKER_HOST=tcp://localhost:23750.

Agent parses incoming Docker API requests.

For POST /containers/create:

* Parses HostConfig.PortBindings → sets up SSH port forwards.

* Parses HostConfig.Binds → syncs local paths to remote temp dirs and rewrites.

For other endpoints → passes through to remote via Docker API over SSH.


You’ll need a remote Docker daemon reachable over SSH, typically:

SSH: user@remote-host

Docker: unix:///var/run/docker.sock on remote


## Roadmap

* Robust SSH connection pooling and reuse.

* Proper Docker API compatibility (streaming responses, hijacked connections, logs, exec, attach).

* Full handling of nat.Port types and multi-port mappings.

* Proper local/remote path detection across OSes (Windows vs Linux).

* Real SFTP sync with incremental updates and ignoring patterns.

* Tracking port forwards per container and closing them on stop.

* Mapping container IDs/events from remote back to local semantics.

* Security hardening: host key verification, restricted remotes, auth.

* Multi-tenant mapping: context → remote host → SSH key → Docker daemon.
