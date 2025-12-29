package daemon

import (
	"fmt"
	"path/filepath"

	"github.com/mutagen-io/mutagen/pkg/filesystem"
)

const (
	// pidFilename is the name of the daemon lock. It resides within the daemon
	// subdirectory of the tinyscale directory.
	pidFilename = "daemon.pid"

	pidTerminateFilename = "daemon.pid.terminate"

	// endpointFilename is the name of the tinyscale local endpoint
	endpointFilename = "tinyscale.sock"
)

// subpath computes a subpath of the daemon subdirectory, creating the daemon
// subdirectory in the process.
func subpath(name string) (string, error) {
	// Compute the daemon root directory path and ensure it exists.
	daemonRoot, err := filesystem.Mutagen(true, filesystem.MutagenDaemonDirectoryName)
	if err != nil {
		return "", fmt.Errorf("unable to compute daemon directory: %w", err)
	}

	// Compute the combined path.
	return filepath.Join(daemonRoot, name), nil
}

func PidPath() (string, error) {
	return subpath(pidFilename)
}

func PidTerminatePath() (string, error) {
	return subpath(pidTerminateFilename)
}

func EndpointPath() (string, error) {
	return subpath(endpointFilename)
}
