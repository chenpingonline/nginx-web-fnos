package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	nginxmanager "github.com/chenpingonline/nginx-web-fnos/internal/nginx"
)

func TestToggleRequiresAppliedBaseline(t *testing.T) {
	s := testService(t)
	if _, err := s.ToggleRule("abcdef123456", false, false); err == nil {
		t.Fatal("accepted unknown applied configuration")
	}
}

func TestImmediateTogglePreservesDraftAndRollback(t *testing.T) {
	bin := os.Getenv("NGINX_TEST_BIN")
	if bin == "" {
		t.Skip("requires real nginx")
	}
	for _, stream := range []bool{false, true} {
		name := "http"
		if stream {
			name = "tcp"
		}
		t.Run(name, func(t *testing.T) {
			s := testService(t)
			s.paths.NginxBin = bin
			s.nginx = nginxmanager.New(s.paths)
			defer s.nginx.Stop()
			os.MkdirAll(filepath.Dir(s.paths.MimeTypes), 0750)
			os.WriteFile(s.paths.MimeTypes, []byte("types { text/html html; }"), 0640)
			id := "abcdef123456"
			if err := s.store.Update(func(state *State) error {
				state.Settings.DefaultHTTPPort = 19989
				if stream {
					state.StreamRules = []domain.StreamRule{{ID: id, Name: "tcp", Enabled: true, Protocol: "tcp", ListenPort: 19990, ListenAddress: "127.0.0.1", UpstreamHost: "127.0.0.1", UpstreamPort: 19991, TLSMode: "off", ConnectTimeoutSeconds: 10, ProxyTimeoutSeconds: 60}}
				} else {
					state.Rules = []domain.ProxyRule{{ID: id, Name: "http", Enabled: true, ListenPort: 19990, Domains: []string{"toggle.test"}, UpstreamScheme: "http", UpstreamHost: "127.0.0.1", UpstreamPort: 19991}}
				}
				state.Dirty = true
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Apply("baseline"); err != nil {
				t.Fatal(err)
			}
			// Disabling while the whole Nginx service is stopped must not start it.
			if _, err := s.NginxStop(); err != nil {
				t.Fatal(err)
			}
			if _, err := s.ToggleRule(id, stream, false); err != nil {
				t.Fatal(err)
			}
			if s.nginx.IsRunning() {
				t.Fatal("disable unexpectedly started nginx")
			}
			if _, err := s.ToggleRule(id, stream, true); err != nil {
				t.Fatal(err)
			}
			// A clean toggle must not leave pending changes.
			if _, err := s.ToggleRule(id, stream, false); err != nil {
				t.Fatal(err)
			}
			if s.State().Dirty {
				t.Fatal("clean toggle left pending changes")
			}
			if _, err := s.ToggleRule(id, stream, true); err != nil {
				t.Fatal(err)
			}
			if err := s.store.Update(func(state *State) error {
				if stream {
					state.StreamRules[0].UpstreamPort = 19992
				} else {
					state.Rules[0].UpstreamPort = 19992
				}
				state.Settings.WorkerConnections = 4321
				state.Dirty = true
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			for _, enabled := range []bool{false, true} {
				if _, err := s.ToggleRule(id, stream, enabled); err != nil {
					t.Fatal(err)
				}
				current := s.State()
				applied, known := s.appliedState(current)
				if !known || !current.Dirty || applied.Settings.WorkerConnections == 4321 {
					t.Fatal("draft settings applied or lost")
				}
				if stream {
					if applied.StreamRules[0].Enabled != enabled || applied.StreamRules[0].UpstreamPort != 19991 || current.StreamRules[0].UpstreamPort != 19992 {
						t.Fatal("TCP state mixed")
					}
				} else {
					if applied.Rules[0].Enabled != enabled || applied.Rules[0].UpstreamPort != 19991 || current.Rules[0].UpstreamPort != 19992 {
						t.Fatal("HTTP state mixed")
					}
				}
				files, err := os.ReadDir(s.paths.NginxConfD)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, file := range files {
					raw, _ := os.ReadFile(filepath.Join(s.paths.NginxConfD, file.Name()))
					text := string(raw)
					if strings.Contains(text, "19992") {
						t.Fatal("draft destination leaked")
					}
					found = found || strings.Contains(text, "19991")
				}
				if found != enabled {
					t.Fatal("disabled rule remained in nginx config, or enabled rule missing")
				}
			}
			// Make draft persistence fail after runtime activation; both runtime and
			// the applied snapshot must return to their previous state.
			original, _ := os.ReadFile(s.paths.StateFile)
			os.Remove(s.paths.StateFile)
			os.Mkdir(s.paths.StateFile, 0750)
			if _, err := s.ToggleRule(id, stream, false); err == nil {
				t.Fatal("accepted failed persistence")
			}
			os.Remove(s.paths.StateFile)
			os.WriteFile(s.paths.StateFile, original, 0600)
			applied, known := s.appliedState(s.State())
			if !known {
				t.Fatal("lost applied snapshot during rollback")
			}
			if stream {
				if !applied.StreamRules[0].Enabled || !s.State().StreamRules[0].Enabled {
					t.Fatal("TCP rollback failed")
				}
			} else if !applied.Rules[0].Enabled || !s.State().Rules[0].Enabled {
				t.Fatal("HTTP rollback failed")
			}
		})
	}
}
