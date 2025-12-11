package agent

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net"
    "net/http"
    "net/url"

    "github.com/docker/docker/api/types/container"
)

type DockerProxy struct {
    cfg       Config
    sshClient *SSHClient
}

func NewDockerProxy(cfg Config, sshClient *SSHClient) *DockerProxy {
    return &DockerProxy{
        cfg:       cfg,
        sshClient: sshClient,
    }
}

// HandleCreateContainer adds port-forward + bind-mount logic, then proxies.
func (p *DockerProxy) HandleCreateContainer(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "read body error", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()

    var req struct {
        Config     container.Config       `json:"Config"`
        HostConfig container.HostConfig   `json:"HostConfig"`
        // other fields omitted
    }

    if err := json.Unmarshal(body, &req); err != nil {
        http.Error(w, "invalid JSON", http.StatusBadRequest)
        return
    }

    // 1. Handle port forwarding
    if err := p.setupPortForwards(&req.HostConfig); err != nil {
        http.Error(w, fmt.Sprintf("port forward error: %v", err), http.StatusInternalServerError)
        return
    }

    // 2. Handle local bind mounts -> remote paths
    newBinds, err := p.rewriteBindMounts(req.HostConfig.Binds)
    if err != nil {
        http.Error(w, fmt.Sprintf("bind rewrite error: %v", err), http.StatusInternalServerError)
        return
    }
    req.HostConfig.Binds = newBinds

    // 3. Re-marshal modified request
    newBody, err := json.Marshal(req)
    if err != nil {
        http.Error(w, "marshal error", http.StatusInternalServerError)
        return
    }

    // 4. Proxy to remote Docker
    resp, err := p.proxyRawRequest(r.Method, r.URL, r.Header, newBody)
    if err != nil {
        http.Error(w, fmt.Sprintf("proxy error: %v", err), http.StatusBadGateway)
        return
    }
    defer resp.Body.Close()

    copyHeaders(w.Header(), resp.Header)
    w.WriteHeader(resp.StatusCode)
    io.Copy(w, resp.Body)
}

func (p *DockerProxy) HandleGeneric(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "read body error", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()

    resp, err := p.proxyRawRequest(r.Method, r.URL, r.Header, body)
    if err != nil {
        http.Error(w, fmt.Sprintf("proxy error: %v", err), http.StatusBadGateway)
        return
    }
    defer resp.Body.Close()

    copyHeaders(w.Header(), resp.Header)
    w.WriteHeader(resp.StatusCode)
    io.Copy(w, resp.Body)
}

func (p *DockerProxy) proxyRawRequest(method string, u *url.URL, hdr http.Header, body []byte) (*http.Response, error) {
    // Dial remote Docker via SSH
    conn, err := p.sshClient.DialRemoteDocker()
    if err != nil {
        return nil, err
    }

    transport := &http.Transport{
        DisableKeepAlives:  true,
        DisableCompression: true,
        DialContext: func(_ net.Context, _, _ string) (net.Conn, error) {
            return conn, nil
        },
    }

    client := &http.Client{Transport: transport}

    remoteURL := &url.URL{
        Scheme: "http",
        Host:   "docker", // ignored due to custom DialContext
        Path:   u.Path,
        RawQuery: u.RawQuery,
    }

    req, err := http.NewRequest(method, remoteURL.String(), bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header = hdr.Clone()

    return client.Do(req)
}

func copyHeaders(dst, src http.Header) {
    for k, vv := range src {
        for _, v := range vv {
            dst.Add(k, v)
        }
    }
}
