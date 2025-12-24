// Package synchronization_protocol provides protocol handlers for connecting to remote endpoints
// via tstunnel (mTLS-enabled TCP) transport for synchronization operations.
package synchronization_protocol

import (
	"context"
	"fmt"
	"io"

	"github.com/mutagen-io/mutagen/pkg/agent"
	"github.com/mutagen-io/mutagen/pkg/logging"
	"github.com/mutagen-io/mutagen/pkg/synchronization"
	"github.com/mutagen-io/mutagen/pkg/synchronization/endpoint/remote"
	urlpkg "github.com/mutagen-io/mutagen/pkg/url"
	tstunneltransport "github.com/teamycloud/tsctl/pkg/ts-tunnel/agent-transport"
)

// ProtocolHandler implements the synchronization.ProtocolHandler interface for
// connecting to remote endpoints over tstunnel (mTLS-enabled TCP). It uses
// the agent infrastructure over a tstunnel transport.
type ProtocolHandler struct{}

// dialResult provides asynchronous agent dialing results.
type dialResult struct {
	// stream is the stream returned by agent dialing.
	stream io.ReadWriteCloser
	// error is the error returned by agent dialing.
	error error
}

// Connect connects to a tstunnel endpoint.
func (h *ProtocolHandler) Connect(
	ctx context.Context,
	logger *logging.Logger,
	url *urlpkg.URL,
	prompter string,
	session string,
	version synchronization.Version,
	configuration *synchronization.Configuration,
	alpha bool,
) (synchronization.Endpoint, error) {
	// Verify that the URL is of the correct kind and protocol.
	if url.Kind != urlpkg.Kind_Synchronization {
		panic("non-synchronization URL dispatched to synchronization protocol handler")
	}
	// Note: Protocol check would go here once tstunnel is added to Protocol enum

	// Extract tstunnel-specific parameters from URL.Parameters.
	// Expected parameters:
	// - endpoint: the HTTPS endpoint (e.g., "containers.tinyscale.net:443")
	// - cert: path to client certificate file
	// - key: path to client key file
	// - ca: path to CA certificate file (optional)

	endpoint := url.Parameters["endpoint"]
	if endpoint == "" {
		return nil, fmt.Errorf("tstunnel endpoint parameter is required")
	}

	certFile := url.Parameters["cert"]
	if certFile == "" {
		return nil, fmt.Errorf("tstunnel cert parameter is required")
	}

	keyFile := url.Parameters["key"]
	if keyFile == "" {
		return nil, fmt.Errorf("tstunnel key parameter is required")
	}

	// Optional parameters.
	caFile := url.Parameters["ca"]

	// Use url.Host as the host ID for SNI routing.
	hostID := url.Host
	if hostID == "" {
		return nil, fmt.Errorf("host identifier is required (use hostname component of URL)")
	}

	// Build TLS configuration.
	builder := tstunneltransport.NewTLSConfigBuilder().
		WithClientCertificate(certFile, keyFile)

	if caFile != "" {
		builder = builder.WithCACertificate(caFile)
	}

	tlsConfig, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("unable to create TLS configuration: %w", err)
	}

	// Create a tstunnel transport.
	transport, err := tstunneltransport.NewTransport(tstunneltransport.TransportOptions{
		Endpoint:  endpoint,
		HostID:    hostID,
		TLSConfig: tlsConfig,
		CertFile:  certFile,
		KeyFile:   keyFile,
		CAFile:    caFile,
		Prompter:  prompter,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create tstunnel transport: %w", err)
	}

	// Create a channel to deliver the dialing result.
	results := make(chan dialResult)

	// Perform dialing in a background Goroutine so that we can monitor for
	// cancellation.
	go func() {
		// Perform the dialing operation.
		stream, err := agent.Dial(logger, transport, agent.CommandSynchronizer, prompter)

		// Transmit the result or, if cancelled, close the stream.
		select {
		case results <- dialResult{stream, err}:
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
	return remote.NewEndpoint(logger, stream, url.Path, session, version, configuration, alpha)
}

// Note: Protocol registration would be done in init() once Protocol_Tstunnel
// is added to the Protocol enum in pkg/url/url.proto:
//
// func init() {
// 	synchronization.ProtocolHandlers[urlpkg.Protocol_Tstunnel] = &protocolHandler{}
// }
