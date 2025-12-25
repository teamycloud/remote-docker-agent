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
	Issuer string
}

// ExtractUserIdentity extracts user identity from the client certificate
// The identity is embedded as a SAN URI: spiffe://tinyscale.com/orgs/<org-id>/users/<user-id>
func ExtractUserIdentity(cert *x509.Certificate, expectedIssuer string) (*UserIdentity, error) {
	if cert == nil {
		return nil, errors.New("certificate is nil")
	}

	// Look for SPIFFE URI in SAN URIs
	for _, uri := range cert.URIs {
		if strings.HasPrefix(uri.Scheme, "spiffe") {
			identity, err := parseSPIFFEURI(uri, expectedIssuer)
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
func parseSPIFFEURI(uri *url.URL, expectedIssuer string) (*UserIdentity, error) {
	if uri.Scheme != "spiffe" {
		return nil, fmt.Errorf("invalid URI scheme: expected 'spiffe', got '%s'", uri.Scheme)
	}

	// Validate issuer (hostname)
	if uri.Host != expectedIssuer {
		return nil, fmt.Errorf("issuer mismatch: expected '%s', got '%s'", expectedIssuer, uri.Host)
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
		Issuer: uri.Host,
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

// ValidateIssuerMatch checks if the certificate issuer matches one of the CA certificates
// This validates that the issuer domain matches the CN or one of the alternative names
func ValidateIssuerMatch(cert *x509.Certificate, caPool *x509.CertPool, expectedIssuer string) error {
	// Get CA certificates from the pool
	// Note: x509.CertPool doesn't provide direct access to certificates,
	// so we rely on the Verify method which already checks the chain
	// The issuer validation is implicit in the certificate chain verification

	// Additional check: verify the issuer field contains expected issuer
	if cert.Issuer.CommonName != "" {
		if matchesIssuer(cert.Issuer.CommonName, expectedIssuer) {
			return nil
		}
	}

	// Check if any part of the issuer DN contains the expected issuer
	issuerStr := cert.Issuer.String()
	if strings.Contains(issuerStr, expectedIssuer) {
		return nil
	}

	return fmt.Errorf("issuer validation failed: certificate issuer does not match expected issuer '%s'", expectedIssuer)
}

// matchesIssuer checks if a domain matches the expected issuer, ignoring wildcards
func matchesIssuer(domain, expectedIssuer string) bool {
	// Remove wildcard prefix if present
	domain = strings.TrimPrefix(domain, "*.")
	expectedIssuer = strings.TrimPrefix(expectedIssuer, "*.")

	return strings.EqualFold(domain, expectedIssuer) || strings.HasSuffix(domain, "."+expectedIssuer)
}
