package service

import (
	"errors"
	"strings"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
)

func (s *AppService) SaveRuleGroup(id string, input domain.RuleGroup) (domain.RuleGroup, error) {
	create := id == ""
	if create {
		id = domain.RandomID()
	}
	input.ID = id
	input.Name = strings.TrimSpace(input.Name)
	if !input.TLS {
		input.CertificateID = ""
	}
	err := s.store.Update(func(state *State) error {
		found := false
		for i := range state.RuleGroups {
			if state.RuleGroups[i].ID == id {
				state.RuleGroups[i] = input
				found = true
				break
			}
		}
		if !found {
			if !create {
				return errors.New("分组不存在")
			}
			state.RuleGroups = append(state.RuleGroups, input)
		}
		for i := range state.Rules {
			rule := &state.Rules[i]
			if rule.GroupID == id && len(rule.InheritFields) > 0 {
				domain.ResolveRuleGroup(rule, state.RuleGroups)
				rule.UpdatedAt = time.Now().UTC()
			}
		}
		state.Dirty = true
		return nil
	})
	return input, err
}

func (s *AppService) DeleteRuleGroup(id string) error {
	return s.store.Update(func(state *State) error {
		for i, group := range state.RuleGroups {
			if group.ID != id {
				continue
			}
			for j := range state.Rules {
				rule := &state.Rules[j]
				if rule.GroupID == id {
					domain.ResolveRuleGroup(rule, state.RuleGroups)
					rule.GroupID = ""
					rule.InheritFields = nil
					rule.UpdatedAt = time.Now().UTC()
				}
			}
			state.RuleGroups = append(state.RuleGroups[:i], state.RuleGroups[i+1:]...)
			state.Dirty = true
			return nil
		}
		return errors.New("分组不存在")
	})
}
