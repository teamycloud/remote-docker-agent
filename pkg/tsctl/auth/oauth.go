package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthClient handles OAuth2 device code flow
type OAuthClient struct {
	httpClient   *http.Client
	authEndpoint string
}

// NewOAuthClient creates a new OAuth client
func NewOAuthClient(authEndpoint string) *OAuthClient {
	return &OAuthClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		authEndpoint: authEndpoint,
	}
}

// StartDeviceAuthorization initiates the device authorization flow
func (c *OAuthClient) StartDeviceAuthorization() (*DeviceAuthorizationResponse, error) {
	endpoint := c.authEndpoint + DeviceAuthorizationPath

	data := url.Values{}
	data.Set("client_id", ClientID)
	data.Set("scope", Scope)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("unable to create device authorization request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to send device authorization request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read device authorization response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device authorization failed with status %d: %s", resp.StatusCode, string(body))
	}

	var authResp DeviceAuthorizationResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return nil, fmt.Errorf("unable to parse device authorization response: %w", err)
	}

	return &authResp, nil
}

// PollForToken polls the token endpoint until authorization is complete, expired, or failed
func (c *OAuthClient) PollForToken(deviceCode string, interval int, expiresIn int) (*TokenResponse, error) {
	endpoint := c.authEndpoint + TokenPath

	pollInterval := time.Duration(interval) * time.Second
	if pollInterval < 5*time.Second {
		pollInterval = 5 * time.Second
	}

	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)

	for time.Now().Before(deadline) {
		tokenResp, shouldRetry, err := c.requestToken(endpoint, deviceCode)
		if err != nil {
			return nil, err
		}
		if tokenResp != nil {
			return tokenResp, nil
		}
		if !shouldRetry {
			return nil, fmt.Errorf("authorization was denied")
		}

		time.Sleep(pollInterval)
	}

	return nil, fmt.Errorf("device authorization expired")
}

// requestToken makes a single token request
// Returns (token, shouldRetry, error)
func (c *OAuthClient) requestToken(endpoint, deviceCode string) (*TokenResponse, bool, error) {
	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	data.Set("device_code", deviceCode)
	data.Set("client_id", ClientID)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, false, fmt.Errorf("unable to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("unable to send token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("unable to read token response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var tokenResp TokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			return nil, false, fmt.Errorf("unable to parse token response: %w", err)
		}
		return &tokenResp, false, nil
	}

	// Check for pending/slow_down errors
	var errResp TokenErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return nil, false, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	switch errResp.Error {
	case "authorization_pending":
		return nil, true, nil // Continue polling
	case "slow_down":
		// Increase interval (handled by caller sleeping longer)
		return nil, true, nil
	case "access_denied":
		return nil, false, fmt.Errorf("authorization denied by user")
	case "expired_token":
		return nil, false, fmt.Errorf("device code expired")
	default:
		return nil, false, fmt.Errorf("token request failed: %s - %s", errResp.Error, errResp.ErrorDescription)
	}
}

// RefreshToken uses a refresh token to obtain a new id_token
func (c *OAuthClient) RefreshToken(refreshToken string) (*TokenResponse, error) {
	endpoint := c.authEndpoint + TokenPath

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", ClientID)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("unable to create refresh token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to send refresh token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read refresh token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("unable to parse refresh token response: %w", err)
	}

	return &tokenResp, nil
}
