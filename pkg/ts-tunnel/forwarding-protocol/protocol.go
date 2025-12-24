// Package forwarding_protocol provides protocol handlers for connecting to remote endpoints
// via tstunnel (mTLS-enabled TCP) transport for forwarding operations.
package forwarding_protocol

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
	tstunneltransport "github.com/teamycloud/tsctl/pkg/ts-tunnel/agent-transport"
)

// protocolHandler implements the forwarding.ProtocolHandler interface for
// connecting to remote endpoints over tstunnel (mTLS-enabled TCP). It uses
// the agent infrastructure over a tstunnel transport.
type protocolHandler struct{}

// dialResult provides asynchronous agent dialing results.
type dialResult struct {
	// stream is the stream returned by agent dialing.
	stream io.ReadWriteCloser
	// error is the error returned by agent dialing.
	error error
}

// Connect connects to a tstunnel endpoint.
func (p *protocolHandler) Connect(
	ctx context.Context,
	logger *logging.Logger,
	url *urlpkg.URL,
	prompter string,
	session string,
	version forwarding.Version,
	configuration *forwarding.Configuration,
	source bool,
) (forwarding.Endpoint, error) {
	// Verify that the URL is of the correct kind and protocol.
	if url.Kind != urlpkg.Kind_Forwarding {
		panic("non-forwarding URL dispatched to forwarding protocol handler")
	}
	// Note: Protocol check would go here once tstunnel is added to Protocol enum

	// Parse the target specification from the URL's Path component.
	protocol, address, err := forwardingurlpkg.Parse(url.Path)
	if err != nil {
		return nil, fmt.Errorf("unable to parse target specification: %w", err)
	}

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
		stream, err := agent.Dial(logger, transport, agent.CommandForwarder, prompter)

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
	return remote.NewEndpoint(logger, stream, version, configuration, protocol, address, source)
}

// Note: Protocol registration would be done in init() once Protocol_Tstunnel
// is added to the Protocol enum in pkg/url/url.proto:
//
// func init() {
// 	forwarding.ProtocolHandlers[urlpkg.Protocol_Tstunnel] = &protocolHandler{}
// }
