package domain

import (
	"errors"
	"fmt"
	"strings"
)

// RuleGroup provides live defaults; resolved values stay on rules for rendering
// and for preserving access settings when a rule leaves its group.
type RuleGroup struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ListenPort    int    `json:"listen_port"`
	TLS           bool   `json:"tls"`
	HTTP2         bool   `json:"http2"`
	CertificateID string `json:"certificate_id"`
}

func ResolveRuleGroup(rule *ProxyRule, groups []RuleGroup) {
	for _, group := range groups {
		if group.ID != rule.GroupID {
			continue
		}
		for _, field := range rule.InheritFields {
			switch field {
			case "listen_port":
				rule.ListenPort = group.ListenPort
			case "tls":
				rule.TLS = group.TLS
			case "certificate_id":
				rule.CertificateID = group.CertificateID
			case "http2":
				rule.HTTP2 = group.HTTP2
			}
		}
		if !rule.TLS {
			rule.CertificateID = ""
		}
		return
	}
}

func ValidateRuleGroups(state State) error {
	groups := map[string]bool{}
	names := map[string]bool{}
	certs := map[string]bool{}
	for _, cert := range state.Certificates {
		certs[cert.ID] = true
	}
	for _, group := range state.RuleGroups {
		name := strings.TrimSpace(group.Name)
		if !ValidID(group.ID) || groups[group.ID] {
			return errors.New("分组 ID 无效或重复")
		}
		if name == "" || len([]rune(name)) > 80 || names[strings.ToLower(name)] {
			return errors.New("分组名称不能为空、超过 80 字或重复")
		}
		if group.ListenPort < 1024 || group.ListenPort > 65535 {
			return errors.New("分组监听端口必须为 1024–65535")
		}
		if group.TLS && !certs[group.CertificateID] {
			return fmt.Errorf("分组 %q 需要选择有效证书", name)
		}
		if !group.TLS && group.CertificateID != "" {
			return errors.New("HTTP 分组不能配置证书")
		}
		groups[group.ID] = true
		names[strings.ToLower(name)] = true
	}
	for _, rule := range state.Rules {
		if rule.GroupID != "" && !groups[rule.GroupID] {
			return fmt.Errorf("规则 %q 的分组不存在", rule.Name)
		}
		if rule.GroupID == "" && len(rule.InheritFields) > 0 {
			return errors.New("未分组规则不能继承分组设置")
		}
		fields := map[string]bool{}
		for _, field := range rule.InheritFields {
			if fields[field] {
				return errors.New("继承字段重复")
			}
			fields[field] = true
			switch field {
			case "listen_port", "tls", "certificate_id", "http2":
			default:
				return errors.New("不支持的分组继承字段")
			}
		}
	}
	return nil
}
