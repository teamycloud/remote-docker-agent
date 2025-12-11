package agent

import (
    "net/http"
)

func NewRouter(proxy *DockerProxy) http.Handler {
    mux := http.NewServeMux()

    // Generic catch-all, proxy to Docker API, but with special handling
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        switch {
        case r.Method == http.MethodPost && r.URL.Path == "/containers/create":
            proxy.HandleCreateContainer(w, r)
        default:
            proxy.HandleGeneric(w, r)
        }
    })

    return mux
}
