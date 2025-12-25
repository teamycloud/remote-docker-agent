package mtlsproxy

import (
	"testing"
)

func TestExtractPathFromRequestLine(t *testing.T) {
	tests := []struct {
		name        string
		requestLine string
		want        string
		wantErr     bool
	}{
		{
			name:        "Docker API request",
			requestLine: "GET /v1/containers/json HTTP/1.1",
			want:        "/v1/containers/json",
			wantErr:     false,
		},
		{
			name:        "Mutagen request",
			requestLine: "POST /tinyscale/v1/host-exec/run HTTP/1.1",
			want:        "/tinyscale/v1/host-exec/run",
			wantErr:     false,
		},
		{
			name:        "Request with query string",
			requestLine: "GET /v1/images/json?all=1 HTTP/1.1",
			want:        "/v1/images/json",
			wantErr:     false,
		},
		{
			name:        "Request with newline",
			requestLine: "GET /v1/info HTTP/1.1\r\n",
			want:        "/v1/info",
			wantErr:     false,
		},
		{
			name:        "Invalid request line",
			requestLine: "INVALID",
			want:        "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractPathFromRequestLine(tt.requestLine)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractPathFromRequestLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractPathFromRequestLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHTTPRouter_DeterminePort(t *testing.T) {
	router := NewHTTPRouter("192.168.1.100")

	tests := []struct {
		name string
		path string
		want int
	}{
		{
			name: "Mutagen host-exec request",
			path: "/tinyscale/v1/host-exec/run",
			want: MutagenPort,
		},
		{
			name: "Mutagen host-exec list",
			path: "/tinyscale/v1/host-exec/sessions",
			want: MutagenPort,
		},
		{
			name: "Docker container list",
			path: "/v1/containers/json",
			want: DockerPort,
		},
		{
			name: "Docker images",
			path: "/v1/images/json",
			want: DockerPort,
		},
		{
			name: "Docker info",
			path: "/v1/info",
			want: DockerPort,
		},
		{
			name: "Docker version",
			path: "/v1/version",
			want: DockerPort,
		},
		{
			name: "Unknown path defaults to Docker",
			path: "/unknown/path",
			want: DockerPort,
		},
		{
			name: "Root path defaults to Docker",
			path: "/",
			want: DockerPort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := router.determinePort(tt.path); got != tt.want {
				t.Errorf("HTTPRouter.determinePort() = %v, want %v", got, tt.want)
			}
		})
	}
}
