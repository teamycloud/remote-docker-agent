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

import (
	"github.com/mutagen-io/mutagen/pkg/forwarding"
	"github.com/mutagen-io/mutagen/pkg/synchronization"
	urlpkg "github.com/mutagen-io/mutagen/pkg/url"
	forwardingprotocol "github.com/teamycloud/tsctl/pkg/ts-tunnel/forwarding-protocol"
	synchronizationprotocol "github.com/teamycloud/tsctl/pkg/ts-tunnel/synchronization-protocol"
)

const (
	// Protocol_Tstunnel is a custom protocol value for ts-tunnel transport.
	// We use value 100 to avoid conflicts with mutagen's built-in protocols (0, 1, 11).
	Protocol_Tstunnel urlpkg.Protocol = 100
)

// init registers the ts-tunnel protocol handlers with mutagen.
// This must be called before using ts-tunnel transport for forwarding or synchronization.
func init() {
	// Register the ts-tunnel forwarding protocol handler
	forwarding.ProtocolHandlers[Protocol_Tstunnel] = &forwardingprotocol.ProtocolHandler{}

	// Register the ts-tunnel synchronization protocol handler
	synchronization.ProtocolHandlers[Protocol_Tstunnel] = &synchronizationprotocol.ProtocolHandler{}
}
