package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/teamycloud/remote-docker-agent/pkg/transparent_ssh_agent"
)

func main() {
	var (
		listenAddr      = flag.String("listen", "127.0.0.1:2375", "Local address to listen on")
		sshUser         = flag.String("ssh-user", "root", "SSH username")
		sshHost         = flag.String("ssh-host", "", "SSH host:port (required)")
		sshKeyPath      = flag.String("ssh-key", os.ExpandEnv("$HOME/.ssh/id_rsa"), "Path to SSH private key")
		remoteDockerURL = flag.String("remote-docker", "unix:///var/run/docker.sock", "Remote Docker socket URL")
		mode            = flag.String("mode", "transparent", "Proxy mode: 'transparent' or 'docker-aware'")
	)

	flag.Parse()

	if *sshHost == "" {
		log.Fatal("SSH host is required (use -ssh-host)")
	}

	cfg := transparent_ssh_agent.Config{
		ListenAddr:      *listenAddr,
		SSHUser:         *sshUser,
		SSHHost:         *sshHost,
		SSHKeyPath:      *sshKeyPath,
		RemoteDockerURL: *remoteDockerURL,
	}

	log.Printf("Starting SSH-transparent-based Docker proxy...")
	log.Printf("  Mode: %s", *mode)
	log.Printf("  Listen: %s", cfg.ListenAddr)
	log.Printf("  SSH: %s@%s", cfg.SSHUser, cfg.SSHHost)
	log.Printf("  Remote Docker: %s", cfg.RemoteDockerURL)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)

	switch *mode {
	case "transparent":
		proxy, err := transparent_ssh_agent.NewTCPProxy(cfg)
		if err != nil {
			log.Fatalf("Failed to create TCP proxy: %v", err)
		}
		defer proxy.Close()

		go func() {
			errCh <- proxy.ListenAndServe()
		}()

	case "docker-aware":
		proxy, err := transparent_ssh_agent.NewDockerTCPProxy(cfg)
		if err != nil {
			log.Fatalf("Failed to create Docker TCP proxy: %v", err)
		}
		defer proxy.Close()

		// Enable container creation interception
		proxy.InterceptCreateContainer()

		// Add logging hooks
		proxy.SetBeforeRequestHook(func(req *http.Request) error {
			log.Printf("[REQUEST] %s %s", req.Method, req.URL.Path)
			return nil
		})

		proxy.SetAfterResponseHook(func(resp *http.Response) error {
			log.Printf("[RESPONSE] %d %s", resp.StatusCode, resp.Status)
			return nil
		})

		go func() {
			errCh <- proxy.ListenAndServe()
		}()

	default:
		log.Fatalf("Invalid mode: %s (use 'transparent' or 'docker-aware')", *mode)
	}

	log.Println("Proxy started. Press Ctrl+C to stop.")
	log.Printf("Use: export DOCKER_HOST=tcp://%s", cfg.ListenAddr)

	// Wait for shutdown signal or error
	select {
	case <-sigCh:
		log.Println("Shutting down gracefully...")
	case err := <-errCh:
		if err != nil {
			log.Fatalf("Proxy error: %v", err)
		}
	}
}
