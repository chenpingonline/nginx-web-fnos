package nginx

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/metrics"
	"github.com/chenpingonline/nginx-web-fnos/internal/platform"
)

func TestLimitAnalysisWithRealNginx(t *testing.T) {
	bin := os.Getenv("NGINX_TEST_BIN")
	if bin == "" {
		t.Skip("set NGINX_TEST_BIN for isolated real Nginx limit tests")
	}

	root, err := os.MkdirTemp("/tmp", "nginx-limit-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	t.Setenv("FNPROXY_APPDEST", filepath.Join(root, "app"))
	t.Setenv("FNPROXY_ETC", filepath.Join(root, "etc"))
	t.Setenv("FNPROXY_VAR", filepath.Join(root, "var"))
	t.Setenv("FNPROXY_TMP", filepath.Join(root, "tmp"))
	paths, err := platform.LoadPaths()
	if err != nil {
		t.Fatal(err)
	}
	paths.NginxBin = bin
	manager := New(paths)
	if err := os.MkdirAll(filepath.Dir(paths.MimeTypes), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.MimeTypes, []byte("types { text/plain txt; }\n"), 0640); err != nil {
		t.Fatal(err)
	}
	state := domain.DefaultState()
	collector := metrics.New(paths.MetricsLog(), paths.MetricsHistory(), time.Now())
	entered, release := make(chan struct{}, 1), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hold" {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-release
		}
		if r.URL.Path == "/ordinary" {
			w.WriteHeader(503)
		}
		fmt.Fprint(w, "limit analysis integration")
	}))
	defer upstream.Close()
	defer unblock()
	_, portString, _ := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
	upstreamPort, _ := strconv.Atoi(portString)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	state.Settings.DefaultHTTPPort = port
	state.Settings.WorkerProcesses = 1
	rate := domain.RateLimitPolicy{ID: "111111111111", Name: "速率策略", Settings: domain.RateLimitSettings{Enabled: true, RequestsPerSecond: 1, Burst: 1}}
	conn := domain.RateLimitPolicy{ID: "222222222222", Name: "并发策略", Settings: domain.RateLimitSettings{Enabled: true, RequestsPerSecond: 1000, Burst: 100, NoDelay: true, Connections: 1}}
	state.RateLimitPolicies = []domain.RateLimitPolicy{rate, conn}
	ids := map[string]string{}
	for host, policy := range map[string]string{"rate.test": rate.ID, "conn.test": conn.ID, "plain.test": ""} {
		id := fmt.Sprintf("%012d", len(state.Rules)+10)
		state.Rules = append(state.Rules, domain.ProxyRule{ID: id, Name: host, Enabled: true, ListenPort: port, Domains: []string{host}, UpstreamScheme: "http", UpstreamHost: "127.0.0.1", UpstreamPort: upstreamPort, RateLimitPolicyID: policy})
		ids[host] = id
	}
	master, files, err := manager.render(state, paths.NginxConfD)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.NginxMaster, []byte(master), 0600); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(paths.NginxConfD, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) {
		t.Helper()
		out, err := exec.Command(bin, append([]string{"-p", paths.NginxPrefix + "/", "-c", paths.NginxMaster}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("nginx: %v: %s", err, out)
		}
	}
	// Exercise generated directives directly on the explicitly supplied test
	// binary. Production's pinned-version check remains unchanged.
	run("-t")
	run()
	defer exec.Command(bin, "-p", paths.NginxPrefix+"/", "-c", paths.NginxMaster, "-s", "quit").Run()
	collector.Collect(time.Now(), nil, true)
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 6 * time.Second}
	send := func(host, path string) (int, error) {
		r, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d%s", port, path), nil)
		r.Host = host
		resp, err := client.Do(r)
		if err != nil {
			return 0, err
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp.StatusCode, nil
	}
	if status, err := send("plain.test", "/ordinary"); err != nil || status != 503 {
		t.Fatalf("ordinary response: %d %v", status, err)
	}
	type outcome struct {
		status int
		err    error
	}
	done := make(chan outcome, 4)
	for i := 0; i < 3; i++ {
		go func() { status, err := send("rate.test", "/rate"); done <- outcome{status, err} }()
	}
	rateRejected := 0
	for i := 0; i < 3; i++ {
		r := <-done
		if r.err != nil {
			t.Fatal(r.err)
		}
		if r.status == 503 {
			rateRejected++
		}
	}
	if rateRejected != 1 {
		t.Fatalf("rate limit expected 1 refusal, got %d", rateRejected)
	}
	go func() { status, err := send("conn.test", "/hold"); done <- outcome{status, err} }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream did not receive held request")
	}
	if status, err := send("conn.test", "/second"); err != nil || status != 503 {
		t.Fatalf("connection limit: %d %v", status, err)
	}
	unblock()
	if r := <-done; r.err != nil || r.status != 200 {
		t.Fatalf("held request: %+v", r)
	}
	time.Sleep(1500 * time.Millisecond)
	collector.Collect(time.Now(), nil, true)
	limits := collector.Snapshot(time.Now(), 60, "").Analysis.Limits
	if limits.Total != 6 || limits.Covered != 6 || limits.Rejected != 2 || limits.RequestRejected != 1 || limits.ConnectionRejected != 1 || limits.Delayed != 1 || limits.AffectedRules != 2 {
		t.Fatalf("real Nginx limit attribution: %+v", limits)
	}
	plain := collector.Snapshot(time.Now(), 60, ids["plain.test"]).Analysis.Limits
	if plain.Covered != 1 || plain.Rejected != 0 {
		t.Fatalf("ordinary 503 misclassified: %+v", plain)
	}
	for _, sample := range limits.Recent {
		if sample.Policy == nil || sample.Policy.Name == "" {
			t.Fatalf("applied policy snapshot missing: %+v", sample)
		}
	}
}
