package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
)

func TestCrossEditionRetainedState(t *testing.T) {
	state := domain.DefaultState()
	state.RuntimeMinListenPort = 1
	state.Settings.DefaultHTTPPort = 80
	state.Settings.DefaultHTTPSPort = 443
	state.RuleGroups = []domain.RuleGroup{{ID: "abcdef123456", Name: "low group", ListenType: "ipv4", ListenPort: 80}}
	state.Rules = []domain.ProxyRule{
		{ID: "abcdef123457", Name: "low", Enabled: true, GroupID: "abcdef123456", InheritFields: []string{"listen_port"}, ListenPort: 19080, Domains: []string{"low.test"}, UpstreamHost: "127.0.0.1", UpstreamPort: 8080, UpstreamScheme: "http"},
		{ID: "abcdef123458", Name: "high", Enabled: true, ListenPort: 19081, Domains: []string{"high.test"}, UpstreamHost: "127.0.0.1", UpstreamPort: 8080, UpstreamScheme: "http"},
	}
	state.StreamRules = []domain.StreamRule{{ID: "abcdef123459", Name: "dns", Enabled: true, Protocol: "udp", ListenPort: 53, ListenAddress: "0.0.0.0", UpstreamHost: "127.0.0.1", UpstreamPort: 5353, TLSMode: "off", ConnectTimeoutSeconds: 10, ProxyTimeoutSeconds: 60}}
	path := filepath.Join(t.TempDir(), "state.json")
	raw, _ := json.Marshal(state)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	got := store.Snapshot()
	if got.Settings.DefaultHTTPPort != 80 || got.Settings.DefaultHTTPSPort != 443 || got.Rules[0].ListenPort != 80 || !got.Rules[1].Enabled {
		t.Fatalf("lost configuration: %+v", got)
	}
	if domain.MinListenPort > 1 {
		if got.Rules[0].Enabled || got.StreamRules[0].Enabled || !got.PortMigrationPending {
			t.Fatal("unsupported rules not paused")
		}
		if err := store.Update(func(s *domain.State) error { s.Rules[0].Enabled = true; return nil }); err == nil {
			t.Fatal("enabled unsupported HTTP listener")
		}
		if err := store.Update(func(s *domain.State) error { s.StreamRules[0].Enabled = true; return nil }); err == nil {
			t.Fatal("enabled unsupported UDP listener")
		}
		if err := store.Update(func(s *domain.State) error { s.PortMigrationPending = false; return nil }); err != nil {
			t.Fatal(err)
		}
		reloaded, err := New(path)
		if err != nil {
			t.Fatal(err)
		}
		if reloaded.Snapshot().PortMigrationPending {
			t.Fatal("migration repeats after successful preparation")
		}
		if err := reloaded.Update(func(s *domain.State) error {
			s.RuleGroups[0].ListenPort = 19082
			s.Rules[0].Enabled = true
			return nil
		}); err != nil {
			t.Fatalf("cannot reactivate after correcting inherited port: %v", err)
		}
		if !reloaded.Snapshot().Rules[0].Enabled || reloaded.Snapshot().Rules[0].ListenPort != 19082 {
			t.Fatal("corrected rule not enabled")
		}
	} else if !got.Rules[0].Enabled || !got.StreamRules[0].Enabled {
		t.Fatal("full edition disabled supported rules")
	}
}
