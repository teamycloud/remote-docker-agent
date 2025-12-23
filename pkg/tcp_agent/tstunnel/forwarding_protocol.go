package tstunnel

import (
	"context"
	"fmt"

	"github.com/mutagen-io/mutagen/pkg/agent"
	"github.com/mutagen-io/mutagen/pkg/forwarding"
	"github.com/mutagen-io/mutagen/pkg/forwarding/endpoint/remote"
	"github.com/mutagen-io/mutagen/pkg/logging"
	urlpkg "github.com/mutagen-io/mutagen/pkg/url"
	forwardingurlpkg "github.com/mutagen-io/mutagen/pkg/url/forwarding"
)

// ForwardingProtocolHandler implements the forwarding.ProtocolHandler interface for
// connecting to remote endpoints over mTLS with HTTP UPGRADE (tstunnel).
type ForwardingProtocolHandler struct {
	config protocolHandlerConfig
}

// NewForwardingProtocolHandler creates a new forwarding protocol handler for tstunnel
func NewForwardingProtocolHandler(endpoint, certPath, keyPath, caPath, sniHost string) *ForwardingProtocolHandler {
	return &ForwardingProtocolHandler{
		config: protocolHandlerConfig{
			endpoint: endpoint,
			certPath: certPath,
			keyPath:  keyPath,
			caPath:   caPath,
			sniHost:  sniHost,
		},
	}
}

// Connect connects to a tstunnel endpoint for forwarding
func (h *ForwardingProtocolHandler) Connect(
	ctx context.Context,
	logger *logging.Logger,
	url *urlpkg.URL,
	prompter string,
	session string,
	version forwarding.Version,
	configuration *forwarding.Configuration,
	source bool,
) (forwarding.Endpoint, error) {
	// Parse the target specification from the URL's Path component.
	protocol, address, err := forwardingurlpkg.Parse(url.Path)
	if err != nil {
		return nil, fmt.Errorf("unable to parse target specification: %w", err)
	}

	// Dial the agent asynchronously
	stream, err := dialAgentAsync(ctx, logger, &h.config, agent.CommandForwarder, prompter)
	if err != nil {
		return nil, err
	}

	// Create the endpoint.
	return remote.NewEndpoint(logger, stream, version, configuration, protocol, address, source)
}
