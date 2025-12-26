package mtlsproxy

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// UserIdentity represents the extracted user identity from the certificate
type UserIdentity struct {
	UserID string
	OrgID  string
}

// ExtractUserIdentity extracts user identity from the client certificate
// The identity is embedded as a SAN URI: spiffe://tinyscale.com/orgs/<org-id>/users/<user-id>
func ExtractUserIdentity(cert *x509.Certificate) (*UserIdentity, error) {
	if cert == nil {
		return nil, errors.New("certificate is nil")
	}

	// Look for SPIFFE URI in SAN URIs
	for _, uri := range cert.URIs {
		if strings.HasPrefix(uri.Scheme, "spiffe") {
			identity, err := parseSPIFFEURI(uri)
			if err != nil {
				continue // Try next URI
			}
			return identity, nil
		}
	}

	return nil, errors.New("no valid SPIFFE URI found in certificate")
}

// parseSPIFFEURI parses a SPIFFE URI and extracts the user identity
// Expected format: spiffe://tinyscale.com/orgs/<org-id>/users/<user-id>
func parseSPIFFEURI(uri *url.URL) (*UserIdentity, error) {
	if uri.Scheme != "spiffe" {
		return nil, fmt.Errorf("invalid URI scheme: expected 'spiffe', got '%s'", uri.Scheme)
	}

	// Parse path: /orgs/<org-id>/users/<user-id>
	path := strings.TrimPrefix(uri.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid SPIFFE path format: expected '/orgs/<org-id>/users/<user-id>', got '%s'", uri.Path)
	}

	if parts[0] != "orgs" {
		return nil, fmt.Errorf("invalid SPIFFE path: expected 'orgs' as first segment, got '%s'", parts[0])
	}

	if parts[2] != "users" {
		return nil, fmt.Errorf("invalid SPIFFE path: expected 'users' as third segment, got '%s'", parts[2])
	}

	orgID := parts[1]
	userID := parts[3]

	if orgID == "" {
		return nil, errors.New("org-id is empty")
	}

	if userID == "" {
		return nil, errors.New("user-id is empty")
	}

	return &UserIdentity{
		UserID: userID,
		OrgID:  orgID,
	}, nil
}

// ValidateCertificate validates the client certificate against the CA pool
// It checks expiry and signature
func ValidateCertificate(cert *x509.Certificate, caPool *x509.CertPool) error {
	if cert == nil {
		return errors.New("certificate is nil")
	}

	// Verify the certificate chain
	opts := x509.VerifyOptions{
		Roots:     caPool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	_, err := cert.Verify(opts)
	if err != nil {
		return fmt.Errorf("certificate verification failed: %w", err)
	}

	return nil
}

