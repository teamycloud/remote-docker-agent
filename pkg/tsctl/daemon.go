package tsctl

import (
	"github.com/spf13/cobra"
)

func NewDaemonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the Tinyscale proxy daemon",
		Long:  `Commands for starting and stopping the Tinyscale local TCP proxy daemon`,
	}

	cmd.AddCommand(NewStartCommand())
	cmd.AddCommand(NewStopCommand())

	return cmd
}
