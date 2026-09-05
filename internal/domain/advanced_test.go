package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNormalizeStreamRuleSerializesEmptyListsAsArrays(t *testing.T) {
	for _, input := range []string{
		`{"name":"legacy","protocol":"tcp"}`,
		`{"name":"legacy","protocol":"udp","trusted_proxies":null,"allow":null,"deny":null,"sni_routes":null}`,
	} {
		var rule StreamRule
		if err := json.Unmarshal([]byte(input), &rule); err != nil {
			t.Fatal(err)
		}
		NormalizeStreamRule(&rule)
		encoded, err := json.Marshal(rule)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"trusted_proxies", "allow", "deny", "sni_routes"} {
			if string(fields[field]) != "[]" {
				t.Errorf("%s must be an empty JSON array, got %s", field, fields[field])
			}
		}
	}
}

func TestNormalizeStreamRulePreservesListsAndInitializesRouteNames(t *testing.T) {
	rule := StreamRule{
		TrustedProxies: []string{"127.0.0.1"}, Allow: []string{"10.0.0.0/8"}, Deny: []string{"all"},
		SNIRoutes: []SNIRoute{{ServerNames: []string{" EXAMPLE.TEST "}, UpstreamHost: "127.0.0.1", UpstreamPort: 443}, {}},
	}
	NormalizeStreamRule(&rule)
	if len(rule.TrustedProxies) != 1 || rule.TrustedProxies[0] != "127.0.0.1" || len(rule.Allow) != 1 || rule.Allow[0] != "10.0.0.0/8" || len(rule.Deny) != 1 || rule.Deny[0] != "all" {
		t.Fatal("normalization changed configured access lists")
	}
	if len(rule.SNIRoutes) != 2 || rule.SNIRoutes[0].ServerNames[0] != "example.test" || rule.SNIRoutes[0].UpstreamPort != 443 || rule.SNIRoutes[1].ServerNames == nil {
		t.Fatalf("unexpected normalized SNI routes: %+v", rule.SNIRoutes)
	}
}

func TestValidateStateAcceptsAdvancedRuntimeAndUpstreamPool(t *testing.T) {
	state := DefaultState()
	state.Settings.RealIP = RealIPSettings{Enabled: true, Header: "X-Forwarded-For", TrustedProxies: []string{"10.0.0.0/8"}, Recursive: true}
	pool := UpstreamPool{ID: "0123456789ab", Name: "web", Protocol: "http", Strategy: "least_conn", Keepalive: 32, Servers: []UpstreamServer{{Host: "127.0.0.1", Port: 8080}}}
	NormalizeUpstreamPool(&pool)
	pool.CreatedAt = time.Now().UTC()
	pool.UpdatedAt = pool.CreatedAt
	state.UpstreamPools = []UpstreamPool{pool}
	rule := ProxyRule{ID: "abcdef012345", Name: "demo", Enabled: true, ListenPort: 19080, Domains: []string{"demo.test"}, UpstreamScheme: "http", UpstreamHost: "127.0.0.1", UpstreamPort: 8080, UpstreamPoolID: pool.ID, ConnectTimeoutSeconds: 10, ReadTimeoutSeconds: 60, SendTimeoutSeconds: 60, RateLimit: RateLimitSettings{Enabled: true, RequestsPerSecond: 20, Burst: 40, Connections: 10}}
	state.Rules = []ProxyRule{rule}
	if err := ValidateState(state); err != nil {
		t.Fatal(err)
	}
}

func TestValidateStateRejectsUntrustedRealIPAndMissingPool(t *testing.T) {
	state := DefaultState()
	state.Settings.RealIP.Enabled = true
	if err := ValidateState(state); err == nil {
		t.Fatal("expected Real IP validation error")
	}
	state = DefaultState()
	rule := ProxyRule{ID: "abcdef012345", Name: "demo", Enabled: true, ListenPort: 19080, Domains: []string{"demo.test"}, UpstreamScheme: "http", UpstreamPoolID: "0123456789ab", ConnectTimeoutSeconds: 10, ReadTimeoutSeconds: 60, SendTimeoutSeconds: 60}
	state.Rules = []ProxyRule{rule}
	if err := ValidateState(state); err == nil {
		t.Fatal("expected missing pool validation error")
	}
}
