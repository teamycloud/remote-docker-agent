package auth

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewLoginCommand creates the login command
func NewLoginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to Tinyscale",
		Long: `Log in to Tinyscale using OAuth2 device code flow.

This command will:
1. Display a verification URL and code
2. Wait for you to authenticate in your browser
3. Save your credentials locally
4. Prompt you to select an active organization`,
		RunE: runLogin,
	}

	return cmd
}

func runLogin(cmd *cobra.Command, args []string) error {
	authEndpoint := GetLoginEndpoint()
	openAPIEndpoint := GetOpenAPIEndpoint()

	fmt.Printf("Logging in to Tinyscale...\n")
	fmt.Printf("Auth server: %s\n\n", authEndpoint)

	// Step 1: Start device authorization
	oauthClient := NewOAuthClient(authEndpoint)
	authResp, err := oauthClient.StartDeviceAuthorization()
	if err != nil {
		return fmt.Errorf("failed to start device authorization: %w", err)
	}

	// Step 2: Display verification info to user
	fmt.Printf("To sign in, use a web browser to open the page:\n")
	fmt.Printf("  %s\n\n", authResp.VerificationURI)
	fmt.Printf("And enter the code:\n")
	fmt.Printf("  %s\n\n", authResp.UserCode)

	if authResp.VerificationURIComplete != "" {
		fmt.Printf("Or open this URL directly:\n")
		fmt.Printf("  %s\n\n", authResp.VerificationURIComplete)
	}

	fmt.Printf("Waiting for authentication...\n")

	// Step 3: Poll for token
	tokenResp, err := oauthClient.PollForToken(authResp.DeviceCode, authResp.Interval, authResp.ExpiresIn)
	if err != nil {
		return fmt.Errorf("failed to complete authentication: %w", err)
	}

	fmt.Printf("Authentication successful!\n\n")

	// Step 4: Parse user info from id_token
	userInfo, err := ExtractUserInfo(tokenResp.IDToken)
	if err != nil {
		return fmt.Errorf("failed to parse user information: %w", err)
	}

	// Step 5: Save auth data (without organization for now)
	authData := &AuthData{
		User: userInfo,
		Token: &TokenInfo{
			IDToken:      tokenResp.IDToken,
			RefreshToken: tokenResp.RefreshToken,
		},
		Endpoints: &EndpointsInfo{
			Auth:    authEndpoint,
			OpenAPI: openAPIEndpoint,
			Connect: DefaultConnectEndpoint,
		},
	}

	if err := SaveAuthData(authData); err != nil {
		return fmt.Errorf("failed to save authentication data: %w", err)
	}

	fmt.Printf("Welcome, %s %s!\n\n", userInfo.FirstName, userInfo.LastName)

	// Step 6: Trigger organization selection
	return selectOrganization(authData)
}
