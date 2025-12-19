package transparent_ssh_agent

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHClient manages SSH connection to remote host
type SSHClient struct {
	cfg    Config
	client *ssh.Client
}

// NewSSHClient creates a new SSH client connection
func NewSSHClient(cfg Config) (*SSHClient, error) {
	key, err := os.ReadFile(cfg.SSHKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read ssh key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("parse ssh key: %w", err)
	}

	sshCfg := &ssh.ClientConfig{
		User: cfg.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: verify host key
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", cfg.SSHHost, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial: %w", err)
	}

	return &SSHClient{
		cfg:    cfg,
		client: client,
	}, nil
}

// DialRemoteDocker dials the remote Docker socket via SSH tunnel
func (s *SSHClient) DialRemoteDocker() (net.Conn, error) {
	u, err := url.Parse(s.cfg.RemoteDockerURL)
	if err != nil {
		return nil, fmt.Errorf("parse remote docker url: %w", err)
	}

	var network, address string
	switch u.Scheme {
	case "unix":
		network = "unix"
		address = u.Path
	case "tcp":
		network = "tcp"
		address = u.Host
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	conn, err := s.client.Dial(network, address)
	if err != nil {
		return nil, fmt.Errorf("ssh dial remote docker: %w", err)
	}

	return conn, nil
}

// Close closes the SSH client connection
func (s *SSHClient) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
