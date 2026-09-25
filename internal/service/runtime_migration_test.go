package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	nginxmanager "github.com/chenpingonline/nginx-web-fnos/internal/nginx"
)

func migrationNginx(t *testing.T, s *AppService) {
	t.Helper()
	if bin := os.Getenv("NGINX_TEST_BIN"); bin != "" {
		s.paths.NginxBin = bin
		s.nginx = nginxmanager.New(s.paths)
	} else {
		os.MkdirAll(filepath.Dir(s.paths.NginxBin), 0750)
		if err := os.WriteFile(s.paths.NginxBin, []byte("#!/bin/sh\ncase \" $* \" in *' -v '*) echo 'nginx/"+domain.NginxVersion+"';; esac\nexit 0\n"), 0750); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(filepath.Dir(s.paths.MimeTypes), 0750)
	os.WriteFile(s.paths.MimeTypes, []byte("types { text/html html; }"), 0640)
}

func migrationState() State {
	state := domain.DefaultState()
	now := time.Now().UTC().Add(-time.Hour)
	state.LastAppliedAt = &now
	state.Dirty = false
	state.RuntimeMinListenPort = 1
	state.Settings.DefaultHTTPPort = 19989
	state.Rules = []domain.ProxyRule{
		{ID: "abcdef123456", Name: "low", Enabled: true, ListenPort: 80, Domains: []string{"low.test"}, UpstreamScheme: "http", UpstreamHost: "127.0.0.1", UpstreamPort: 19991},
		{ID: "abcdef123457", Name: "high", Enabled: true, ListenPort: 19990, Domains: []string{"high.test"}, UpstreamScheme: "http", UpstreamHost: "127.0.0.1", UpstreamPort: 19991},
	}
	state.StreamRules = []domain.StreamRule{{ID: "abcdef123458", Name: "dns", Enabled: true, Protocol: "udp", ListenPort: 53, ListenAddress: "127.0.0.1", UpstreamHost: "127.0.0.1", UpstreamPort: 5353, TLSMode: "off", ConnectTimeoutSeconds: 10, ProxyTimeoutSeconds: 60}}
	return state
}

func writeMigrationJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticRuntimeMigration(t *testing.T) {
	for _, source := range []string{"snapshot", "history", "clean-state", "legacy-cleared-time", "current-schema"} {
		if source == "current-schema" && domain.MinListenPort == 1 {
			continue
		}
		for _, dirty := range []bool{false, true} {
			if source == "clean-state" && dirty {
				continue
			}
			t.Run(source+map[bool]string{false: "-clean", true: "-draft"}[dirty], func(t *testing.T) {
				s := testService(t)
				base := migrationState()
				if source == "current-schema" {
					base.RuntimeConfigVersion = runtimeConfigVersion
				}
				draft := domain.CloneState(base)
				if dirty {
					draft.Dirty = true
					draft.Rules[1].UpstreamPort = 19992
					draft.Settings.WorkerConnections = 4321
				}
				switch source {
				case "snapshot", "legacy-cleared-time", "current-schema":
					writeMigrationJSON(t, s.paths.AppliedState(), base)
				case "history":
					if err := s.saveRevision(base, "old applied"); err != nil {
						t.Fatal(err)
					}
				}
				if source == "legacy-cleared-time" {
					domain.PauseUnsupportedPorts(&draft)
					draft.LastAppliedAt = nil
					draft.Dirty = true
					draft.PortMigrationPending = false // Previous releases already cleared this in Prepare.
				}
				writeMigrationJSON(t, s.paths.StateFile, draft)
				s, err := New(s.paths)
				if err != nil {
					t.Fatal(err)
				}
				migrationNginx(t, s)
				if _, err := s.Prepare(); err != nil {
					t.Fatal(err)
				}
				current := s.State()
				applied, known := s.appliedState(current)
				if !known || current.LastAppliedAt == nil || current.PortMigrationPending || current.RuntimeConfigVersion != runtimeConfigVersion || current.Dirty != dirty {
					t.Fatalf("migration state mismatch: known=%v state=%+v", known, current)
				}
				if applied.StreamRules[0].Enabled != (domain.MinListenPort == 1) || applied.Rules[0].Enabled != (domain.MinListenPort == 1) || !applied.Rules[1].Enabled || applied.Rules[1].UpstreamPort != 19991 {
					t.Fatalf("invalid applied rules: %+v", applied.Rules)
				}
				if dirty && (current.Rules[1].UpstreamPort != 19992 || current.Settings.WorkerConnections != 4321 || applied.Settings.WorkerConnections == 4321) {
					t.Fatal("draft lost or activated")
				}
				revs, _ := s.ListRevisions()
				// Installation callback can be repeated; serving must not create another migration.
				if _, err := s.Prepare(); err != nil {
					t.Fatal(err)
				}
				if err := s.Initialize(); err != nil {
					t.Fatal(err)
				}
				after, _ := s.ListRevisions()
				if len(after) != len(revs) {
					t.Fatal("migration repeated")
				}
				files, err := os.ReadDir(s.paths.NginxConfD)
				if err != nil {
					t.Fatal(err)
				}
				var config string
				for _, file := range files {
					raw, _ := os.ReadFile(filepath.Join(s.paths.NginxConfD, file.Name()))
					config += string(raw)
				}
				if strings.Contains(config, "19992") || (domain.MinListenPort > 1 && strings.Contains(config, "low.test")) || !strings.Contains(config, "high.test") {
					t.Fatal("runtime configuration contains a draft or disabled rule")
				}
				if os.Getenv("NGINX_TEST_BIN") != "" {
					t.Cleanup(func() { s.nginx.Stop() })
					if _, err := s.NginxStart(); err != nil {
						t.Fatal(err)
					}
					if _, err := s.ToggleRule("abcdef123457", false, false); err != nil {
						t.Fatal("immediate toggle after migration:", err)
					}
					if s.State().Dirty != dirty {
						t.Fatal("toggle changed draft status after migration")
					}
				}

			})
		}
	}
}

func TestRuntimeMigrationFailureAndRetry(t *testing.T) {
	s := testService(t)
	base := migrationState()
	writeMigrationJSON(t, s.paths.AppliedState(), base)
	writeMigrationJSON(t, s.paths.StateFile, base)
	s, err := New(s.paths)
	if err != nil {
		t.Fatal(err)
	}
	migrationNginx(t, s)
	originalBin := s.paths.NginxBin
	failBin := filepath.Join(t.TempDir(), "fail-nginx")
	os.WriteFile(failBin, []byte("#!/bin/sh\nexit 1\n"), 0750)
	s.paths.NginxBin = failBin
	s.nginx = nginxmanager.New(s.paths)
	if _, err := s.Prepare(); err == nil {
		t.Fatal("expected failure")
	}
	if s.State().RuntimeConfigVersion != 0 {
		t.Fatal("failed migration marked complete")
	}
	raw, _ := os.ReadFile(s.paths.AppliedState())
	var unchanged State
	json.Unmarshal(raw, &unchanged)
	if !unchanged.Rules[0].Enabled {
		t.Fatal("failed validation changed snapshot")
	}
	s.paths.NginxBin = originalBin
	s.nginx = nginxmanager.New(s.paths)

	// Fail state persistence after the runtime and snapshot have been generated.
	os.Remove(s.paths.StateFile)
	os.Mkdir(s.paths.StateFile, 0700)
	if _, err := s.Prepare(); err == nil {
		t.Fatal("expected state persistence failure")
	}
	if s.State().RuntimeConfigVersion != 0 {
		t.Fatal("failed commit marked complete")
	}
	revs, _ := s.ListRevisions()
	os.Remove(s.paths.StateFile)
	if _, err := s.Prepare(); err != nil {
		t.Fatal(err)
	}
	after, _ := s.ListRevisions()
	if len(after) != len(revs) {
		t.Fatal("retry duplicated migration history")
	}

	if s.State().Dirty || s.State().LastAppliedAt == nil {
		t.Fatal("retry not synchronized")
	}
}

func TestPrepareNeverActivatesUnknownDraft(t *testing.T) {
	s := testService(t)
	if err := s.store.Update(func(state *State) error { state.Settings.WorkerConnections = 4321; state.Dirty = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Prepare(); err == nil {
		t.Fatal("unknown edited settings were prepared")
	}
	if !s.State().Dirty || s.State().LastAppliedAt != nil {
		t.Fatal("draft changed")
	}
}
