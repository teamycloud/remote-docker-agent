package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIClient handles Tinyscale OpenAPI calls
type APIClient struct {
	httpClient  *http.Client
	apiEndpoint string
	idToken     string
}

// NewAPIClient creates a new API client
func NewAPIClient(apiEndpoint, idToken string) *APIClient {
	return &APIClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiEndpoint: apiEndpoint,
		idToken:     idToken,
	}
}

// GetMyOrganizations fetches the list of organizations for the current user
func (c *APIClient) GetMyOrganizations() ([]Organization, error) {
	endpoint := c.apiEndpoint + OrganizationsPath

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create organizations request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.idToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to send organizations request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read organizations response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("organizations request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var orgs []Organization
	if err := json.Unmarshal(body, &orgs); err != nil {
		return nil, fmt.Errorf("unable to parse organizations response: %w", err)
	}

	return orgs, nil
}
