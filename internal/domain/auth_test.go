package domain

import (
	"testing"
	"time"
)

func TestPublicStateRedactsAuthenticationHashes(t *testing.T) {
	state := DefaultState()
	state.AuthProfiles = []AuthProfile{{ID: "0123456789ab", Name: "family", Realm: "Private", Users: []AuthUser{{ID: "abcdef012345", Username: "alice", PasswordHash: "secret-hash", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}}}}
	public := PublicState(state)
	if public.AuthProfiles[0].Users[0].PasswordHash != "" {
		t.Fatal("public state exposed a password hash")
	}
	if state.AuthProfiles[0].Users[0].PasswordHash != "secret-hash" {
		t.Fatal("redaction mutated persisted state")
	}
}

func TestValidateStateRequiresUsableAuthenticationProfile(t *testing.T) {
	state := DefaultState()
	state.AuthProfiles = []AuthProfile{{ID: "0123456789ab", Name: "family", Realm: "Private", Users: []AuthUser{{ID: "abcdef012345", Username: "alice", PasswordHash: "$2a$10$7EqJtq98hPqEX7fNZaFWoO5qB2lYqoV4Q1R4Z8F1w7v5jM8mP2h4K", Enabled: true}}}}
	rule := ProxyRule{ID: "111111111111", Name: "demo", Enabled: true, ListenPort: 19080, Domains: []string{"demo.test"}, UpstreamScheme: "http", UpstreamHost: "127.0.0.1", UpstreamPort: 8080, ConnectTimeoutSeconds: 10, ReadTimeoutSeconds: 60, SendTimeoutSeconds: 60, Authentication: RuleAuthentication{Enabled: true, Mode: AuthenticationModeBasic, ProfileID: state.AuthProfiles[0].ID}}
	NormalizeRule(&rule, state.Settings)
	state.Rules = []ProxyRule{rule}
	if err := ValidateState(state); err != nil {
		t.Fatalf("valid authentication profile rejected: %v", err)
	}
	state.AuthProfiles[0].Users[0].Enabled = false
	if err := ValidateState(state); err == nil {
		t.Fatal("rule accepted an authentication profile without enabled users")
	}
}
