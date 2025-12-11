package agent

import (
    "fmt"
    "io"
    "net"
    "os"
    "time"

    "golang.org/x/crypto/ssh"
)

type SSHClient struct {
    cfg    Config
    client *ssh.Client
}

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

// DialRemoteDocker dials the remote Docker socket via SSH.
func (s *SSHClient) DialRemoteDocker() (net.Conn, error) {
    // For unix socket on remote, use ssh's direct-streamlocal; here we'll
    // approximate by running "socat" if available, or use a simple TCP -> unix
    // bridge. To keep it simple, assume Docker also listens on tcp://127.0.0.1:2375.
    // You can improve this later by using "unix" support in ssh.
    conn, err := s.client.Dial("tcp", "127.0.0.1:2375")
    if err != nil {
        return nil, fmt.Errorf("ssh dial docker tcp: %w", err)
    }
    return conn, nil
}

// StartRemotePortForward sets up remote→local or local→remote tunnel.
// For now, a simple local listener that dials remote host via SSH.
func (s *SSHClient) StartLocalForward(localAddr, remoteAddr string) (net.Listener, error) {
    ln, err := net.Listen("tcp", localAddr)
    if err != nil {
        return nil, fmt.Errorf("listen local: %w", err)
    }

    go func() {
        for {
            lc, err := ln.Accept()
            if err != nil {
                return
            }
            go func(c net.Conn) {
                defer c.Close()
                rc, err := s.client.Dial("tcp", remoteAddr)
                if err != nil {
                    return
                }
                defer rc.Close()
                go io.Copy(rc, c)
                io.Copy(c, rc)
            }(lc)
        }
    }()

    return ln, nil
}
