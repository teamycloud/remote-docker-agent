// Package ts_tunnel provides an agent transport implementation that operates
// over mTLS-enabled TCP connections. This transport is designed to work with
// a server that provides connection upgrade capabilities via HTTP UPGRADE to
// establish a raw TCP stream suitable for mutagen agent communication.
//
// The transport supports:
// - mTLS authentication using client certificates
// - SNI-based routing to different remote hosts
// - HTTP UPGRADE to establish bidirectional TCP streams
// - Standard mutagen agent operations (command execution and file copying)
//
// Note: Unlike SSH, this transport does not provide direct process execution
// capabilities. Instead, it relies on a server-side component that maintains
// persistent agent connections and routes commands accordingly.
package ts_tunnel
