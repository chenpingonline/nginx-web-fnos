package app

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestHealthCheckRequiresSuccessfulHTTPResponse(t *testing.T) {
	tempDir, err := os.MkdirTemp("/tmp", "nginx-web-health-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })
	socketPath := filepath.Join(tempDir, "app.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("unexpected health path %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	})}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()

	if err := HealthCheck(socketPath); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHealthCheckRejectsUnreachableSocket(t *testing.T) {
	err := HealthCheck(filepath.Join(t.TempDir(), "missing.sock"))
	if err == nil {
		t.Fatal("expected an unavailable socket to fail the health check")
	}
}
