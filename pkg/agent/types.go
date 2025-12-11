package agent

type Config struct {
    ListenAddr   string
    SSHUser      string
    SSHHost      string
    SSHKeyPath   string
    RemoteDocker string
}
