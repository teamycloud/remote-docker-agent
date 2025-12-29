package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/teamycloud/tsctl/pkg/tsctl"
)

var rootCmd = &cobra.Command{
	Use:   "tsctl",
	Short: "tinyscale - your container runtime on the cloud",
	Long:  `Utilities for managing and connecting container hosts on the tinyscale platform`,
}

func init() {
	// todo: move start/stop as sub commands of daemon
	rootCmd.AddCommand(tsctl.NewStartCommand())
	rootCmd.AddCommand(tsctl.NewStopCommand())
	rootCmd.AddCommand(tsctl.NewHostExecCommand())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
