package tcp_agent

import (
	"io"
	"log"
	"net"
	"sync"
)

// TCPProxy implements a transparent TCP proxy that forwards connections
// through an SSH tunnel to a remote Docker daemon
type TCPProxy struct {
	cfg       Config
	tcpClient *net.Dialer
	listener  net.Listener
	wg        sync.WaitGroup
	stopCh    chan struct{}
}

// NewTCPProxy creates a new TCP proxy instance
func NewTCPProxy(cfg Config) (*TCPProxy, error) {
	tcpClient := &net.Dialer{}
	return &TCPProxy{
		cfg:       cfg,
		tcpClient: tcpClient,
		stopCh:    make(chan struct{}),
	}, nil
}

// ListenAndServe starts the TCP proxy server
func (p *TCPProxy) ListenAndServe() error {
	listener, err := net.Listen("tcp", p.cfg.ListenAddr)
	if err != nil {
		return err
	}
	p.listener = listener

	log.Printf("TCP proxy listening on %s, proxying to %s via SSH", p.cfg.ListenAddr, p.cfg.RemoteAddress)

	for {
		select {
		case <-p.stopCh:
			return nil
		default:
		}

		// Accept new connections
		clientConn, err := p.listener.Accept()
		if err != nil {
			select {
			case <-p.stopCh:
				return nil
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}

		// Handle each connection in a goroutine
		p.wg.Add(1)
		go p.handleConnection(clientConn)
	}
}

// handleConnection proxies data between client and remote Docker daemon
func (p *TCPProxy) handleConnection(clientConn net.Conn) {
	defer p.wg.Done()
	defer clientConn.Close()

	// Establish connection to remote Docker via SSH
	remoteConn, err := p.tcpClient.Dial("tcp", p.cfg.RemoteAddress)
	if err != nil {
		log.Printf("Failed to dial remote Docker: %v", err)
		return
	}
	defer remoteConn.Close()

	log.Printf("New connection from %s -> %s", clientConn.RemoteAddr(), p.cfg.RemoteAddress)

	// Bidirectional copy
	errCh := make(chan error, 2)

	// Client -> Remote
	go func() {
		_, err := io.Copy(remoteConn, clientConn)
		errCh <- err
	}()

	// Remote -> Client
	go func() {
		_, err := io.Copy(clientConn, remoteConn)
		errCh <- err
	}()

	// Wait for either direction to complete
	err = <-errCh
	if err != nil && err != io.EOF {
		log.Printf("Connection copy error: %v", err)
	}

	log.Printf("Connection closed from %s", clientConn.RemoteAddr())
}

// Close gracefully shuts down the proxy
func (p *TCPProxy) Close() error {
	close(p.stopCh)

	if p.listener != nil {
		p.listener.Close()
	}

	p.wg.Wait()

	return nil
}
