package agent

import (
	"fmt"

	"github.com/docker/docker/api/types/container"
)

// Very simplified: we only care about host-level port bindings like "8080:80/tcp".
func (p *DockerProxy) setupPortForwards(hc *container.HostConfig) error {
	if len(hc.PortBindings) == 0 {
		return nil
	}

	for _, bindings := range hc.PortBindings {
		// port is nat.Port, but we only care about the container part (like "80/tcp").
		for _, b := range bindings {
			hostPort := b.HostPort
			if hostPort == "" {
				// Let Docker choose a random remote port; we can't forward that automatically.
				continue
			}

			localAddr := "127.0.0.1:" + hostPort
			remoteAddr := "127.0.0.1:" + hostPort

			ln, err := p.sshClient.StartLocalForward(localAddr, remoteAddr)
			if err != nil {
				return fmt.Errorf("setup forward %s->%s: %w", localAddr, remoteAddr, err)
			}
			_ = ln // store to close later, or track by container ID
		}
	}

	return nil
}
