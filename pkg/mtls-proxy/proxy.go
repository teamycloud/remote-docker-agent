package mtlsproxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Proxy represents the mTLS TCP proxy server
type Proxy struct {
	config   *Config
	caPool   *x509.CertPool
	db       *DatabaseProvider
	logger   *logrus.Logger
	listener net.Listener
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewProxy creates a new mTLS proxy instance
func NewProxy(config *Config, logger *logrus.Logger) (*Proxy, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Load CA certificates
	caPool, err := config.LoadCACertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificates: %w", err)
	}

	// Connect to database
	db, err := NewDatabaseProvider(&config.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Proxy{
		config: config,
		caPool: caPool,
		db:     db,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start starts the proxy server
func (p *Proxy) Start() error {
	// Load server certificate
	cert, err := tls.LoadX509KeyPair(p.config.ServerCertPath, p.config.ServerKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    p.caPool,
		MinVersion:   tls.VersionTLS12,
	}

	// Create TLS listener
	listener, err := tls.Listen("tcp", p.config.ListenAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	p.listener = listener
	p.logger.Infof("mTLS proxy listening on %s", p.config.ListenAddr)

	// Accept connections
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.acceptConnections()
	}()

	return nil
}

// acceptConnections accepts incoming connections
func (p *Proxy) acceptConnections() {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			select {
			case <-p.ctx.Done():
				return
			default:
				p.logger.Errorf("failed to accept connection: %v", err)
				continue
			}
		}

		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.handleConnection(conn)
		}()
	}
}

// handleConnection handles a single client connection
func (p *Proxy) handleConnection(conn net.Conn) {
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		p.logger.Error("connection is not a TLS connection")
		return
	}

	// Get client certificate
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		p.logger.Error("no client certificate provided")
		return
	}

	clientCert := state.PeerCertificates[0]

	// Validate certificate (already done by TLS, but we do additional checks)
	if err := ValidateCertificate(clientCert, p.caPool); err != nil {
		p.logger.Errorf("certificate validation failed: %v", err)
		return
	}

	// Validate issuer match
	if err := ValidateIssuerMatch(clientCert, p.caPool, p.config.Issuer); err != nil {
		p.logger.Errorf("issuer validation failed: %v", err)
		return
	}

	// Extract user identity
	identity, err := ExtractUserIdentity(clientCert, p.config.Issuer)
	if err != nil {
		p.logger.Errorf("failed to extract user identity: %v", err)
		return
	}

	p.logger.Infof("authenticated user: %s (org: %s)", identity.UserID, identity.OrgID)

	// Read the connect_id from the client
	// The client should send the connect_id as the first message
	// Format: <connect_id>\n
	connectID, err := p.readConnectID(tlsConn)
	if err != nil {
		p.logger.Errorf("failed to read connect_id: %v", err)
		return
	}

	p.logger.Infof("routing connection to: %s", connectID)

	// Route the connection
	ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer cancel()

	target, err := p.db.RouteConnection(ctx, identity.UserID, identity.OrgID, connectID)
	if err != nil {
		p.logger.Errorf("routing failed: %v", err)
		p.sendError(tlsConn, fmt.Sprintf("routing failed: %v", err))
		return
	}

	p.logger.Infof("routing user %s to backend %s", identity.UserID, target.BackendAddr)

	// Connect to backend
	if err := p.proxyToBackend(tlsConn, target.BackendAddr); err != nil {
		p.logger.Errorf("proxy failed: %v", err)
		return
	}
}

// readConnectID reads the connect_id from the client
func (p *Proxy) readConnectID(conn net.Conn) (string, error) {
	// Set read deadline
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return "", err
	}
	defer conn.SetReadDeadline(time.Time{})

	// Read until newline
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}

	// Parse connect_id
	connectID := string(buf[:n])
	// Remove trailing newline
	if len(connectID) > 0 && connectID[len(connectID)-1] == '\n' {
		connectID = connectID[:len(connectID)-1]
	}
	// Remove trailing carriage return
	if len(connectID) > 0 && connectID[len(connectID)-1] == '\r' {
		connectID = connectID[:len(connectID)-1]
	}

	if connectID == "" {
		return "", errors.New("empty connect_id")
	}

	return connectID, nil
}

// sendError sends an error message to the client
func (p *Proxy) sendError(conn net.Conn, message string) {
	errMsg := fmt.Sprintf("ERROR: %s\n", message)
	conn.Write([]byte(errMsg))
}

// proxyToBackend proxies the connection to the backend server
func (p *Proxy) proxyToBackend(clientConn net.Conn, backendAddr string) error {
	// Connect to backend
	backendConn, err := net.DialTimeout("tcp", backendAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to backend %s: %w", backendAddr, err)
	}
	defer backendConn.Close()

	// Send success message to client
	if _, err := clientConn.Write([]byte("OK\n")); err != nil {
		return fmt.Errorf("failed to send OK to client: %w", err)
	}

	// Bidirectional copy
	errChan := make(chan error, 2)

	// Client -> Backend
	go func() {
		_, err := io.Copy(backendConn, clientConn)
		errChan <- err
	}()

	// Backend -> Client
	go func() {
		_, err := io.Copy(clientConn, backendConn)
		errChan <- err
	}()

	// Wait for either direction to complete
	err = <-errChan

	// Close both connections to terminate the other goroutine
	clientConn.Close()
	backendConn.Close()

	// Wait for the second goroutine
	<-errChan

	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("proxy error: %w", err)
	}

	return nil
}

// Stop stops the proxy server
func (p *Proxy) Stop() error {
	p.cancel()

	if p.listener != nil {
		if err := p.listener.Close(); err != nil {
			p.logger.Errorf("failed to close listener: %v", err)
		}
	}

	// Wait for all connections to finish (with timeout)
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("all connections closed gracefully")
	case <-time.After(30 * time.Second):
		p.logger.Warn("timeout waiting for connections to close")
	}

	if p.db != nil {
		p.db.Close()
	}

	return nil
}
