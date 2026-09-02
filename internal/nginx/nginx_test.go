package nginx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenpingonline/fn-nginx-web/internal/domain"
)

func TestRenderUsesOnlyFnProxyPaths(t *testing.T) {
	root := t.TempDir()
	paths := Paths{
		AppDest:        filepath.Join(root, "app"),
		EtcDir:         filepath.Join(root, "etc"),
		VarDir:         filepath.Join(root, "var"),
		TmpDir:         filepath.Join(root, "tmp"),
		NginxBin:       filepath.Join(root, "app", "bin", "nginx"),
		MimeTypes:      filepath.Join(root, "app", "etc", "mime.types"),
		StateFile:      filepath.Join(root, "var", "fnproxy.json"),
		CertificateDir: filepath.Join(root, "var", "certificates"),
		RevisionDir:    filepath.Join(root, "etc", "nginx", "revisions"),
		NginxPrefix:    filepath.Join(root, "var", "nginx"),
		NginxConfigDir: filepath.Join(root, "etc", "nginx"),
		NginxConfD:     filepath.Join(root, "etc", "nginx", "conf.d"),
		NginxMaster:    filepath.Join(root, "etc", "nginx", "nginx.conf"),
		NginxRunDir:    filepath.Join(root, "var", "nginx", "run"),
		NginxPID:       filepath.Join(root, "var", "nginx", "run", "nginx.pid"),
		NginxLogDir:    filepath.Join(root, "var", "logs"),
		NginxAccessLog: filepath.Join(root, "var", "logs", "nginx-access.log"),
		NginxErrorLog:  filepath.Join(root, "var", "logs", "nginx-error.log"),
		NginxTempDir:   filepath.Join(root, "tmp", "nginx"),
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.MimeTypes), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.MimeTypes, []byte("types { text/plain txt; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := domain.DefaultState()
	state.Settings.DefaultHTTPPort = 19080
	rule := domain.ProxyRule{
		ID: "0123456789ab", Name: "Demo", Enabled: true, ListenPort: 19080,
		Domains: []string{"proxy.example.com"}, UpstreamScheme: "http",
		UpstreamHost: "127.0.0.1", UpstreamPort: 8080, PreserveHost: true, WebSocket: true,
		ConnectTimeoutSeconds: 10, ReadTimeoutSeconds: 60, SendTimeoutSeconds: 60,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	rule.UpstreamHost = "::1"
	state.Rules = []domain.ProxyRule{rule}

	manager := New(paths)
	master, files, err := manager.render(state, paths.NginxConfD)
	if err != nil {
		t.Fatal(err)
	}
	all := master
	for _, content := range files {
		all += content
	}
	for _, forbidden := range []string{"include \"/etc/nginx", "/usr/trim/nginx", "pid \"/var/run/nginx.pid"} {
		if strings.Contains(all, forbidden) {
			t.Fatalf("generated config contains system path %s", forbidden)
		}
	}
	for _, expected := range []string{paths.NginxPID, paths.NginxErrorLog, "listen 19080", "proxy_pass http://[::1]:8080", "proxy_set_header Upgrade"} {
		if !strings.Contains(all, expected) {
			t.Fatalf("generated config missing %q:\n%s", expected, all)
		}
	}
}

func TestLastNginxErrorIgnoresNotice(t *testing.T) {
	lines := []string{
		"2026/01/01 [notice] start worker process",
		"2026/01/01 [error] upstream timed out",
		"2026/01/01 [notice] signal process started",
	}
	if got := lastNginxError(lines); !strings.Contains(got, "upstream timed out") {
		t.Fatalf("unexpected error line: %q", got)
	}
}
