package doctor

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func restEnv(url string) func(string) string {
	values := map[string]string{
		"SLURM_REST_URL": url, "SLURM_API_VERSION": "v0.0.43",
		"SLURM_USER": "alice", "SLURM_JWT": "secret-token",
	}
	return func(key string) string { return values[key] }
}

func TestRESTProbeHTTP(t *testing.T) {
	tests := []struct {
		name   string
		code   int
		body   string
		status Status
		want   string
	}{
		{"ok", http.StatusOK, "pong", OK, "reachable"},
		{"unauthorized", http.StatusUnauthorized, "", FAIL, "authentication rejected"},
		{"not found", http.StatusNotFound, `{"versions":["v0.0.42"]}`, FAIL, "advertised versions: v0.0.42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser, gotToken string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotUser = r.Header.Get("X-SLURM-USER-NAME")
				gotToken = r.Header.Get("X-SLURM-USER-TOKEN")
				if r.URL.Path == "/openapi/v3" {
					_, _ = w.Write([]byte(`{"versions":["v0.0.42"]}`))
					return
				}
				w.WriteHeader(tt.code)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			result := (restProbeCheck{env: restEnv(srv.URL)}).Run(context.Background())
			if result.Status != tt.status || !strings.Contains(result.Detail+result.Hint, tt.want) {
				t.Fatalf("result %#v, want %s containing %q", result, tt.status, tt.want)
			}
			if gotUser != "alice" || gotToken != "secret-token" {
				t.Fatalf("headers user=%q token=%q", gotUser, gotToken)
			}
			if strings.Contains(result.Detail+result.Hint, "secret-token") {
				t.Fatal("token leaked in result")
			}
		})
	}
}

func TestRESTProbeSocketAndFailures(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "slurm.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go server.Serve(listener)
	env := func(key string) string {
		return map[string]string{
			"SLURM_SOCKET": socket, "SLURM_API_VERSION": "v0.0.43",
			"SLURM_USER": "alice", "SLURM_JWT": "secret-token",
		}[key]
	}
	if got := (restProbeCheck{env: env}).Run(context.Background()); got.Status != OK {
		t.Fatalf("socket result: %#v", got)
	}
	_ = server.Shutdown(context.Background())

	refused := func(key string) string {
		return map[string]string{
			"SLURM_REST_URL": "http://127.0.0.1:1", "SLURM_API_VERSION": "v0.0.43",
			"SLURM_USER": "alice", "SLURM_JWT": "secret-token",
		}[key]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	got := (restProbeCheck{env: refused}).Run(ctx)
	if got.Status != FAIL || strings.Contains(got.Detail+got.Hint, "secret-token") {
		t.Fatalf("refused result: %#v", got)
	}
}

func TestRESTProbeMissingAndOversized(t *testing.T) {
	missing := (restProbeCheck{env: func(key string) string {
		if key == "SLURM_USER" {
			return "alice"
		}
		return ""
	}}).Run(context.Background())
	if missing.Status != FAIL || !strings.Contains(missing.Detail, "SLURM_API_VERSION") {
		t.Fatalf("missing result: %#v", missing)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, 1<<20+1))
	}))
	defer srv.Close()
	got := (restProbeCheck{env: restEnv(srv.URL)}).Run(context.Background())
	if got.Status != FAIL {
		t.Fatalf("oversized result: %#v", got)
	}
}
