package transparent_ssh_agent

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Example usage of the transparent TCP proxy
func ExampleTCPProxy() {
	cfg := Config{
		ListenAddr:      "127.0.0.1:2375",
		SSHUser:         "root",
		SSHHost:         "remote.example.com:22",
		SSHKeyPath:      "/home/user/.ssh/id_rsa",
		RemoteDockerURL: "unix:///var/run/docker.sock",
	}

	proxy, err := NewTCPProxy(cfg)
	if err != nil {
		fmt.Printf("Failed to create proxy: %v\n", err)
		return
	}
	defer proxy.Close()

	// Start the proxy in a goroutine
	go func() {
		if err := proxy.ListenAndServe(); err != nil {
			fmt.Printf("Proxy error: %v\n", err)
		}
	}()

	// Keep running
	time.Sleep(1 * time.Hour)
}

// Example usage of the Docker-aware TCP proxy
func ExampleDockerTCPProxy() {
	cfg := Config{
		ListenAddr:      "127.0.0.1:2375",
		SSHUser:         "root",
		SSHHost:         "remote.example.com:22",
		SSHKeyPath:      "/home/user/.ssh/id_rsa",
		RemoteDockerURL: "unix:///var/run/docker.sock",
	}

	proxy, err := NewDockerTCPProxy(cfg)
	if err != nil {
		fmt.Printf("Failed to create Docker proxy: %v\n", err)
		return
	}
	defer proxy.Close()

	// Enable container creation interception
	proxy.InterceptCreateContainer()

	// Set custom hooks
	proxy.SetBeforeRequestHook(func(req *http.Request) error {
		fmt.Printf("Before request: %s %s\n", req.Method, req.URL.Path)
		return nil
	})

	proxy.SetAfterResponseHook(func(resp *http.Response) error {
		fmt.Printf("After response: %d\n", resp.StatusCode)
		return nil
	})

	// Start the proxy
	go func() {
		if err := proxy.ListenAndServe(); err != nil {
			fmt.Printf("Docker proxy error: %v\n", err)
		}
	}()

	// Keep running
	time.Sleep(1 * time.Hour)
}

// Basic test to ensure package builds
func TestTCPAgentPackage(t *testing.T) {
	// Just a placeholder to ensure the package can be tested
	cfg := Config{
		ListenAddr:      "127.0.0.1:2375",
		SSHUser:         "testuser",
		SSHHost:         "localhost:22",
		SSHKeyPath:      "/tmp/test_key",
		RemoteDockerURL: "unix:///var/run/docker.sock",
	}

	if cfg.ListenAddr == "" {
		t.Error("Config should have listen address")
	}
}
