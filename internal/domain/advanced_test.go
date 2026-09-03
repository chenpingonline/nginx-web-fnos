package domain

import (
	"testing"
	"time"
)

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
