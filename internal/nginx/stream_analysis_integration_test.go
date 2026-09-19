package nginx

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/metrics"
	"github.com/chenpingonline/nginx-web-fnos/internal/platform"
)

func TestStreamAnalysisWithRealNginx(t *testing.T) {
	bin := os.Getenv("NGINX_TEST_BIN")
	if bin == "" {
		t.Skip("set NGINX_TEST_BIN for real TCP/UDP integration")
	}
	root, err := os.MkdirTemp("/tmp", "nginx-stream-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	for key, dir := range map[string]string{"FNPROXY_APPDEST": "app", "FNPROXY_ETC": "etc", "FNPROXY_VAR": "var", "FNPROXY_TMP": "tmp"} {
		t.Setenv(key, filepath.Join(root, dir))
	}
	paths, err := platform.LoadPaths()
	if err != nil {
		t.Fatal(err)
	}
	paths.NginxBin = bin
	manager := New(paths)
	os.MkdirAll(filepath.Dir(paths.MimeTypes), 0750)
	os.WriteFile(paths.MimeTypes, []byte("types {}\n"), 0600)
	tcp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	go func() {
		for {
			conn, err := tcp.Accept()
			if err != nil {
				return
			}
			go func() { defer conn.Close(); io.Copy(conn, conn) }()
		}
	}()
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	go func() {
		buf := make([]byte, 256)
		for {
			n, addr, err := udp.ReadFrom(buf)
			if err != nil {
				return
			}
			udp.WriteTo(buf[:n], addr)
		}
	}()
	reserve := func() int {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		p := l.Addr().(*net.TCPAddr).Port
		l.Close()
		return p
	}
	state := domain.DefaultState()
	state.Settings.DefaultHTTPPort = reserve()
	state.Settings.WorkerProcesses = 1
	ports := map[string]int{"tcp": reserve(), "udp": reserve()}
	for _, protocol := range []string{"tcp", "udp"} {
		port := tcp.Addr().(*net.TCPAddr).Port
		if protocol == "udp" {
			port = udp.LocalAddr().(*net.UDPAddr).Port
		}
		r := domain.StreamRule{ID: fmt.Sprintf("%012d", port), Name: protocol, Enabled: true, Protocol: protocol, ListenAddress: "127.0.0.1", ListenPort: ports[protocol], UpstreamHost: "127.0.0.1", UpstreamPort: port, AccessLog: true, UDPResponses: 1, ProxyTimeoutSeconds: 2, MaxConnections: 1}
		domain.NormalizeStreamRule(&r)
		state.StreamRules = append(state.StreamRules, r)
	}
	collector := metrics.NewStream(paths.StreamMetricsLog(), paths.StreamMetricsHistory(), time.Now())
	master, files, err := manager.render(state, paths.NginxConfD)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(paths.NginxMaster, []byte(master), 0600)
	for name, content := range files {
		os.WriteFile(filepath.Join(paths.NginxConfD, name), []byte(content), 0600)
	}
	run := func(args ...string) {
		t.Helper()
		out, e := exec.Command(bin, append([]string{"-p", paths.NginxPrefix + "/", "-c", paths.NginxMaster}, args...)...).CombinedOutput()
		if e != nil {
			t.Fatalf("nginx: %v %s", e, out)
		}
	}
	run("-t")
	run()
	defer exec.Command(bin, "-p", paths.NginxPrefix+"/", "-c", paths.NginxMaster, "-s", "stop").Run()
	for _, protocol := range []string{"tcp", "udp"} {
		conn, e := net.DialTimeout(protocol, fmt.Sprintf("127.0.0.1:%d", ports[protocol]), time.Second)
		if e != nil {
			t.Fatal(e)
		}
		conn.SetDeadline(time.Now().Add(3 * time.Second))
		conn.Write([]byte("ping"))
		buf := make([]byte, 4)
		if _, e = io.ReadFull(conn, buf); e != nil || string(buf) != "ping" {
			t.Fatalf("%s echo: %q %v", protocol, buf, e)
		}
		if protocol == "tcp" {
			blocked, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", ports[protocol]), time.Second)
			if e != nil {
				t.Fatal(e)
			}
			blocked.SetDeadline(time.Now().Add(time.Second))
			blocked.Write([]byte("blocked"))
			blocked.Read(buf)
			blocked.Close()
		}
		conn.Close()
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		collector.Collect(time.Now())
		r := collector.Snapshot(time.Now(), 15, "")
		if r.Sessions >= 3 {
			if r.Sessions != 3 || r.Rejected != 1 || r.Errors != 1 || r.Sent != 8 || r.Received != 8 {
				t.Fatalf("wrong metrics: %+v", r)
			}
			if len(r.Rules) != 2 || len(r.Recent) != 3 {
				t.Fatalf("missing protocols: %+v", r)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("sessions missing: %+v", collector.Snapshot(time.Now(), 15, ""))
}
