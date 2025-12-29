package tsctl

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/mutagen-io/mutagen/pkg/forwarding"
	_ "github.com/mutagen-io/mutagen/pkg/forwarding/protocols/local"
	_ "github.com/mutagen-io/mutagen/pkg/forwarding/protocols/ssh"
	"github.com/mutagen-io/mutagen/pkg/logging"
	"github.com/mutagen-io/mutagen/pkg/synchronization"
	_ "github.com/mutagen-io/mutagen/pkg/synchronization/protocols/local"
	_ "github.com/mutagen-io/mutagen/pkg/synchronization/protocols/ssh"
	"github.com/spf13/cobra"
	"github.com/teamycloud/tsctl/pkg/daemon"
	docker_proxy "github.com/teamycloud/tsctl/pkg/docker-proxy"
	"github.com/teamycloud/tsctl/pkg/docker-proxy/types"

	_ "github.com/teamycloud/tsctl/pkg/ts-tunnel/forwarding-protocol"
	_ "github.com/teamycloud/tsctl/pkg/ts-tunnel/synchronization-protocol"
)

func NewStartCommand() *cobra.Command {
	var (
		listenAddr   string
		sshUser      string
		sshHost      string
		sshKeyPath   string
		remoteDocker string
		logLevelFlag string

		tsTunnelServer   string // HTTPS endpoint (e.g., "containers.tinyscale.net:443")
		tsTunnelCertFile string // Path to client certificate file
		tsTunnelKeyFile  string // Path to client key file
		tsTunnelCAFile   string // Path to CA certificate file (optional)
		tsTunnelInsecure bool   // whether can we skip tls verification
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the local proxy for Tinyscale Container API",
		Long:  `Start the TCP proxy server that forwards Container API calls to a remote daemon over running Tinyscale`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create the root logger.
			logLevel := logging.LevelInfo
			if l, ok := logging.NameToLevel(logLevelFlag); !ok {
				fmt.Printf("WARNING: invalid log level specified in environment: %s, default log level 'info' will be used\n", logLevelFlag)
			} else {
				logLevel = l
			}
			logger := logging.NewLogger(logLevel, os.Stderr)

			// Attempt to acquire the daemon lock and defer its release.
			lock, err := daemon.AcquireLock()
			if err != nil {
				return fmt.Errorf("unable to acquire lock for the daemon pid path: %w, tsctl daemon is probably already running", err)
			}
			defer lock.Release()

			// Create a channel to track termination signals. We do this before creating
			// and starting other infrastructure so that we can ensure things terminate
			// smoothly, not mid-initialization.
			signalTermination := make(chan os.Signal, 2)
			signal.Notify(signalTermination, syscall.SIGINT, syscall.SIGTERM)

			fileTermination := make(chan bool, 1)
			if err := watchTerminationSignal(fileTermination, logger); err != nil {
				return err
			}

			cfg := types.Config{
				ListenAddr:    listenAddr,
				TransportType: types.TransportSSH,
				SSHUser:       sshUser,
				SSHHost:       sshHost,
				SSHKeyPath:    sshKeyPath,
				RemoteDocker:  remoteDocker,
			}

			remoteAddr := ""
			if cfg.SSHHost != "" {
				remoteAddr = fmt.Sprintf("%s@%s", cfg.SSHUser, cfg.SSHHost)
			} else if tsTunnelServer != "" {
				cfg.TransportType = types.TransportTSTunnel
				cfg.TSTunnelServer = tsTunnelServer
				remoteAddr = tsTunnelServer

				if tsTunnelCertFile != "" && tsTunnelKeyFile != "" {
					cfg.TSTunnelCertFile = tsTunnelCertFile
					cfg.TSTunnelKeyFile = tsTunnelKeyFile

				}

				if tsTunnelCAFile != "" {
					cfg.TSTunnelCAFile = tsTunnelCAFile
				}
				cfg.TSInsecure = tsTunnelInsecure
			} else {
				return fmt.Errorf("we need to connect to remote docker daemon by either SSH or ts-tunnel")
			}

			bannerFormat := `
Starting TCP proxy with %s transport...
  Listen: %s
  Remote: %s
`
			logger.Infof(bannerFormat, (string)(cfg.TransportType), cfg.ListenAddr, remoteAddr)

			forwardingManager, err := forwarding.NewManager(logger.Sublogger("port-forward"))
			if err != nil {
				return fmt.Errorf("unable to create forwarding session manager: %v", err)
			}
			defer forwardingManager.Shutdown()

			synchronizationManager, err := synchronization.NewManager(logger.Sublogger("file-sync"))
			if err != nil {
				return fmt.Errorf("unable to create synchronization session manager: %v", err)
			}
			defer synchronizationManager.Shutdown()

			errCh := make(chan error, 1)

			proxy, err := docker_proxy.NewProxy(cfg, forwardingManager, synchronizationManager, logger.Sublogger("proxy"))
			if err != nil {
				return fmt.Errorf("failed to create TCP proxy: %v", err)
			}
			go func() {
				errCh <- proxy.ListenAndServe()
			}()

			logger.Info("Proxy started. Press Ctrl+C to stop.")
			logger.Infof("Use: export DOCKER_HOST=tcp://%s", cfg.ListenAddr)

			// Wait for termination from a signal, the daemon service, or the gRPC
			// server. We treat termination via the daemon service as a non-error.
			select {
			case s := <-signalTermination:
				logger.Info("Terminating due to signal:", s)
				proxy.Close()
				return fmt.Errorf("terminated by signal: %s", s)
			case <-fileTermination:
				logger.Info("Terminating due to file signal")
				proxy.Close()
				return nil
			case err = <-errCh:
				logger.Error("Daemon server failure:", err)
				return fmt.Errorf("daemon server termination: %w", err)
			}
		},
		SilenceUsage: true,
	}

	// Add flags to the start command
	cmd.Flags().StringVar(&listenAddr, "listen", "127.0.0.1:2375", "Local address to listen on")
	cmd.Flags().StringVar(&sshUser, "ssh-user", "root", "SSH username")
	cmd.Flags().StringVar(&sshHost, "ssh-host", "", "SSH host and port")
	cmd.Flags().StringVar(&sshKeyPath, "ssh-key", os.Getenv("HOME")+"/.ssh/id_rsa", "Path to SSH private key")
	cmd.Flags().StringVar(&remoteDocker, "remote-docker", "unix:///var/run/docker.sock", "Remote Docker socket URL when using the SSH transport")

	cmd.Flags().StringVar(&tsTunnelServer, "ts-server", "", "Tinyscale server address")
	cmd.Flags().StringVar(&tsTunnelCertFile, "ts-cert", "", "Path to mTLS certificate")
	cmd.Flags().StringVar(&tsTunnelKeyFile, "ts-key", "", "Path to mTLS private key")
	cmd.Flags().StringVar(&tsTunnelCAFile, "ts-ca", "", "Path to accepted Tinyscale CA certificate")
	cmd.Flags().BoolVar(&tsTunnelInsecure, "ts-insecure", false, "Skip tlsconfig verification when connecting to Tinyscale server")

	cmd.Flags().StringVar(&logLevelFlag, "log-level", "info", "Log level")
	return cmd
}

