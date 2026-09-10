package domain

import "testing"

func TestListenFamilyConflicts(t *testing.T) {
	s := DefaultState()
	a := testRule("0123456789ab", "A", "example.com", 19080)
	b := testRule("abcdef012345", "B", "example.com", 19080)
	b.ListenType = "ipv6"
	s.Rules = []ProxyRule{a, b}
	if err := ValidateState(s); err != nil {
		t.Fatal(err)
	}
	s.Rules[0].ListenType = "dual"
	if err := ValidateState(s); err == nil {
		t.Fatal("dual listener must conflict with IPv6 domain")
	}
	s.Rules[0].ListenType = "invalid"
	if err := ValidateState(s); err == nil {
		t.Fatal("invalid listener accepted")
	}
}
func TestListenGroupInheritance(t *testing.T) {
	g := RuleGroup{ID: "0123456789ab", ListenType: "dual"}
	r := ProxyRule{GroupID: g.ID, InheritFields: []string{"listen_type"}}
	ResolveRuleGroup(&r, []RuleGroup{g})
	if r.ListenType != "dual" {
		t.Fatal(r.ListenType)
	}
	r.InheritFields = nil
	g.ListenType = "ipv6"
	ResolveRuleGroup(&r, []RuleGroup{g})
	if r.ListenType != "dual" {
		t.Fatal("custom listener overwritten")
	}
	if got := ListenFamilies(""); len(got) != 1 || got[0] != "ipv4" {
		t.Fatal("legacy listener changed")
	}
}
