package service

import (
	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/store"
	"testing"
)

func groupedRule(name, host, group string, inherit bool) domain.ProxyRule {
	rule := domain.ProxyRule{Name: name, Enabled: true, Domains: []string{host}, ListenPort: 19080, UpstreamHost: "127.0.0.1", UpstreamPort: 8080, GroupID: group}
	if inherit {
		rule.InheritFields = []string{"listen_port", "tls", "certificate_id", "http2"}
	}
	return rule
}

func TestRuleGroupInheritanceAndDetach(t *testing.T) {
	s := testService(t)
	group, err := s.SaveRuleGroup("", domain.RuleGroup{Name: "家庭服务", ListenPort: 19100, HTTP2: true})
	if err != nil {
		t.Fatal(err)
	}
	inherited, err := s.CreateRule(groupedRule("继承", "one.example.com", group.ID, true))
	if err != nil {
		t.Fatal(err)
	}
	custom, err := s.CreateRule(groupedRule("自定义", "two.example.com", group.ID, false))
	if err != nil {
		t.Fatal(err)
	}
	if inherited.ListenPort != 19100 {
		t.Fatal("create did not resolve defaults", inherited)
	}
	group.ListenPort = 19200
	if _, err = s.SaveRuleGroup(group.ID, group); err != nil {
		t.Fatal(err)
	}
	state := s.State()
	if state.Rules[0].ListenPort != 19200 || state.Rules[1].ListenPort != custom.ListenPort {
		t.Fatal("inheritance overwrote custom settings", state.Rules)
	}
	reopened, err := store.New(s.paths.StateFile)
	if err != nil || reopened.Snapshot().Rules[0].GroupID != group.ID {
		t.Fatal("group did not persist", err)
	}
	if err = s.DeleteRuleGroup(group.ID); err != nil {
		t.Fatal(err)
	}
	state = s.State()
	if len(state.RuleGroups) != 0 || state.Rules[0].GroupID != "" || len(state.Rules[0].InheritFields) != 0 || state.Rules[0].ListenPort != 19200 {
		t.Fatal("detach lost effective settings", state)
	}
}

func TestRuleGroupConflictIsAtomic(t *testing.T) {
	s := testService(t)
	group, err := s.SaveRuleGroup("", domain.RuleGroup{Name: "测试", ListenPort: 19100})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateRule(groupedRule("继承", "same.example.com", group.ID, true)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateRule(groupedRule("独立", "same.example.com", "", false)); err != nil {
		t.Fatal(err)
	}
	group.ListenPort = 19080
	if _, err = s.SaveRuleGroup(group.ID, group); err == nil {
		t.Fatal("accepted conflicting group update")
	}
	state := s.State()
	if state.RuleGroups[0].ListenPort != 19100 || state.Rules[0].ListenPort != 19100 {
		t.Fatal("failed update partially persisted")
	}
	rule := groupedRule("无效", "invalid.example.com", "missing", true)
	if _, err = s.CreateRule(rule); err == nil {
		t.Fatal("accepted missing group")
	}
	rule.GroupID = group.ID
	rule.InheritFields = []string{"unknown"}
	if _, err = s.CreateRule(rule); err == nil {
		t.Fatal("accepted unknown inheritance field")
	}
}

func TestRuleGroupMoveKeepsCustomValues(t *testing.T) {
	s := testService(t)
	group, err := s.SaveRuleGroup("", domain.RuleGroup{Name: "新组", ListenPort: 19100})
	if err != nil {
		t.Fatal(err)
	}
	rule, err := s.CreateRule(groupedRule("已有规则", "existing.example.com", "", false))
	if err != nil {
		t.Fatal(err)
	}
	rule.GroupID = group.ID
	if _, err = s.UpdateRule(rule.ID, rule); err != nil {
		t.Fatal(err)
	}
	if s.State().Rules[0].ListenPort != 19080 {
		t.Fatal("moving changed access port")
	}
	rule.InheritFields = []string{"listen_port"}
	if _, err = s.UpdateRule(rule.ID, rule); err != nil {
		t.Fatal(err)
	}
	if s.State().Rules[0].ListenPort != 19100 {
		t.Fatal("opt in did not inherit")
	}
}

func TestRuleGroupTLSAndCertificateReferences(t *testing.T) {
	s := testService(t)
	certID := domain.RandomID()
	if _, err := s.deployACME(certID, "group cert", acmeResource(t, 81)); err != nil {
		t.Fatal(err)
	}
	group, err := s.SaveRuleGroup("", domain.RuleGroup{Name: "HTTPS", ListenPort: 19443, TLS: true, HTTP2: true, CertificateID: certID})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCertificate(certID); err == nil {
		t.Fatal("deleted group certificate")
	}
	rule, err := s.CreateRule(groupedRule("安全入口", "example.com", group.ID, true))
	if err != nil {
		t.Fatal(err)
	}
	if !rule.TLS || !rule.HTTP2 || rule.CertificateID != certID {
		t.Fatal("TLS defaults not resolved", rule)
	}
	group.TLS = false
	group.ListenPort = 19080
	if _, err = s.SaveRuleGroup(group.ID, group); err != nil {
		t.Fatal(err)
	}
	updated := s.State().Rules[0]
	if updated.TLS || updated.CertificateID != "" || updated.ListenPort != 19080 {
		t.Fatal("HTTP transition retained certificate", updated)
	}
}
