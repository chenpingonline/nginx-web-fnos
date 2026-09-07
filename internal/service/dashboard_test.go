package service

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
)

func TestDashboardSeparatesDraftFromAppliedRules(t *testing.T) {
	s := testService(t)
	rule, err := s.CreateRule(domain.ProxyRule{Name: "Demo", Enabled: true, ListenPort: 19080, Domains: []string{"example.test"}, UpstreamScheme: "http", UpstreamHost: "127.0.0.1", UpstreamPort: 8080})
	if err != nil {
		t.Fatal(err)
	}
	applied := s.State()
	applied.Dirty = false
	now := time.Now().UTC()
	applied.LastAppliedAt = &now
	s.store.Update(func(state *State) error { state.LastAppliedAt = &now; return nil })
	data, _ := json.Marshal(applied)
	os.WriteFile(s.paths.AppliedState(), data, 0o600)
	d := s.Dashboard(60, "")
	if d.AppliedCount != 1 || d.Rules[0].ConfigState != "applied" {
		t.Fatalf("applied baseline incorrect: %+v", d.Rules)
	}
	if d.Rules[0].ListenAddress != "0.0.0.0:19080" || d.Rules[0].Entry != "example.test:19080" {
		t.Fatalf("HTTP listener must be independent from its domain entry: %+v", d.Rules[0])
	}
	s.store.Update(func(state *State) error { state.Rules[0].Enabled = false; state.Dirty = true; return nil })
	d = s.Dashboard(60, "")
	if d.AppliedCount != 1 || d.Rules[0].ConfigState != "pending" {
		t.Fatal("draft disabled treated as already stopped")
	}
	s.store.Update(func(state *State) error { state.Rules = nil; return nil })
	d = s.Dashboard(60, "")
	if len(d.Rules) != 1 || d.Rules[0].ID != rule.ID || d.Rules[0].ConfigState != "pending_delete" || d.Rules[0].ListenAddress != "0.0.0.0:19080" {
		t.Fatal("still forwarding deleted draft omitted")
	}
}

