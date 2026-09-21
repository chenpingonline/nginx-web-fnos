package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/platform"
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

func TestCreateRuleRequiresAuthorizedStaticDirectory(t *testing.T) {
	service := testService(t)
	staticDir := t.TempDir()
	input := domain.ProxyRule{
		Name: "Static", Enabled: true, ListenPort: 19080, Domains: []string{"static.example.com"},
		UpstreamScheme: "http", UpstreamHost: "127.0.0.1", UpstreamPort: 8080,
		RootLocation: domain.LocationSettings{BackendType: "static", StaticPath: staticDir},
	}
	if _, err := service.CreateRule(input); err == nil {
		t.Fatal("unauthorized static directory was accepted")
	}
	if len(service.State().Rules) != 0 {
		t.Fatal("rejected static rule changed persisted state")
	}
	t.Setenv("TRIM_DATA_ACCESSIBLE_PATHS", staticDir)
	if _, err := service.CreateRule(input); err != nil {
		t.Fatalf("authorized static directory rejected: %v", err)
	}
}

func TestRestoredDraftSourcePersistsAcrossEditsAndReopen(t *testing.T) {
	service := testService(t)
	original := service.State()
	if err := service.saveRevision(original, "first"); err != nil {
		t.Fatal(err)
	}
	revisions, err := service.ListRevisions()
	if err != nil || len(revisions) != 1 {
		t.Fatalf("list revisions: %v, count %d", err, len(revisions))
	}
	id := revisions[0].ID
	restored, err := service.RestoreRevision(id)
	if err != nil || !restored.Dirty || restored.DraftRevisionID != id {
		t.Fatalf("restore state: %+v, %v", restored, err)
	}
	settings := restored.Settings
	settings.RevisionLimit = 30
	if err := service.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(service.paths)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.State(); !got.Dirty || got.DraftRevisionID != id || got.Settings.RevisionLimit != 30 {
		t.Fatalf("draft/source not retained after edit and reopen: %+v", got)
	}
	if _, err := reopened.RestoreRevision("missing"); err == nil {
		t.Fatal("expected missing revision error")
	}
	if got := reopened.State(); got.DraftRevisionID != id || got.Settings.RevisionLimit != 30 {
		t.Fatal("failed restore changed current draft")
	}
}

func TestDiscardDraftRestoresAppliedStateWithoutCreatingRevision(t *testing.T) {
	service := testService(t)
	applied := service.State()
	now := time.Now().UTC()
	applied.LastAppliedAt = &now
	applied.LastApplyMessage = "配置已应用"
	if err := service.store.Replace(applied); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(applied)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(service.paths.AppliedState(), data, 0600); err != nil {
		t.Fatal(err)
	}
	settings := applied.Settings
	settings.RevisionLimit = 30
	if err = service.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err = service.store.Update(func(state *State) error {
		state.DraftRevisionID = "restored-source"
		state.Certificates = append(state.Certificates, CertificateMeta{ID: "0123456789abcdef", Name: "draft-certificate"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	discarded, err := service.DiscardDraft()
	if err != nil {
		t.Fatal(err)
	}
	if discarded.Dirty || discarded.DraftRevisionID != "" || discarded.Settings.RevisionLimit != applied.Settings.RevisionLimit {
		t.Fatalf("draft was not restored to applied state: %+v", discarded)
	}
	if len(discarded.Certificates) != 1 || discarded.Certificates[0].Name != "draft-certificate" {
		t.Fatalf("discard removed current certificate inventory: %+v", discarded.Certificates)
	}
	if discarded.LastApplyMessage != "已放弃未应用的草稿修改" || discarded.LastApplyError != "" {
		t.Fatalf("discard status was not recorded: %+v", discarded)
	}
	revisions, err := service.ListRevisions()
	if err != nil || len(revisions) != 0 {
		t.Fatalf("discard created configuration history: %v, %+v", err, revisions)
	}
	if _, err = service.DiscardDraft(); err == nil {
		t.Fatal("discard accepted without a pending draft")
	}
}

func TestSettingsCandidateWithRealNginxDoesNotPersist(t *testing.T) {
	binary := os.Getenv("NGINX_TEST_BIN")
	if binary == "" {
		t.Skip("set NGINX_TEST_BIN for real candidate validation")
	}
	service := testService(t)
	if err := os.MkdirAll(filepath.Dir(service.paths.NginxBin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(binary, service.paths.NginxBin); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(service.paths.MimeTypes), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(service.paths.MimeTypes, []byte("types { text/html html; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(service.paths.StateFile)
	if err != nil {
		t.Fatal(err)
	}
	marker := []byte("# active config must remain untouched")
	if err := os.WriteFile(service.paths.NginxMaster, marker, 0o644); err != nil {
		t.Fatal(err)
	}
	settings := service.State().Settings
	settings.WorkerConnections = 2048
	if _, err := service.TestSettings(settings); err != nil {
		t.Fatal(err)
	}
	// A variable accepted by domain validation but rejected by nginx proves the submitted form is tested.
	settings.Logging.CustomFormat = "$nonexistent_settings_test_variable"
	if _, err := service.TestSettings(settings); err == nil {
		t.Fatal("expected nginx to reject unknown variable")
	}
	after, err := os.ReadFile(service.paths.StateFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("validation changed saved state")
	}
	active, err := os.ReadFile(service.paths.NginxMaster)
	if err != nil || string(active) != string(marker) {
		t.Fatal("validation changed active config", err)
	}
	candidates, err := filepath.Glob(filepath.Join(service.paths.TmpDir, "fnproxy-candidate-*"))
	if err != nil || len(candidates) != 0 {
		t.Fatal("candidate tree was not cleaned up", candidates, err)
	}
}
