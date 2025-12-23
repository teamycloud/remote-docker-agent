package tstunnel

import (
	"context"
	"fmt"
	"io"

	"github.com/mutagen-io/mutagen/pkg/agent"
	"github.com/mutagen-io/mutagen/pkg/logging"
	"github.com/mutagen-io/mutagen/pkg/synchronization"
	"github.com/mutagen-io/mutagen/pkg/synchronization/endpoint/remote"
	urlpkg "github.com/mutagen-io/mutagen/pkg/url"
)

// SyncProtocolHandler implements the synchronization.ProtocolHandler interface for
// connecting to remote endpoints over mTLS with HTTP UPGRADE (tstunnel).
type SyncProtocolHandler struct {
	endpoint string
	certPath string
	keyPath  string
	caPath   string
	sniHost  string
}

// NewSyncProtocolHandler creates a new synchronization protocol handler for tstunnel
func NewSyncProtocolHandler(endpoint, certPath, keyPath, caPath, sniHost string) *SyncProtocolHandler {
	return &SyncProtocolHandler{
		endpoint: endpoint,
		certPath: certPath,
		keyPath:  keyPath,
		caPath:   caPath,
		sniHost:  sniHost,
	}
}

// syncDialResult provides asynchronous agent dialing results for synchronization.
type syncDialResult struct {
	// stream is the stream returned by agent dialing.
	stream io.ReadWriteCloser
	// error is the error returned by agent dialing.
	error error
}

// Connect connects to a tstunnel endpoint for synchronization
func (h *SyncProtocolHandler) Connect(
	ctx context.Context,
	logger *logging.Logger,
	url *urlpkg.URL,
	prompter string,
	session string,
	version synchronization.Version,
	configuration *synchronization.Configuration,
	alpha bool,
) (synchronization.Endpoint, error) {
	// Create a tstunnel transport
	transport, err := NewTransport(h.endpoint, h.certPath, h.keyPath, h.caPath, h.sniHost, logger)
	if err != nil {
		return nil, fmt.Errorf("unable to create tstunnel transport: %w", err)
	}

	// Create a channel to deliver the dialing result.
	results := make(chan syncDialResult)

	// Perform dialing in a background Goroutine so that we can monitor for
	// cancellation.
	go func() {
		// Perform the dialing operation.
		stream, err := agent.Dial(logger, transport, agent.CommandSynchronizer, prompter)

		// Transmit the result or, if cancelled, close the stream.
		select {
		case results <- syncDialResult{stream, err}:
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

	// Create the endpoint client.
	return remote.NewEndpoint(logger, stream, url.Path, session, version, configuration, alpha)
}