func TestDashboardStreamListenersPreserveAddressesForPendingDeletes(t *testing.T) {
	s := testService(t)
	if err := s.store.Update(func(state *State) error {
		state.StreamRules = []domain.StreamRule{
			{ID: "111111111111", Name: "IPv6", Protocol: "tcp", Enabled: true, ListenAddress: "2001:db8::1", ListenPort: 9443, UpstreamHost: "127.0.0.1", UpstreamPort: 8080},
			{ID: "222222222222", Name: "IPv4", Protocol: "udp", Enabled: true, ListenAddress: "127.0.0.1", ListenPort: 5353, UpstreamHost: "127.0.0.1", UpstreamPort: 8080},
			{ID: "333333333333", Name: "Wildcard", Protocol: "tcp", Enabled: true, ListenAddress: "*", ListenPort: 9000, UpstreamHost: "127.0.0.1", UpstreamPort: 8080},
		}
		now := time.Now().UTC()
		state.LastAppliedAt = &now
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(s.State())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.paths.AppliedState(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"111111111111": "[2001:db8::1]:9443", "222222222222": "127.0.0.1:5353", "333333333333": "0.0.0.0:9000"}
	for _, phase := range []string{"applied", "pending_delete"} {
		if phase == "pending_delete" {
			if err := s.store.Update(func(state *State) error { state.StreamRules = nil; return nil }); err != nil {
				t.Fatal(err)
			}
		}
		d := s.Dashboard(60, "")
		if len(d.Rules) != len(want) {
			t.Fatalf("%s stream rules missing: %+v", phase, d.Rules)
		}
		for _, rule := range d.Rules {
			if rule.ListenAddress != want[rule.ID] || rule.ConfigState != phase {
				t.Fatalf("%s listener incorrect: %+v", phase, rule)
			}
			if rule.ID == "111111111111" && rule.Entry != "2001:db8::1:9443" {
				t.Fatalf("existing entry changed: %+v", rule)
			}
			encoded, err := json.Marshal(rule)
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]any
			if err := json.Unmarshal(encoded, &payload); err != nil || payload["listen_address"] != want[rule.ID] {
				t.Fatalf("listener API field missing or incorrect: %s (%v)", encoded, err)
			}
		}
	}
}

func TestDashboardTreatsLegacyNullStreamListsAsUnchanged(t *testing.T) {
	s := testService(t)
	_, err := s.CreateStreamRule(domain.StreamRule{Name: "Legacy TCP", Enabled: true, Protocol: "tcp", ListenPort: 19081, UpstreamHost: "127.0.0.1", UpstreamPort: 8080})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := s.store.Update(func(state *State) error { state.LastAppliedAt = &now; return nil }); err != nil {
		t.Fatal(err)
	}
	applied := s.State()
	applied.StreamRules[0].TrustedProxies = nil
	applied.StreamRules[0].Allow = nil
	applied.StreamRules[0].Deny = nil
	applied.StreamRules[0].SNIRoutes = nil
	encoded, err := json.Marshal(applied)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.paths.AppliedState(), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := s.Dashboard(60, "").Rules[0].ConfigState; got != "applied" {
		t.Fatalf("empty-list normalization must not create a pending change: %s", got)
	}
	if err := s.store.Update(func(state *State) error { state.StreamRules[0].UpstreamPort = 8081; return nil }); err != nil {
		t.Fatal(err)
	}
	if got := s.Dashboard(60, "").Rules[0].ConfigState; got != "pending" {
		t.Fatalf("actual target changes must still be pending: %s", got)
	}
}

func TestCertificateAlertsIncludeAppliedAndStreamReferences(t *testing.T) {
	now := time.Now()
	state := State{Certificates: []domain.CertificateMeta{
		{ID: "used", Name: "在用", NotBefore: now.Add(-time.Hour), NotAfter: now.Add(6 * 24 * time.Hour)},
		{ID: "unused", Name: "未用", NotAfter: now.Add(-time.Hour)},
		{ID: "future", Name: "尚未生效", NotBefore: now.Add(time.Hour), NotAfter: now.Add(365 * 24 * time.Hour)},
	}}
	applied := State{Rules: []domain.ProxyRule{{ID: "rule", Name: "旧配置", Enabled: true, TLS: true, CertificateID: "used"}}}
	state.StreamRules = []domain.StreamRule{{Name: "TLS 转发", Enabled: true, TLSMode: "terminate", CertificateID: "future"}}
	alerts := certificateAlerts(state, applied, now)
	if len(alerts) != 2 || alerts[0].Name != "在用" || alerts[1].Severity != "danger" {
		t.Fatalf("bad alerts: %+v", alerts)
	}
}

func TestDashboardKeepsLastApplyFailure(t *testing.T) {
	s := testService(t)
	if _, err := s.Apply("missing binary"); err == nil {
		t.Fatal("expected missing Nginx binary failure")
	}
	if s.Overview().LastApplyError == "" {
		t.Fatal("application failure missing from dashboard")
	}
}

func TestDashboardHTTPListenFamilies(t *testing.T) {
	for kind, want := range map[string]string{"": "0.0.0.0:9560", "ipv6": "[::]:9560", "dual": "0.0.0.0:9560, [::]:9560"} {
		if got := dashboardHTTPListenAddress(kind, 9560); got != want {
			t.Fatalf("%s: %s", kind, got)
		}
	}
}

func TestDashboardURLsPreserveSchemesAndIPv6(t *testing.T) {
	r := domain.ProxyRule{TLS: true, ListenPort: 9560, Domains: []string{"music.example.com", "2001:db8::1"}, UpstreamScheme: "http", UpstreamHost: "192.168.1.88", UpstreamPort: 4533}
	urls := dashboardEntryURLs(r)
	if len(urls) != 2 || urls[0] != "https://music.example.com:9560" || urls[1] != "https://[2001:db8::1]:9560" {
		t.Fatal(urls)
	}
	if got := dashboardTargetURL(r); got != "http://192.168.1.88:4533" {
		t.Fatal(got)
	}
	r.RootLocation = domain.LocationSettings{BackendType: "proxy", UpstreamScheme: "https", UpstreamHost: "::1", UpstreamPort: 8443}
	if got := dashboardTargetURL(r); got != "https://[::1]:8443" {
		t.Fatal(got)
	}
	r.RootLocation.UpstreamPoolID = "pool"
	if got := dashboardTargetURL(r); got != "" {
		t.Fatal("pool is not a direct URL", got)
	}
}
