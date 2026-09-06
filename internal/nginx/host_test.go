package nginx

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
)

func TestPreservedHostWithRealNginx(t *testing.T) {
	bin := os.Getenv("NGINX_TEST_BIN")
	if bin == "" {
		t.Skip("set NGINX_TEST_BIN for real proxy header verification")
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"host": r.Host, "forwarded": r.Header.Get("X-Forwarded-Host"), "origin": r.Header.Get("Origin")})
	}))
	defer backend.Close()
	manager := New(Paths{})
	master, _, err := manager.render(domain.DefaultState(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(master, "map $http_host $fnproxy_client_host {")
	if start < 0 {
		t.Fatal("missing original Host mapping")
	}
	mapping := master[start : start+strings.Index(master[start:], "}")+1]
	root := t.TempDir()
	// Match the bundled Nginx core's relative log and temporary paths.
	for _, dir := range []string{"logs", "temp/body", "temp/proxy", "temp/fastcgi", "temp/scgi", "temp/uwsgi"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := ln.Addr().String()
	ln.Close()
	var locations strings.Builder
	for _, preserve := range []bool{true, false} {
		var settings strings.Builder
		manager.renderProxySettings(&settings, domain.ProxyRule{PreserveHost: preserve, ConnectTimeoutSeconds: 5, ReadTimeoutSeconds: 5, SendTimeoutSeconds: 5}, domain.LocationSettings{}, "")
		fmt.Fprintf(&locations, "location /%t {\n%s\nproxy_pass %s;\n}\n", preserve, settings.String(), backend.URL)
	}
	conf := fmt.Sprintf("daemon off; master_process off; pid %s; error_log stderr; events {} http { access_log off; %s server { listen %s; server_name fallback.example; %s } }", filepath.Join(root, "nginx.pid"), mapping, address, locations.String())
	path := filepath.Join(root, "nginx.conf")
	if err = os.WriteFile(path, []byte(conf), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-p", root+"/", "-c", path)
	logFile, err := os.Create(filepath.Join(root, "nginx.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Signal(os.Interrupt); _ = cmd.Wait() }()
	ready := false
	for i := 0; i < 100; i++ {
		c, e := net.DialTimeout("tcp", address, 50*time.Millisecond)
		if e == nil {
			c.Close()
			ready = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		b, _ := os.ReadFile(logFile.Name())
		t.Fatal(string(b))
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, host := range []string{"192.168.1.29:9097", "Example.COM:9097", "[2001:db8::1]:9097", "example.com"} {
		for _, preserve := range []bool{true, false} {
			req, _ := http.NewRequest("GET", fmt.Sprintf("http://%s/%t", address, preserve), nil)
			req.Host = host
			req.Header.Set("Origin", "http://"+host)
			resp, e := client.Do(req)
			if e != nil {
				t.Fatal(e)
			}
			var got map[string]string
			e = json.NewDecoder(resp.Body).Decode(&got)
			resp.Body.Close()
			if e != nil {
				t.Fatal(e)
			}
			want := host
			if !preserve {
				want = strings.TrimPrefix(backend.URL, "http://")
			}
			if got["host"] != want || got["forwarded"] != host || got["origin"] != "http://"+host {
				t.Fatalf("preserve=%v host=%s: %v", preserve, host, got)
			}
		}
	}
	c, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprint(c, "GET /true HTTP/1.0\r\n\r\n")
	resp, err := http.ReadResponse(bufio.NewReader(c), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got map[string]string
	if err = json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["host"] != "fallback.example" || got["forwarded"] != "fallback.example" {
		t.Fatal("missing-host fallback", got)
	}
}
