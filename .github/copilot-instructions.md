
```
remote-docker-agent/
  go.mod
  go.sum
  cmd/
    main.go
  pkg/
    agent/
      server.go          # HTTP server exposing Docker-like API
      router.go          # Routing/handlers
      ssh_client.go      # SSH tunnel + Docker API over SSH
      docker_proxy.go    # High-level Docker API proxy logic
      port_forward.go    # Local<->remote port forwarding helpers
      filesync.go        # Local->remote file sync for bind mounts
      types.go           # Shared types / config
```