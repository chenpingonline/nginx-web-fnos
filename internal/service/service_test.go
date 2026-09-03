package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenpingonline/fn-nginx-web/internal/domain"
	"github.com/chenpingonline/fn-nginx-web/internal/platform"
)

func testService(t *testing.T) *AppService {
	t.Helper()
	root := t.TempDir()
	t.Setenv("FNPROXY_APPDEST", filepath.Join(root, "app"))
	t.Setenv("FNPROXY_ETC", filepath.Join(root, "etc"))
	t.Setenv("FNPROXY_VAR", filepath.Join(root, "var"))
	t.Setenv("FNPROXY_TMP", filepath.Join(root, "tmp"))
	paths, err := platform.LoadPaths()
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(paths)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestCacheCleanupAndLogRotationStayInsideAppData(t *testing.T) {
	service := testService(t)
	cacheFile := filepath.Join(service.paths.NginxCacheDir, "rule", "cache.data")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, []byte("cache"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.ClearCache(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Fatalf("cache file still exists: %v", err)
	}

	if err := os.WriteFile(service.paths.NginxAccessLog, []byte("line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rotated, err := service.RotateLogs(true)
	if err != nil {
		t.Fatal(err)
	}
	if !rotated {
		t.Fatal("expected log rotation")
	}
	if _, err := os.Stat(service.paths.NginxAccessLog + ".1"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSettingsValidatesBeforePersisting(t *testing.T) {
	service := testService(t)
	invalid := service.State().Settings
	invalid.DefaultHTTPPort = 80
	if err := service.UpdateSettings(invalid); err == nil {
		t.Fatal("expected privileged port to be rejected")
	}
	if got := service.State().Settings.DefaultHTTPPort; got != 9080 {
		t.Fatalf("invalid settings changed persisted state: %d", got)
	}
}

func TestCreateRuleNormalizesInput(t *testing.T) {
	service := testService(t)
	rule, err := service.CreateRule(domain.ProxyRule{
		Name: " Demo ", Enabled: true, ListenPort: 19080,
		Domains:        []string{"EXAMPLE.COM, example.com"},
		UpstreamScheme: "HTTP", UpstreamHost: "127.0.0.1", UpstreamPort: 8080,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rule.Name != "Demo" || len(rule.Domains) != 1 || rule.Domains[0] != "example.com" {
		t.Fatalf("rule was not normalized: %+v", rule)
	}
}
