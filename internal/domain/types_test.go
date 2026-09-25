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

func TestValidateLowListenPorts(t *testing.T) {
	for _, port := range []int{1, 53, 80, 443, 1023, 1024, 65535, -1, 65536} {
		valid := port >= 1 && port <= 65535
		state := DefaultState()
		state.Rules = []ProxyRule{testRule("0123456789ab", "A", "demo.example.com", port)}
		if err := ValidateState(state); (err == nil) != valid {
			t.Errorf("HTTP port %d: %v", port, err)
		}
		state = DefaultState()
		state.Settings.DefaultHTTPPort = port
		state.Settings.DefaultHTTPSPort = port
		if err := ValidateState(state); (err == nil) != valid {
			t.Errorf("default port %d: %v", port, err)
		}
		state = DefaultState()
		state.RuleGroups = []RuleGroup{{ID: "0123456789ab", Name: "A", ListenPort: port, ListenType: "ipv4"}}
		if err := ValidateRuleGroups(state); (err == nil) != valid {
			t.Errorf("group port %d: %v", port, err)
		}
		for _, protocol := range []string{"tcp", "udp"} {
			rule := StreamRule{ID: "0123456789ab", Name: "A", Protocol: protocol, ListenPort: port, ListenAddress: "0.0.0.0", ConnectTimeoutSeconds: 10, ProxyTimeoutSeconds: 60, TLSMode: "off", UpstreamHost: "127.0.0.1", UpstreamPort: 8080}
			if err := ValidateStreamRule(rule, nil, nil); (err == nil) != valid {
				t.Errorf("%s port %d: %v", protocol, port, err)
			}
		}
	}
	// Direct validators reject zero; state normalization intentionally supplies defaults for zero.
	rule := testRule("0123456789ab", "A", "demo.example.com", 0)
	if err := ValidateRule(rule, nil); err == nil {
		t.Error("zero listen port accepted")
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

func TestNormalizeRuleMigratesLegacyHTTPSRedirect(t *testing.T) {
	rule := testRule("0123456789ab", "redirect", "demo.example.com", 19080)
	rule.RootLocation.RedirectToHTTPS = true

	NormalizeRule(&rule, DefaultState().Settings)

	if !rule.RedirectToHTTPS || rule.RedirectHTTPSPort != 443 || rule.RootLocation.RedirectToHTTPS {
		t.Fatalf("legacy HTTPS redirect was not migrated: %#v", rule)
	}
}

func TestValidateRuleRejectsInvalidHTTPSRedirect(t *testing.T) {
	rule := testRule("0123456789ab", "redirect", "demo.example.com", 19080)
	NormalizeRule(&rule, DefaultState().Settings)
	rule.RedirectToHTTPS = true
	rule.RedirectHTTPSPort = 70000
	if err := ValidateRule(rule, nil); err == nil || !strings.Contains(err.Error(), "跳转目标端口") {
		t.Fatalf("expected redirect-port error, got %v", err)
	}

	rule.RedirectHTTPSPort = 443
	rule.TLS = true
	if err := ValidateRule(rule, nil); err == nil || !strings.Contains(err.Error(), "不能再启用") {
		t.Fatalf("expected TLS redirect conflict, got %v", err)
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

// The build mode limits local listeners, never the destination service ports.
func TestPermissionModeAllowsLowUpstreamPorts(t *testing.T) {
	for _, port := range []int{1, 80, 443, 1023} {
		rule := testRule("0123456789ab", "upstream", "demo.example.com", 19080)
		rule.UpstreamPort = port
		NormalizeRule(&rule, DefaultState().Settings)
		if err := ValidateRule(rule, nil); err != nil {
			t.Errorf("upstream port %d rejected: %v", port, err)
		}
	}
}
