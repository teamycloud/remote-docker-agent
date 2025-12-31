package auth

import (
	"github.com/spf13/cobra"
)

// NewAuthCommand creates the auth parent command
func NewAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Tinyscale authentication",
		Long:  `Commands for authenticating with Tinyscale and managing organizations.`,
	}

	cmd.AddCommand(NewLoginCommand())
	cmd.AddCommand(NewLogoutCommand())
	cmd.AddCommand(NewSwitchOrgCommand())

	return cmd
}
