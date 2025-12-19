package tcp_agent

// Config holds configuration for the TCP agent
type Config struct {
	ListenAddr    string // Local address to listen on (e.g., "127.0.0.1:2375")
	RemoteAddress string // Remote Docker socket (e.g., "unix:///var/run/docker.sock")
}
