package service

import (
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
