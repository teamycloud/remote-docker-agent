package transparent_ssh_agent

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
)

// DockerTCPProxy is a Docker-aware TCP proxy that can intercept and modify
// Docker API requests/responses
type DockerTCPProxy struct {
	cfg       Config
	sshClient *SSHClient
	listener  net.Listener
	wg        sync.WaitGroup
	stopCh    chan struct{}

	// Hooks for intercepting Docker API calls
	beforeRequest func(*http.Request) error
	afterResponse func(*http.Response) error
}

// NewDockerTCPProxy creates a new Docker-aware TCP proxy
func NewDockerTCPProxy(cfg Config) (*DockerTCPProxy, error) {
	sshClient, err := NewSSHClient(cfg)
	if err != nil {
		return nil, err
	}

	return &DockerTCPProxy{
		cfg:       cfg,
		sshClient: sshClient,
		stopCh:    make(chan struct{}),
	}, nil
}

// SetBeforeRequestHook sets a hook to be called before forwarding requests
func (p *DockerTCPProxy) SetBeforeRequestHook(hook func(*http.Request) error) {
	p.beforeRequest = hook
}

// SetAfterResponseHook sets a hook to be called after receiving responses
func (p *DockerTCPProxy) SetAfterResponseHook(hook func(*http.Response) error) {
	p.afterResponse = hook
}

// ListenAndServe starts the Docker TCP proxy server
func (p *DockerTCPProxy) ListenAndServe() error {
	listener, err := net.Listen("tcp", p.cfg.ListenAddr)
	if err != nil {
		return err
	}
	p.listener = listener

	log.Printf("Docker TCP proxy listening on %s, proxying to %s via SSH", p.cfg.ListenAddr, p.cfg.RemoteDockerURL)

	for {
		select {
		case <-p.stopCh:
			return nil
		default:
		}

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

		p.wg.Add(1)
		go p.handleDockerConnection(clientConn)
	}
}

// handleDockerConnection handles a Docker API connection with HTTP awareness
func (p *DockerTCPProxy) handleDockerConnection(clientConn net.Conn) {
	defer p.wg.Done()
	defer clientConn.Close()

	remoteConn, err := p.sshClient.DialRemoteDocker()
	if err != nil {
		log.Printf("Failed to dial remote Docker: %v", err)
		return
	}
	defer remoteConn.Close()

	log.Printf("New Docker connection from %s", clientConn.RemoteAddr())

	// Create buffered readers/writers for HTTP parsing
	clientReader := bufio.NewReader(clientConn)
	remoteReader := bufio.NewReader(remoteConn)

	// Handle the connection - attempt HTTP parsing, fall back to transparent proxy
	errCh := make(chan error, 2)

	// Client -> Remote (with HTTP interception)
	go func() {
		errCh <- p.proxyClientToRemote(clientReader, clientConn, remoteConn)
	}()

	// Remote -> Client (with HTTP interception)
	go func() {
		errCh <- p.proxyRemoteToClient(remoteReader, remoteConn, clientConn)
	}()

	err = <-errCh
	if err != nil && err != io.EOF {
		log.Printf("Docker connection error: %v", err)
	}

	log.Printf("Docker connection closed from %s", clientConn.RemoteAddr())
}

// proxyClientToRemote forwards client requests to remote, intercepting HTTP
func (p *DockerTCPProxy) proxyClientToRemote(reader *bufio.Reader, clientConn, remoteConn net.Conn) error {
	for {
		// Try to parse as HTTP request
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err == io.EOF {
				return io.EOF
			}
			// Not HTTP or malformed - fall back to transparent copy
			log.Printf("Failed to parse HTTP request, falling back to transparent mode: %v", err)
			_, copyErr := io.Copy(remoteConn, reader)
			return copyErr
		}

		// Log the request
		log.Printf("Request: %s %s", req.Method, req.URL.Path)

		// Call before hook if set
		if p.beforeRequest != nil {
			if err := p.beforeRequest(req); err != nil {
				log.Printf("Before request hook error: %v", err)
				return err
			}
		}

		// Forward the request to remote
		if err := req.Write(remoteConn); err != nil {
			return fmt.Errorf("write request to remote: %w", err)
		}
	}
}

// proxyRemoteToClient forwards remote responses to client, intercepting HTTP
func (p *DockerTCPProxy) proxyRemoteToClient(reader *bufio.Reader, remoteConn, clientConn net.Conn) error {
	for {
		// Try to parse as HTTP response
		resp, err := http.ReadResponse(reader, nil)
		if err != nil {
			if err == io.EOF {
				return io.EOF
			}
			// Not HTTP or malformed - fall back to transparent copy
			log.Printf("Failed to parse HTTP response, falling back to transparent mode: %v", err)
			_, copyErr := io.Copy(clientConn, reader)
			return copyErr
		}

		// Log the response
		log.Printf("Response: %d %s", resp.StatusCode, resp.Status)

		// Call after hook if set
		if p.afterResponse != nil {
			if err := p.afterResponse(resp); err != nil {
				log.Printf("After response hook error: %v", err)
				return err
			}
		}

		// Forward the response to client
		if err := resp.Write(clientConn); err != nil {
			return fmt.Errorf("write response to client: %w", err)
		}
	}
}

// Close gracefully shuts down the proxy
func (p *DockerTCPProxy) Close() error {
	close(p.stopCh)

	if p.listener != nil {
		p.listener.Close()
	}

	p.wg.Wait()

	if p.sshClient != nil {
		return p.sshClient.Close()
	}

	return nil
}

// InterceptCreateContainer is a helper to intercept container creation
func (p *DockerTCPProxy) InterceptCreateContainer() {
	p.SetBeforeRequestHook(func(req *http.Request) error {
		// Check if this is a container create request
		if req.Method == "POST" && strings.HasPrefix(req.URL.Path, "/v") &&
			strings.Contains(req.URL.Path, "/containers/create") {

			// Read the request body
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return fmt.Errorf("read request body: %w", err)
			}
			req.Body.Close()

			log.Printf("Intercepted container create request: %s", string(body))

			// TODO: Modify the request body here (e.g., adjust port mappings, volumes)
			// For now, just pass through

			// Restore the body
			req.Body = io.NopCloser(bytes.NewReader(body))
			req.ContentLength = int64(len(body))
		}
		return nil
	})
}
