package domain

import (
	"strings"
	"testing"
	"time"
)

func testRule(id, name, domain string, port int) ProxyRule {
	return ProxyRule{
		ID:                    id,
		Name:                  name,
		Enabled:               true,
		ListenPort:            port,
		Domains:               []string{domain},
		UpstreamScheme:        "http",
		UpstreamHost:          "127.0.0.1",
		UpstreamPort:          8080,
		PreserveHost:          true,
		WebSocket:             true,
		Streaming:             true,
		ConnectTimeoutSeconds: 10,
		ReadTimeoutSeconds:    60,
		SendTimeoutSeconds:    60,
		CreatedAt:             time.Now().UTC(),
		UpdatedAt:             time.Now().UTC(),
	}
}

func TestValidateStateRejectsDuplicateDomain(t *testing.T) {
	state := DefaultState()
	state.Rules = []ProxyRule{
		testRule("0123456789ab", "A", "demo.example.com", 19080),
		testRule("abcdef012345", "B", "demo.example.com", 19080),
	}
	if err := ValidateState(state); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("expected duplicate-domain error, got %v", err)
	}
}

func TestValidateStateAcceptsHTTPAndHTTPSStandardPorts(t *testing.T) {
	state := DefaultState()
	state.Settings.DefaultHTTPPort = 80
	state.Settings.DefaultHTTPSPort = 443
	state.Rules = []ProxyRule{
		testRule("0123456789ab", "HTTP", "http.example.com", 80),
		testRule("abcdef012345", "HTTPS", "https.example.com", 443),
	}
	state.Rules[1].TLS = true
	state.Rules[1].CertificateID = "0123456789ab"
	state.Certificates = []CertificateMeta{{ID: "0123456789ab", Name: "test"}}
	if err := ValidateState(state); err != nil {
		t.Fatalf("expected ports 80 and 443 to be accepted, got %v", err)
	}
}

func TestValidateStateRejectsZeroListenPort(t *testing.T) {
	state := DefaultState()
	state.Rules = []ProxyRule{testRule("0123456789ab", "A", "demo.example.com", 81)}
	if err := ValidateState(state); err == nil || !strings.Contains(err.Error(), "80、443") {
		t.Fatalf("expected invalid listen-port error, got %v", err)
	}
}

func TestNormalizeRule(t *testing.T) {
	rule := ProxyRule{
		Name:           " Demo ",
		Domains:        []string{"B.EXAMPLE.COM, a.example.com", "a.example.com"},
		UpstreamHost:   "[::1]",
		UpstreamScheme: "HTTP",
	}
	NormalizeRule(&rule, DefaultState().Settings)
	if rule.Name != "Demo" || rule.UpstreamScheme != "http" || rule.UpstreamHost != "::1" {
		t.Fatalf("unexpected normalized rule: %#v", rule)
	}
	if len(rule.Domains) != 2 || rule.Domains[0] != "a.example.com" || rule.Domains[1] != "b.example.com" {
		t.Fatalf("unexpected normalized domains: %#v", rule.Domains)
	}
	if rule.ListenPort != 9080 || rule.ConnectTimeoutSeconds != 10 {
		t.Fatalf("defaults not applied: %#v", rule)
	}
}

func TestApplyStateDefaultsMigratesInlineRateLimitToPolicy(t *testing.T) {
	state := DefaultState()
	state.Rules = []ProxyRule{{
		ID: "abcdef012345", Name: "公开接口", RateLimit: RateLimitSettings{
			Enabled: true, RequestsPerSecond: 12, Burst: 24, NoDelay: true,
		},
	}}

	ApplyStateDefaults(&state)

	if len(state.RateLimitPolicies) != 1 {
		t.Fatalf("expected one migrated policy, got %d", len(state.RateLimitPolicies))
	}
	if state.Rules[0].RateLimitPolicyID != state.RateLimitPolicies[0].ID {
		t.Fatal("expected migrated rule to reference the new policy")
	}
	if state.RateLimitPolicies[0].Settings.RequestsPerSecond != 12 {
		t.Fatal("expected migrated policy to preserve rate-limit settings")
	}
}
