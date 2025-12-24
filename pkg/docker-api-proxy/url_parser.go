package docker_api_proxy

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	urlpkg "github.com/mutagen-io/mutagen/pkg/url"
	"github.com/teamycloud/tsctl/pkg/ts-tunnel"
)

// ParseTSTunnelURL parses a tstunnel:// URL and converts it to a mutagen URL.
// Format: tstunnel://<hostid>/<path>?endpoint=<endpoint>&cert=<cert>&key=<key>[&ca=<ca>]
func ParseTSTunnelURL(rawURL string, kind urlpkg.Kind) (*urlpkg.URL, error) {
	// Check if this is a tstunnel URL
	if !strings.HasPrefix(rawURL, "tstunnel://") {
		// Not a tstunnel URL, parse normally
		return urlpkg.Parse(rawURL, kind, true)
	}

	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tstunnel URL: %w", err)
	}

	// Extract parameters from query string
	query := parsedURL.Query()
	params := make(map[string]string)
	for key, values := range query {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}

	// Validate required parameters
	endpoint := params["endpoint"]
	if endpoint == "" {
		return nil, fmt.Errorf("tstunnel URL missing required 'endpoint' parameter")
	}
	// cert and key are optional - omit them for insecure dev/debug scenarios
	// If one is provided, both must be provided
	certFile := params["cert"]
	keyFile := params["key"]
	if (certFile != "" && keyFile == "") || (certFile == "" && keyFile != "") {
		return nil, fmt.Errorf("tstunnel URL requires both 'cert' and 'key' parameters or neither")
	}

	port := parsedURL.Port()
	if port == "" {
		useTLS := params["cert"] != "" && params["key"] != ""
		if useTLS {
			port = "443"
		} else {
			port = "80"
		}
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		portNumber = 80
	}

	mutagenURL := &urlpkg.URL{
		Kind:       kind,
		Protocol:   ts_tunnel.Protocol_Tstunnel,
		Host:       parsedURL.Host,       // This is the hostID
		Port:       (uint32)(portNumber), // This is the hostID
		Path:       parsedURL.Path,
		Parameters: params,
	}

	return mutagenURL, nil
}
