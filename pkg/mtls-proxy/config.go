package mtlsproxy

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"
)

// Config holds the mTLS proxy configuration
type Config struct {
	// ListenAddr is the address to listen on (e.g., ":8443")
	ListenAddr string

	// CACertPaths is a list of paths to CA certificate files
	// Multiple CAs are supported for CA rotation
	CACertPaths []string

	// ServerCertPath is the path to the server certificate
	ServerCertPath string

	// ServerKeyPath is the path to the server private key
	ServerKeyPath string

	// Issuer is the expected issuer domain (e.g., "tinyscale.com")
	Issuer string

	// ClientCertPath is the path to the client certificate for backend connections
	ClientCertPath string

	// ClientKeyPath is the path to the client private key for backend connections
	ClientKeyPath string

	// Database configuration
	Database DatabaseConfig
}

// DatabaseConfig holds PostgreSQL database configuration
type DatabaseConfig struct {
	Host              string
	Port              int
	User              string
	Password          string
	DbName            string
	ConnectionTimeout int
	MaxOpenConns      int
	MaxIdleConns      int
	ConnMaxLifetime   time.Duration
	ConnMaxIdleTime   time.Duration
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.ListenAddr == "" {
		return errors.New("ListenAddr is required")
	}

	if len(c.CACertPaths) == 0 {
		return errors.New("at least one CA certificate path is required")
	}

	for _, path := range c.CACertPaths {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("CA certificate not found at %s: %w", path, err)
		}
	}

	if c.ServerCertPath == "" {
		return errors.New("ServerCertPath is required")
	}

	if c.ServerKeyPath == "" {
		return errors.New("ServerKeyPath is required")
	}

	if c.Issuer == "" {
		return errors.New("Issuer is required")
	}

	if c.ClientCertPath == "" {
		return errors.New("ClientCertPath is required")
	}

	if c.ClientKeyPath == "" {
		return errors.New("ClientKeyPath is required")
	}

	// Validate client certificate exists
	if _, err := os.Stat(c.ClientCertPath); err != nil {
		return fmt.Errorf("client certificate not found at %s: %w", c.ClientCertPath, err)
	}

	if _, err := os.Stat(c.ClientKeyPath); err != nil {
		return fmt.Errorf("client key not found at %s: %w", c.ClientKeyPath, err)
	}

	if err := c.Database.Validate(); err != nil {
		return fmt.Errorf("database config validation failed: %w", err)
	}

	return nil
}

// LoadCACertPool loads all CA certificates into a cert pool
func (c *Config) LoadCACertPool() (*x509.CertPool, error) {
	pool := x509.NewCertPool()

	for _, path := range c.CACertPaths {
		certPEM, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate at %s: %w", path, err)
		}

		if !pool.AppendCertsFromPEM(certPEM) {
			return nil, fmt.Errorf("failed to parse CA certificate at %s", path)
		}
	}

	return pool, nil
}

// LoadClientCertificate loads the client certificate and key for backend connections
func (c *Config) LoadClientCertificate() (tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(c.ClientCertPath, c.ClientKeyPath)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to load client certificate: %w", err)
	}
	return cert, nil
}

// Validate checks if the database configuration is valid
func (d *DatabaseConfig) Validate() error {
	if d.Host == "" {
		return errors.New("database host is required")
	}

	if d.Port <= 0 || d.Port > 65535 {
		return errors.New("database port must be between 1 and 65535")
	}

	if d.User == "" {
		return errors.New("database user is required")
	}

	if d.Password == "" {
		return errors.New("database password is required")
	}

	if d.DbName == "" {
		return errors.New("database name is required")
	}

	return nil
}

// ConnectionString returns the PostgreSQL connection string
func (d *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s connect_timeout=%d sslmode=disable",
		d.Host, d.Port, d.User, d.Password, d.DbName, d.ConnectionTimeout,
	)
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		ListenAddr:     ":8443",
		CACertPaths:    []string{},
		ServerCertPath: "",
		ServerKeyPath:  "",
		Issuer:         "tinyscale.com",
		Database: DatabaseConfig{
			Host:              "127.0.0.1",
			Port:              5432,
			User:              "tinyscale",
			Password:          "tinyscale",
			DbName:            "tinyscale-ssh",
			ConnectionTimeout: 5,
			MaxOpenConns:      50,
			MaxIdleConns:      50,
			ConnMaxLifetime:   time.Hour,
			ConnMaxIdleTime:   30 * time.Minute,
		},
	}
}
