package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	mtlsproxy "github.com/teamycloud/tsctl/pkg/mtls-proxy"
)

func main() {
	// Command line flags
	var (
		listenAddr   = flag.String("listen", ":8443", "Listen address for the proxy")
		caCerts      = flag.String("ca-certs", "", "Comma-separated list of CA certificate paths. These CAs are used to validate client certificates.")
		serverCert   = flag.String("server-cert", "", "Server certificate path, client will verify this certificate to authenticate us as the proxy server")
		serverKey    = flag.String("server-key", "", "Server private key path")
		clientCert   = flag.String("client-cert", "", "Path to client certificate used for backend connections")
		clientKey    = flag.String("client-key", "", "Path to client private key used for backend connections")
		dbHost       = flag.String("db-host", "127.0.0.1", "Database host")
		dbPort       = flag.Int("db-port", 5432, "Database port")
		dbUser       = flag.String("db-user", "tinyscale", "Database user")
		dbPassword   = flag.String("db-password", "tinyscale", "Database password")
		dbName       = flag.String("db-name", "tinyscale-ssh", "Database name")
		dockerPort   = flag.Int("docker-port", 2375, "Docker Engine API port on backend hosts")
		hostExecPort = flag.Int("host-exec-port", 2090, "Host exec port on backend hosts")
		logLevel     = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	)

	flag.Parse()

	// Setup logger
	logger := logrus.New()
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logger.Fatalf("Invalid log level: %v", err)
	}
	logger.SetLevel(level)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Validate required flags
	if *caCerts == "" {
		logger.Fatal("--ca-certs is required")
	}
	if *serverCert == "" {
		logger.Fatal("--server-cert is required")
	}
	if *serverKey == "" {
		logger.Fatal("--server-key is required")
	}
	if *clientCert == "" {
		logger.Fatal("--client-cert is required")
	}
	if *clientKey == "" {
		logger.Fatal("--client-key is required")
	}

	// Parse CA certificates
	caCertPaths := parseCACertPaths(*caCerts)
	if len(caCertPaths) == 0 {
		logger.Fatal("At least one CA certificate path is required")
	}

	// Create configuration
	config := &mtlsproxy.Config{
		ListenAddr:     *listenAddr,
		CACertPaths:    caCertPaths,
		ServerCertPath: *serverCert,
		ServerKeyPath:  *serverKey,
		ClientCertPath: *clientCert,
		ClientKeyPath:  *clientKey,
		Database: mtlsproxy.DatabaseConfig{
			Host:              *dbHost,
			Port:              *dbPort,
			User:              *dbUser,
			Password:          *dbPassword,
			DbName:            *dbName,
			ConnectionTimeout: 5,
			MaxOpenConns:      50,
			MaxIdleConns:      50,
		},
		DockerPort:   *dockerPort,
		HostExecPort: *hostExecPort,
	}

	// Create and start proxy
	proxy, err := mtlsproxy.NewProxy(config, logger)
	if err != nil {
		logger.Fatalf("Failed to create proxy: %v", err)
	}

	if err := proxy.Start(); err != nil {
		logger.Fatalf("Failed to start proxy: %v", err)
	}

	logger.Info("mTLS proxy started successfully")

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down proxy...")
	if err := proxy.Stop(); err != nil {
		logger.Errorf("Error during shutdown: %v", err)
	}

	logger.Info("Proxy stopped")
}

// parseCACertPaths parses a comma-separated list of CA certificate paths
func parseCACertPaths(caCerts string) []string {
	if caCerts == "" {
		return nil
	}

	paths := []string{}
	for _, path := range splitComma(caCerts) {
		if path != "" {
			paths = append(paths, path)
		}
	}

	return paths
}

// splitComma splits a string by comma
func splitComma(s string) []string {
	result := []string{}
	current := ""

	for _, c := range s {
		if c == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}
