package agent

import (
    "log"
    "net/http"
)

type Server struct {
    cfg    Config
    router http.Handler
}

func NewServer(cfg Config) (*Server, error) {
    sshClient, err := NewSSHClient(cfg)
    if err != nil {
        return nil, err
    }

    proxy := NewDockerProxy(cfg, sshClient)
    router := NewRouter(proxy)

    return &Server{
        cfg:    cfg,
        router: router,
    }, nil
}

func (s *Server) ListenAndServe() error {
    server := &http.Server{
        Addr:    s.cfg.ListenAddr,
        Handler: s.router,
    }

    log.Printf("Starting HTTP server on %s", s.cfg.ListenAddr)
    return server.ListenAndServe()
}