func watchTerminationSignal(fileTermination chan<- bool, logger *logging.Logger) error {
	terminatePath, err := daemon.PidTerminatePath()
	if err != nil {
		return fmt.Errorf("unable to compute terminate file path: %w", err)
	}

	// Create a file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("unable to create file watcher: %w", err)
	}

	// Watch the daemon directory for file creation events
	daemonDir := filepath.Dir(terminatePath)
	if err := watcher.Add(daemonDir); err != nil {
		watcher.Close()
		return fmt.Errorf("unable to watch daemon directory: %w", err)
	}

	logger.Infof("Watching for termination signal at: %s", terminatePath)

	// Get current process PID
	currentPid := os.Getpid()

	// Start a goroutine to monitor file events
	go func() {
		defer watcher.Close()

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					logger.Info("File watcher events channel closed")
					return
				}

				logger.Debugf("File event received: Op=%v, Name=%s", event.Op, event.Name)

				// Check if the terminate file was created or written
				if (event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Write == fsnotify.Write) &&
					event.Name == terminatePath {
					logger.Debugf("Terminate file detected, reading content...")
					// Read the file content
					if content, err := os.ReadFile(terminatePath); err == nil {
						contentStr := string(content)
						logger.Debugf("Terminate file content: '%s' (length: %d), current PID: %d", contentStr, len(contentStr), currentPid)
						if pid, err := strconv.Atoi(contentStr); err == nil && pid == currentPid {
							logger.Debugf("PID matches! Sending termination signal")
							_ = os.Remove(terminatePath)
							fileTermination <- true
							return
						} else {
							if err != nil {
								logger.Debugf("Failed to parse PID from content '%s': %v", contentStr, err)
							} else {
								logger.Debugf("PID mismatch: expected %d, got %d", currentPid, pid)
							}
						}
					} else {
						logger.Infof("Failed to read terminate file: %v", err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					logger.Info("Terminate file watcher errors channel closed")
					return
				}
				logger.Infof("Terminate file watcher error: %v", err)
			}
		}
	}()

	return nil
}
