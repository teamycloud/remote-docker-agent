package tstunnel

import (
	"context"
	"fmt"
	"io"

	"github.com/mutagen-io/mutagen/pkg/agent"
	"github.com/mutagen-io/mutagen/pkg/logging"
)

// agentDialResult provides asynchronous agent dialing results.
// This type is shared across forwarding and synchronization protocols.
type agentDialResult struct {
	// stream is the stream returned by agent dialing.
	stream io.ReadWriteCloser
	// error is the error returned by agent dialing.
	error error
}

// protocolHandlerConfig holds common configuration for protocol handlers
type protocolHandlerConfig struct {
	endpoint string
	certPath string
	keyPath  string
	caPath   string
	sniHost  string
}

// dialAgentAsync performs asynchronous agent dialing with cancellation support
func dialAgentAsync(
	ctx context.Context,
	logger *logging.Logger,
	config *protocolHandlerConfig,
	command agent.Command,
	prompter string,
) (io.ReadWriteCloser, error) {
	// Create a tstunnel transport
	transport, err := NewTransport(config.endpoint, config.certPath, config.keyPath, config.caPath, config.sniHost, logger)
	if err != nil {
		return nil, fmt.Errorf("unable to create tstunnel transport: %w", err)
	}

	// Create a channel to deliver the dialing result.
	results := make(chan agentDialResult)

	// Perform dialing in a background Goroutine so that we can monitor for
	// cancellation.
	go func() {
		// Perform the dialing operation.
		stream, err := agent.Dial(logger, transport, command, prompter)

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
	select {
	case result := <-results:
		if result.error != nil {
			return nil, fmt.Errorf("unable to dial agent endpoint: %w", result.error)
		}
		return result.stream, nil
	case <-ctx.Done():
		return nil, context.Canceled
	}
}
