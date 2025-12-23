package tstunnel

import (
	"context"
	"fmt"

	"github.com/mutagen-io/mutagen/pkg/agent"
	"github.com/mutagen-io/mutagen/pkg/logging"
	"github.com/mutagen-io/mutagen/pkg/synchronization"
	"github.com/mutagen-io/mutagen/pkg/synchronization/endpoint/remote"
	urlpkg "github.com/mutagen-io/mutagen/pkg/url"
)

// SyncProtocolHandler implements the synchronization.ProtocolHandler interface for
// connecting to remote endpoints over mTLS with HTTP UPGRADE (tstunnel).
type SyncProtocolHandler struct {
	config protocolHandlerConfig
}

// NewSyncProtocolHandler creates a new synchronization protocol handler for tstunnel
func NewSyncProtocolHandler(endpoint, certPath, keyPath, caPath, sniHost string) *SyncProtocolHandler {
	return &SyncProtocolHandler{
		config: protocolHandlerConfig{
			endpoint: endpoint,
			certPath: certPath,
			keyPath:  keyPath,
			caPath:   caPath,
			sniHost:  sniHost,
		},
	}
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
	// Dial the agent asynchronously
	stream, err := dialAgentAsync(ctx, logger, &h.config, agent.CommandSynchronizer, prompter)
	if err != nil {
		return nil, err
	}

	// Create the endpoint client.
	return remote.NewEndpoint(logger, stream, url.Path, session, version, configuration, alpha)
}
