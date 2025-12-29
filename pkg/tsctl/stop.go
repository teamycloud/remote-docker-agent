package tsctl

import (
	"fmt"
	"io"
	"os"
	"strconv"

	_ "github.com/mutagen-io/mutagen/pkg/forwarding/protocols/local"
	_ "github.com/mutagen-io/mutagen/pkg/forwarding/protocols/ssh"
	"github.com/mutagen-io/mutagen/pkg/logging"
	_ "github.com/mutagen-io/mutagen/pkg/synchronization/protocols/local"
	_ "github.com/mutagen-io/mutagen/pkg/synchronization/protocols/ssh"
	"github.com/spf13/cobra"
	"github.com/teamycloud/tsctl/pkg/daemon"
	_ "github.com/teamycloud/tsctl/pkg/ts-tunnel/forwarding-protocol"
	_ "github.com/teamycloud/tsctl/pkg/ts-tunnel/synchronization-protocol"
)

func NewStopCommand() *cobra.Command {
	var logLevelFlag string

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the Tinyscale proxy daemon",
		Long:  `Stop the running Tinyscale local TCP proxy server`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create the root logger.
			logLevel := logging.LevelInfo
			if l, ok := logging.NameToLevel(logLevelFlag); !ok {
				fmt.Printf("WARNING: invalid log level specified in environment: %s, default log level 'info' will be used\n", logLevelFlag)
			} else {
				logLevel = l
			}
			logger := logging.NewLogger(logLevel, os.Stderr)

			pidPath, err := daemon.PidPath()
			if err != nil {
				return fmt.Errorf("unable to compute daemon pid file path: %w", err)
			}

			file, err := os.OpenFile(pidPath, os.O_RDONLY, 0)
			if err != nil {
				return fmt.Errorf("unable to open daemon pid file: %w", err)
			}
			bytes, err := io.ReadAll(file)
			_ = file.Close()
			if err != nil {
				return fmt.Errorf("unable to read daemon pid file: %w", err)
			}

			// Trim any whitespace from the PID string
			pidStr := string(bytes)
			logger.Debugf("Read PID from file: '%s' (length: %d)\n", pidStr, len(pidStr))

			if pid, err := strconv.Atoi(pidStr); err != nil {
				return fmt.Errorf("invalid pid read from daemon pid file: %w", err)
			} else {
				// Create terminate file with the PID content
				terminatePath, err := daemon.PidTerminatePath()
				if err != nil {
					return fmt.Errorf("unable to compute terminate file path: %w", err)
				}

				logger.Debugf("Creating termination file at: %s\n", terminatePath)
				logger.Debugf("Writing PID: %d\n", pid)

				if err := os.WriteFile(terminatePath, []byte(pidStr), 0644); err != nil {
					return fmt.Errorf("unable to create terminate file: %w", err)
				}

				// Verify the file was written correctly
				if content, err := os.ReadFile(terminatePath); err == nil {
					logger.Debugf("Verified termination file content: '%s'\n", string(content))
				}

				logger.Infof("Sent termination signal to daemon process (PID: %d)\n", pid)
				return nil
			}
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&logLevelFlag, "log-level", "info", "Log level")
	return cmd
}
