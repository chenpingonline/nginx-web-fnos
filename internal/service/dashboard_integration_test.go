package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/metrics"
	nginxmanager "github.com/chenpingonline/nginx-web-fnos/internal/nginx"
)

func TestDashboardWithRealNginx(t *testing.T) {
	bin := os.Getenv("NGINX_TEST_BIN")
	if bin == "" {
		t.Skip("set NGINX_TEST_BIN to run a real proxy and verify dashboard traffic")
	}
	s := testService(t)
	s.paths.NginxBin = bin
	s.nginx = nginxmanager.New(s.paths)
	os.MkdirAll(filepath.Dir(s.paths.MimeTypes), 0o750)
	os.WriteFile(s.paths.MimeTypes, []byte("types { text/plain txt; }\n"), 0o640)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/failure" {
			w.WriteHeader(502)
		} else if r.URL.Path == "/missing" {
			w.WriteHeader(404)
		}
		fmt.Fprint(w, "dashboard integration")
	}))
	defer upstream.Close()
	_, upstreamPort, _ := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
	port, _ := strconv.Atoi(upstreamPort)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxyPort := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	settings := s.State().Settings
	settings.DefaultHTTPPort = proxyPort
	settings.WorkerProcesses = 1
	// Custom formats must not interfere with the fixed statistics format.
	settings.Logging.CustomFormat = "$status"
	if err := s.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, name := range []string{"alpha", "beta"} {
		rule, err := s.CreateRule(domain.ProxyRule{Name: name, Enabled: true, ListenPort: proxyPort, Domains: []string{name + ".test"}, UpstreamScheme: "http", UpstreamHost: "127.0.0.1", UpstreamPort: port})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, rule.ID)
	}
	if _, err := s.Apply("dashboard integration"); err != nil {
		t.Fatal(err)
	}
	defer s.nginx.Stop()
	collect := func() {
		status, err := s.nginx.BasicStatus(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		s.metrics.Collect(time.Now(), &metrics.Sample{PID: status.PID, Requests: status.Requests, Connections: status.Connections}, true)
	}
	collect()
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 3 * time.Second}
	for i := 0; i < 20; i++ {
		path, host := "/", "alpha.test"
		if i < 3 {
			path = "/failure"
		} else if i < 7 {
			path = "/missing"
		}
		if i >= 15 {
			host = "beta.test"
		}
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d%s", proxyPort, path), nil)
		req.Host = host
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if i < 3 && resp.StatusCode != 502 {
			t.Fatalf("expected 502, got %d", resp.StatusCode)
		}
		if i >= 3 && i < 7 && resp.StatusCode != 404 {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	}
	time.Sleep(1500 * time.Millisecond) // Nginx's fixed metrics log flush interval is 1 second.
	collect()
	d := s.Dashboard(60, "")
	if !d.MonitoringReady || !d.Metrics.Available || d.AppliedCount != 2 {
		t.Fatalf("dashboard unavailable: %+v", d)
	}
	if d.Metrics.Counts.Requests != 20 || d.Metrics.Counts.Errors != 3 || d.Metrics.Rules[ids[0]].Requests != 15 || d.Metrics.Rules[ids[1]].Requests != 5 {
		t.Fatalf("real traffic attribution failed: %+v", d.Metrics)
	}
	if d.Metrics.Counts.ClientErrors == nil || *d.Metrics.Counts.ClientErrors != 4 ||
		d.Metrics.ErrorRate == nil || *d.Metrics.ErrorRate != 35 ||
		d.Metrics.ClientErrorRate == nil || *d.Metrics.ClientErrorRate != 20 ||
		d.Metrics.ServerErrorRate == nil || *d.Metrics.ServerErrorRate != 15 {
		t.Fatalf("real traffic error categories failed: %+v", d.Metrics)
	}
	if alpha := d.Metrics.Rules[ids[0]]; alpha.ClientErrors == nil || *alpha.ClientErrors != 4 || alpha.Errors != 3 {
		t.Fatalf("rule error attribution failed: %+v", alpha)
	}
	if d.Metrics.RPS == nil || *d.Metrics.RPS <= 0 {
		t.Fatal("real status rate missing")
	}
	if d.Metrics.ResponseRPS == nil || *d.Metrics.ResponseRPS <= 0 {
		t.Fatal("completed response rate missing")
	}
	selected := s.Dashboard(60, ids[1])
	if selected.Metrics.ResponseRPS == nil || *selected.Metrics.ResponseRPS != *d.Metrics.ResponseRPS/4 {
		t.Fatal("live completed response rate was not attributed to the selected rule")
	}
	if runtime.GOOS == "linux" {
		status := d.Overview.Nginx
		if status.UptimeSeconds == nil || *status.UptimeSeconds < 1 || status.WorkerProcesses == nil || *status.WorkerProcesses != 1 {
			t.Fatalf("real Linux process statistics missing: %+v", status)
		}
	}
	if _, err := s.RotateLogs(true); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	collect()
	if d := s.Dashboard(60, ""); d.Metrics.Counts.Requests != 20 || d.Metrics.RPS == nil || *d.Metrics.RPS != 0 || d.Metrics.ResponseRPS == nil || *d.Metrics.ResponseRPS != 0 {
		t.Fatalf("rotation duplicated traffic or polling inflated rate: %+v", d.Metrics)
	}
}
