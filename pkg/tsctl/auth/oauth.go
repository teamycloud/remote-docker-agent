package auth

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
)

// OAuthClient handles OAuth2 device code flow using golang.org/x/oauth2
type OAuthClient struct {
	config       *oauth2.Config
	authEndpoint string
}

// NewOAuthClient creates a new OAuth client using the standard golang.org/x/oauth2 library
func NewOAuthClient(authEndpoint string) *OAuthClient {
	config := &oauth2.Config{
		ClientID: ClientID,
		Endpoint: oauth2.Endpoint{
			DeviceAuthURL: authEndpoint + DeviceAuthorizationPath,
			TokenURL:      authEndpoint + TokenPath,
		},
		Scopes: []string{"openapi", "hosts"},
	}

	return &OAuthClient{
		config:       config,
		authEndpoint: authEndpoint,
	}
}

// StartDeviceAuthorization initiates the device authorization flow
// Returns the device authorization response from the OAuth2 library
func (c *OAuthClient) StartDeviceAuthorization() (*oauth2.DeviceAuthResponse, error) {
	ctx := context.Background()

	deviceAuth, err := c.config.DeviceAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to start device authorization: %w", err)
	}

	return deviceAuth, nil
}

// PollForToken polls the token endpoint until authorization is complete, expired, or failed
func (c *OAuthClient) PollForToken(deviceAuth *oauth2.DeviceAuthResponse) (*TokenResponse, error) {
	ctx := context.Background()

	token, err := c.config.DeviceAccessToken(ctx, deviceAuth)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain token: %w", err)
	}

	// Extract id_token from the extra fields
	idToken, ok := token.Extra("id_token").(string)
	if !ok || idToken == "" {
		return nil, fmt.Errorf("id_token not found in token response - check OAuth provider configuration")
	}

	return &TokenResponse{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		IDToken:      idToken,
	}, nil
}

// RefreshToken uses a refresh token to obtain a new id_token
func (c *OAuthClient) RefreshToken(refreshToken string) (*TokenResponse, error) {
	ctx := context.Background()

	// Create a token source from the refresh token
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	tokenSource := c.config.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("unable to refresh token: %w", err)
	}

	// Extract id_token from the extra fields
	idToken, ok := newToken.Extra("id_token").(string)
	if !ok || idToken == "" {
		return nil, fmt.Errorf("id_token not found in refresh response - check OAuth provider configuration")
	}

	return &TokenResponse{
		AccessToken:  newToken.AccessToken,
		TokenType:    newToken.TokenType,
		RefreshToken: newToken.RefreshToken,
		IDToken:      idToken,
	}, nil
}
