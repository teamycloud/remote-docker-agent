package tstunnel

import (
	"context"
	"fmt"
	"io"

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
	endpoint string
	certPath string
	keyPath  string
	caPath   string
	sniHost  string
}

// NewForwardingProtocolHandler creates a new forwarding protocol handler for tstunnel
func NewForwardingProtocolHandler(endpoint, certPath, keyPath, caPath, sniHost string) *ForwardingProtocolHandler {
	return &ForwardingProtocolHandler{
		endpoint: endpoint,
		certPath: certPath,
		keyPath:  keyPath,
		caPath:   caPath,
		sniHost:  sniHost,
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

	// Create a tstunnel transport
	transport, err := NewTransport(h.endpoint, h.certPath, h.keyPath, h.caPath, h.sniHost, logger)
	if err != nil {
		return nil, fmt.Errorf("unable to create tstunnel transport: %w", err)
	}

	// Create a channel to deliver the dialing result.
	results := make(chan agentDialResult)

	// Perform dialing in a background Goroutine so that we can monitor for
	// cancellation.
	go func() {
		// Perform the dialing operation.
		stream, err := agent.Dial(logger, transport, agent.CommandForwarder, prompter)

		// Transmit the result or, if cancelled, close the stream.
		select {
		case results <- agentDialResult{stream, err}:
		case <-ctx.Done():
			if stream != nil {
				stream.Close()
			}
		}
	}()

	// Wait for dialing results or cancellation.
	var stream io.ReadWriteCloser
	select {
	case result := <-results:
		if result.error != nil {
			return nil, fmt.Errorf("unable to dial agent endpoint: %w", result.error)
		}
		stream = result.stream
	case <-ctx.Done():
		return nil, context.Canceled
	}

	// Create the endpoint.
	return remote.NewEndpoint(logger, stream, version, configuration, protocol, address, source)
}
