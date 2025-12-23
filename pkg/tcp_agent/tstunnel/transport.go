package tstunnel

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"github.com/mutagen-io/mutagen/pkg/agent"
	"github.com/mutagen-io/mutagen/pkg/logging"
)

// Transport implements a custom transport for mutagen that uses HTTP UPGRADE
// over mTLS to establish TCP tunnels to Tinyscale servers
type Transport struct {
	endpoint string // mTLS endpoint (e.g., "gateway.tinyscale.net:443")
	certPath string // Client certificate path
	keyPath  string // Client private key path
	caPath   string // CA certificate path (optional)
	sniHost  string // SNI hostname
	logger   *logging.Logger
}

// NewTransport creates a new tstunnel transport
func NewTransport(endpoint, certPath, keyPath, caPath, sniHost string, logger *logging.Logger) (*Transport, error) {
	return &Transport{
		endpoint: endpoint,
		certPath: certPath,
		keyPath:  keyPath,
		caPath:   caPath,
		sniHost:  sniHost,
		logger:   logger,
	}, nil
}

// Dial establishes a connection to the remote agent via HTTP UPGRADE over mTLS
func (t *Transport) Dial(command agent.Command) (io.ReadWriteCloser, error) {
	// Load client certificate and key
	cert, err := tls.LoadX509KeyPair(t.certPath, t.keyPath)
	if err != nil {
		return nil, fmt.Errorf("load client certificate: %w", err)
	}

	// Load CA certificate if provided
	var rootCAs *x509.CertPool
	if t.caPath != "" {
		caCert, err := os.ReadFile(t.caPath)
		if err != nil {
			return nil, fmt.Errorf("read CA certificate: %w", err)
		}

		rootCAs = x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      rootCAs,
		ServerName:   t.sniHost,
		MinVersion:   tls.VersionTLS12,
	}

	// Dial the mTLS endpoint
	conn, err := tls.Dial("tcp", t.endpoint, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("mtls dial: %w", err)
	}

	// Determine the API path based on the command type
	var apiPath string
	switch command {
	case agent.CommandForwarder:
		apiPath = "/tinyscale/v1/tunnel/forward"
	case agent.CommandSynchronizer:
		apiPath = "/tinyscale/v1/tunnel/sync"
	default:
		conn.Close()
		return nil, fmt.Errorf("unsupported agent command: %v", command)
	}

	// Send HTTP UPGRADE request to establish TCP tunnel
	req, err := http.NewRequest("GET", apiPath, nil)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create upgrade request: %w", err)
	}

	// Set required headers for HTTP UPGRADE
	req.Host = t.sniHost
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "tcp")
	req.Header.Set("X-Tinyscale-Command", string(command))

	// Write the request to the connection
	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("write upgrade request: %w", err)
	}

	// Read the HTTP response
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read upgrade response: %w", err)
	}
	defer resp.Body.Close()

	// Check if the upgrade was successful
	if resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return nil, fmt.Errorf("upgrade failed with status: %d %s", resp.StatusCode, resp.Status)
	}

	// Check if the connection was upgraded to TCP
	if resp.Header.Get("Upgrade") != "tcp" {
		conn.Close()
		return nil, fmt.Errorf("upgrade response missing 'Upgrade: tcp' header")
	}

	t.logger.Info("Successfully established TCP tunnel via HTTP UPGRADE")

	// Now the connection is upgraded to a raw TCP tunnel
	// We need to wrap it to handle any buffered data
	return &upgradedConn{
		Conn:   conn,
		reader: reader,
	}, nil
}

// Copy implements the Transport.Copy method (optional for some transports)
func (t *Transport) Copy() agent.Transport {
	return &Transport{
		endpoint: t.endpoint,
		certPath: t.certPath,
		keyPath:  t.keyPath,
		caPath:   t.caPath,
		sniHost:  t.sniHost,
		logger:   t.logger,
	}
}

// upgradedConn wraps a net.Conn and bufio.Reader to handle buffered data
// after HTTP UPGRADE
type upgradedConn struct {
	net.Conn
	reader *bufio.Reader
}

// Read reads from the buffered reader first, then from the underlying connection
func (u *upgradedConn) Read(p []byte) (int, error) {
	// If there's buffered data, read from it first
	if u.reader != nil && u.reader.Buffered() > 0 {
		return u.reader.Read(p)
	}
	// Otherwise, read directly from the connection
	// After this point, we can bypass the reader
	if u.reader != nil {
		u.reader = nil
	}
	return u.Conn.Read(p)
}

// Dialer creates a net.Dialer that uses the tstunnel transport
// This can be used for port forwarding
func (t *Transport) Dialer(ctx context.Context) (net.Conn, error) {
	// Load client certificate and key
	cert, err := tls.LoadX509KeyPair(t.certPath, t.keyPath)
	if err != nil {
		return nil, fmt.Errorf("load client certificate: %w", err)
	}

	// Load CA certificate if provided
	var rootCAs *x509.CertPool
	if t.caPath != "" {
		caCert, err := os.ReadFile(t.caPath)
		if err != nil {
			return nil, fmt.Errorf("read CA certificate: %w", err)
		}

		rootCAs = x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      rootCAs,
		ServerName:   t.sniHost,
		MinVersion:   tls.VersionTLS12,
	}

	// Dial the mTLS endpoint
	conn, err := tls.Dial("tcp", t.endpoint, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("mtls dial: %w", err)
	}

	// Send HTTP UPGRADE request to establish TCP tunnel for port forwarding
	req, err := http.NewRequest("GET", "/tinyscale/v1/tunnel/port", nil)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create upgrade request: %w", err)
	}

	// Set required headers for HTTP UPGRADE
	req.Host = t.sniHost
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "tcp")

	// Write the request to the connection
	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("write upgrade request: %w", err)
	}

	// Read the HTTP response
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read upgrade response: %w", err)
	}
	defer resp.Body.Close()

	// Check if the upgrade was successful
	if resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return nil, fmt.Errorf("upgrade failed with status: %d %s", resp.StatusCode, resp.Status)
	}

	// Check if the connection was upgraded to TCP
	if resp.Header.Get("Upgrade") != "tcp" {
		conn.Close()
		return nil, fmt.Errorf("upgrade response missing 'Upgrade: tcp' header")
	}

	t.logger.Info("Successfully established TCP tunnel for port forwarding via HTTP UPGRADE")

	// Now the connection is upgraded to a raw TCP tunnel
	return &upgradedConn{
		Conn:   conn,
		reader: reader,
	}, nil
}
