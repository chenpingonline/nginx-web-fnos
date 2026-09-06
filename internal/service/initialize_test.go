package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	nginxmanager "github.com/chenpingonline/nginx-web-fnos/internal/nginx"
)

func TestInitializePreservesExistingDraft(t *testing.T) {
	for _, kind := range []string{"settings", "rule", "previously-applied"} {
		t.Run(kind, func(t *testing.T) {
			s := testService(t)
			if err := s.store.Update(func(state *State) error {
				switch kind {
				case "settings":
					state.Settings.WorkerConnections++
				case "rule":
					state.Rules = []ProxyRule{{ID: domain.RandomID(), Name: "test", ListenPort: 19080, Domains: []string{"example.com"}, UpstreamScheme: "http", UpstreamHost: "127.0.0.1", UpstreamPort: 8080}}
				case "previously-applied":
					now := time.Now().UTC()
					state.LastAppliedAt = &now
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			original := []byte("existing active configuration")
			if err := os.WriteFile(s.paths.NginxMaster, original, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := s.Initialize(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(s.paths.NginxMaster)
			if err != nil || string(data) != string(original) || !s.State().Dirty {
				t.Fatalf("initialization changed the active config or cleared the draft: %s, %v", data, err)
			}
		})
	}
}

func TestInitializeFailureRemainsUnapplied(t *testing.T) {
	s := testService(t)
	// Pass binary/config validation, then fail the actual start command.
	if err := os.MkdirAll(filepath.Dir(s.paths.NginxBin), 0o750); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncase \" $* \" in\n*' -v '*) echo 'nginx/" + domain.NginxVersion + "'; exit 0;;\n*' -t '*) exit 0;;\nesac\necho 'simulated startup failure' >&2\nexit 1\n"
	if err := os.WriteFile(s.paths.NginxBin, []byte(script), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := s.Initialize(); err == nil {
		t.Fatal("expected activation failure")
	}
	state := s.State()
	if !state.Dirty || state.LastAppliedAt != nil || state.LastApplyError == "" {
		t.Fatalf("failed initialization reported success: %+v", state)
	}
	if _, err := os.Stat(s.paths.AppliedState()); !os.IsNotExist(err) {
		t.Fatalf("failed initialization saved an applied snapshot: %v", err)
	}
}

func TestInitializeWithRealNginx(t *testing.T) {
	bin := os.Getenv("NGINX_TEST_BIN")
	if bin == "" {
		t.Skip("set NGINX_TEST_BIN for initial activation regression tests")
	}
	for _, prepared := range []bool{false, true} {
		name := "fresh-install"
		if prepared {
			name = "existing-unapplied-defaults"
		}
		t.Run(name, func(t *testing.T) {
			s := testService(t)
			s.paths.NginxBin = bin
			s.nginx = nginxmanager.New(s.paths)
			if err := os.MkdirAll(filepath.Dir(s.paths.MimeTypes), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(s.paths.MimeTypes, []byte("types { text/plain txt; }\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { s.nginx.Stop() })
			if prepared {
				if _, err := s.Prepare(); err != nil {
					t.Fatal(err)
				}
				if _, err := s.NginxStart(); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.Initialize(); err != nil {
				t.Fatal(err)
			}
			d := s.Dashboard(60, "")
			if d.Overview.Dirty || d.Overview.LastAppliedAt == nil || !d.Overview.Nginx.Running || !d.AppliedKnown || !d.MonitoringReady {
				t.Fatalf("initial state is not synchronized: %+v", d)
			}
			reloaded, err := New(s.paths)
			if err != nil {
				t.Fatal(err)
			}
			if reloaded.State().Dirty || reloaded.State().LastAppliedAt == nil {
				t.Fatal("applied state was not persisted")
			}
			before := *s.State().LastAppliedAt
			if err := s.Initialize(); err != nil {
				t.Fatal(err)
			}
			if !before.Equal(*s.State().LastAppliedAt) {
				t.Fatal("restart applied the configuration again")
			}
			settings := s.State().Settings
			settings.WorkerConnections = domain.DefaultState().Settings.WorkerConnections + 1
			if err := s.UpdateSettings(settings); err != nil {
				t.Fatal(err)
			}
			if err := s.Initialize(); err != nil || !s.State().Dirty {
				t.Fatalf("restart must preserve an edited draft: %v", err)
			}
		})
	}
}
