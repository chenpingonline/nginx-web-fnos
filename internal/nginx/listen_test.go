package nginx

import (
	"fmt"
	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenderListenFamilies(t *testing.T) {
	for _, tc := range []struct {
		kind   string
		v4, v6 bool
	}{{"", true, false}, {"ipv4", true, false}, {"ipv6", false, true}, {"dual", true, true}} {
		t.Run(tc.kind, func(t *testing.T) {
			s := domain.DefaultState()
			r := domain.ProxyRule{ID: "0123456789ab", Name: "music", Enabled: true, ListenType: tc.kind, ListenPort: 9560, Domains: []string{"music.example.com"}, UpstreamHost: "192.168.1.88", UpstreamPort: 4533}
			domain.NormalizeRule(&r, s.Settings)
			s.Rules = []domain.ProxyRule{r}
			m := New(Paths{})
			_, files, err := m.render(s, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			all := ""
			for _, v := range files {
				all += v
			}
			if strings.Contains(all, "listen 9560;") != tc.v4 || strings.Contains(all, "listen [::]:9560;") != tc.v6 {
				t.Fatal(all)
			}
			if strings.Contains(all, "listen 9560 default_server;") != tc.v4 || strings.Contains(all, "listen [::]:9560 default_server;") != tc.v6 {
				t.Fatal("default listener mismatch", all)
			}
			if !strings.Contains(all, "proxy_pass http://192.168.1.88:4533") {
				t.Fatal("IPv4 upstream missing", all)
			}
			r.Domains = []string{"*"}
			s.Rules = []domain.ProxyRule{r}
			_, files, err = m.render(s, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			for name := range files {
				if strings.HasPrefix(name, "000-default-") {
					t.Fatal("duplicate default", name)
				}
			}
		})
	}
}
func TestTLSListenSuffix(t *testing.T) {
	var b strings.Builder
	writeHTTPListeners(&b, "dual", 9560, " ssl default_server")
	if b.String() != "    listen 9560 ssl default_server;\n    listen [::]:9560 ssl default_server;\n" {
		t.Fatal(b.String())
	}
}

func TestRenderStandardHTTPAndHTTPSPorts(t *testing.T) {
	var b strings.Builder
	writeHTTPListeners(&b, "ipv4", 80, "")
	writeHTTPListeners(&b, "ipv4", 443, " ssl")
	if b.String() != "    listen 80;\n    listen 443 ssl;\n" {
		t.Fatal(b.String())
	}
}

func TestIPv6ToIPv4WithRealNginx(t *testing.T) {
	bin := os.Getenv("NGINX_TEST_BIN")
	if bin == "" {
		t.Skip("set NGINX_TEST_BIN for real IPv6 proxy verification")
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ipv4-backend") }))
	defer backend.Close()
	ln, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	root := t.TempDir()
	for _, dir := range []string{"logs", "temp/body", "temp/proxy", "temp/fastcgi", "temp/scgi", "temp/uwsgi"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	var listeners strings.Builder
	writeHTTPListeners(&listeners, "dual", port, "")
	conf := fmt.Sprintf("daemon off; master_process off; pid nginx.pid; error_log stderr; events {} http { access_log off; server { %s location / { proxy_pass %s; } } }", listeners.String(), backend.URL)
	path := filepath.Join(root, "nginx.conf")
	if err := os.WriteFile(path, []byte(conf), 0600); err != nil {
		t.Fatal(err)
	}
	log, err := os.Create(filepath.Join(root, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	cmd := exec.Command(bin, "-p", root+"/", "-c", path)
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Signal(os.Interrupt); _ = cmd.Wait() }()
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	for _, host := range []string{"[::1]", "127.0.0.1"} {
		success := false
		for i := 0; i < 50; i++ {
			resp, e := client.Get(fmt.Sprintf("http://%s:%d/", host, port))
			if e == nil {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if string(body) != "ipv4-backend" {
					t.Fatal(string(body))
				}
				success = true
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if !success {
			data, _ := os.ReadFile(log.Name())
			t.Fatalf("%s failed: %s", host, data)
		}
	}
}
