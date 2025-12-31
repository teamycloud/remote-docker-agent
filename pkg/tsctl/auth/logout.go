package auth

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewLogoutCommand creates the logout command
func NewLogoutCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out from Tinyscale",
		Long: `Log out from Tinyscale and clear local credentials.

This command will remove the locally stored authentication data.`,
		RunE: runLogout,
	}

	return cmd
}

func runLogout(cmd *cobra.Command, args []string) error {
	authData, err := LoadAuthData()
	if err != nil {
		return fmt.Errorf("failed to load authentication data: %w", err)
	}

	if authData == nil {
		fmt.Println("You are not logged in.")
		return nil
	}

	if err := ClearAuthData(); err != nil {
		return fmt.Errorf("failed to clear authentication data: %w", err)
	}

	userName := ""
	if authData.User != nil {
		userName = fmt.Sprintf("%s %s", authData.User.FirstName, authData.User.LastName)
	}

	if userName != "" {
		fmt.Printf("Logged out %s successfully.\n", userName)
	} else {
		fmt.Println("Logged out successfully.")
	}

	return nil
}
