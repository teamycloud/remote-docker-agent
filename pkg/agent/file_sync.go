package agent

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// rewriteBindMounts:
// - Detects local paths like "./src:/app"
// - Syncs "./src" to remote temp dir (e.g. "/tmp/agent-sync-1234")
// - Returns new binds like "/tmp/agent-sync-1234:/app"
func (p *DockerProxy) rewriteBindMounts(binds []string) ([]string, error) {
    var newBinds []string
    for _, b := range binds {
        parts := strings.SplitN(b, ":", 3)
        if len(parts) < 2 {
            newBinds = append(newBinds, b)
            continue
        }
        local := parts[0]
        remote := parts[1]
        mode := ""
        if len(parts) == 3 {
            mode = parts[2]
        }

        if !isLocalPath(local) {
            // Assume it's already a remote path, pass through.
            newBinds = append(newBinds, b)
            continue
        }

        absLocal, err := filepath.Abs(local)
        if err != nil {
            return nil, fmt.Errorf("abs local path: %w", err)
        }
        info, err := os.Stat(absLocal)
        if err != nil {
            return nil, fmt.Errorf("stat local path: %w", err)
        }
        if !info.IsDir() {
            // For simplicity, handle only directories; files are similar.
        }

        remoteTemp := fmt.Sprintf("/tmp/agent-sync-%x", hashPath(absLocal)) // implement hashPath yourself

        if err := p.syncLocalDirToRemote(absLocal, remoteTemp); err != nil {
            return nil, fmt.Errorf("sync %s -> %s: %w", absLocal, remoteTemp, err)
        }

        if mode != "" {
            newBinds = append(newBinds, fmt.Sprintf("%s:%s:%s", remoteTemp, remote, mode))
        } else {
            newBinds = append(newBinds, fmt.Sprintf("%s:%s", remoteTemp, remote))
        }
    }
    return newBinds, nil
}

func isLocalPath(p string) bool {
    // Rough heuristic: treat anything starting with ./, ../, or / as local.
    return strings.HasPrefix(p, ".") || strings.HasPrefix(p, "/")
}

// syncLocalDirToRemote: simple SFTP/rsync stub; you’d implement using SSH client.
func (p *DockerProxy) syncLocalDirToRemote(localDir, remoteDir string) error {
    // For now, stub; you'd use sftp.NewClient(p.sshClient.client) and walk localDir.
    // Example: mkdir -p remoteDir, then copy all files/dirs.
    fmt.Printf("SYNC: %s -> %s (stub)\n", localDir, remoteDir)
    return nil
}

func hashPath(s string) uint64 {
    // Very naive hash; replace with a better one.
    var h uint64 = 1469598103934665603
    for i := 0; i < len(s); i++ {
        h ^= uint64(s[i])
        h *= 1099511628211
    }
    return h
}
