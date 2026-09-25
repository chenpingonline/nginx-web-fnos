package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/fileutil"
)

// ToggleRule applies only the enabled flag of an existing applied rule. Draft
// edits (including edits to this rule's destination) never enter the candidate.
func (s *AppService) ToggleRule(id string, stream, enabled bool) (ApplyResult, error) {
	if !domain.ValidID(id) {
		return ApplyResult{}, errors.New("规则 ID 不合法")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var base, candidate State
	var result ApplyResult
	var activated bool
	var wasRunning bool
	var oldSnapshot []byte
	var snapshotExisted bool
	err := s.store.Update(func(draft *State) error {
		var known bool
		base, known = s.appliedState(*draft)
		if !known || draft.LastAppliedAt == nil || base.LastAppliedAt == nil || !draft.LastAppliedAt.Equal(*base.LastAppliedAt) {
			return errors.New("尚无可确认的已应用配置，请先检查并应用配置，再使用即时启停")
		}
		candidate = domain.CloneState(base)
		setFlag := func(state *State) bool {
			if stream {
				for i := range state.StreamRules {
					if state.StreamRules[i].ID == id {
						state.StreamRules[i].Enabled = enabled
						return true
					}
				}
			} else {
				for i := range state.Rules {
					if state.Rules[i].ID == id {
						state.Rules[i].Enabled = enabled
						return true
					}
				}
			}
			return false
		}
		if !setFlag(draft) {
			return errors.New("找不到指定规则")
		}
		if !setFlag(&candidate) {
			return errors.New("该规则尚未应用，请先保存并应用规则，再使用即时启停")
		}
		domain.ApplyStateDefaults(draft)
		if err := domain.ValidateState(*draft); err != nil {
			return err
		}
		if err := domain.ValidateRuntimePorts(*draft); err != nil {
			return err
		}
		// Read the previous snapshot before modifying runtime state, for rollback.
		var err error
		oldSnapshot, err = os.ReadFile(s.paths.AppliedState())
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		snapshotExisted = err == nil
		wasRunning = s.nginx.IsRunning()
		if !enabled && !wasRunning {
			result, err = s.nginx.Prepare(candidate)
		} else {
			result, err = s.nginx.Apply(candidate)
		}
		if err != nil {
			return err
		}
		activated = true
		now := time.Now().UTC()
		candidate.LastAppliedAt = &now
		candidate.Dirty = false
		candidate.DraftRevisionID = ""
		candidate.PortMigrationPending = false
		candidate.LastApplyError = ""
		result.Message = "规则已停用并立即生效"
		if enabled {
			result.Message = "规则已启用并立即生效"
		}
		candidate.LastApplyMessage = result.Message
		data, err := json.Marshal(candidate)
		if err != nil {
			return err
		}
		if err := fileutil.WriteFileAtomic(s.paths.AppliedState(), data, 0600); err != nil {
			return err
		}
		draft.LastAppliedAt = &now
		draft.LastApplyMessage = result.Message
		draft.LastApplyError = ""
		draft.Dirty = !sameToggleConfiguration(*draft, candidate)
		if !draft.Dirty {
			draft.DraftRevisionID = ""
		}
		// Pending edits are compared independently of application timestamps.
		return nil
	})
	if err != nil {
		if activated {
			var rollbackErr error
			if wasRunning {
				_, rollbackErr = s.nginx.Apply(base)
			} else {
				if s.nginx.IsRunning() {
					_, rollbackErr = s.nginx.Stop()
				}
				if rollbackErr == nil {
					_, rollbackErr = s.nginx.Prepare(base)
				}
			}
			var snapshotErr error
			if snapshotExisted {
				snapshotErr = fileutil.WriteFileAtomic(s.paths.AppliedState(), oldSnapshot, 0600)
			} else {
				snapshotErr = os.Remove(s.paths.AppliedState())
				if os.IsNotExist(snapshotErr) {
					snapshotErr = nil
				}
			}
			if rollbackErr != nil || snapshotErr != nil {
				return ApplyResult{}, fmt.Errorf("保存启停状态失败：%v；回退运行配置：%v；回退生效记录：%v，请检查运行状态", err, rollbackErr, snapshotErr)
			}
		}
		return ApplyResult{}, err
	}
	s.setAppliedAuthProfiles(candidate.AuthProfiles)
	if err := s.saveRevision(candidate, result.Message); err != nil {
		result.Message += "；配置历史保存失败: " + err.Error()
	}
	return result, nil
}

func sameToggleConfiguration(left, right State) bool {
	clean := func(state State) State {
		state = domain.CloneState(state)
		domain.ApplyStateDefaults(&state)
		// Storage format versions are not user configuration changes.
		state.SchemaVersion = 0
		state.RuntimeConfigVersion = 0
		state.RuntimeMinListenPort = 0
		state.PortMigrationPending = false
		state.Dirty = false
		state.DraftRevisionID = ""
		state.LastAppliedAt = nil
		state.LastApplyMessage = ""
		state.LastApplyError = ""
		state.UpdatedAt = time.Time{}
		policies := make(map[string]domain.RateLimitSettings, len(state.RateLimitPolicies))
		for _, policy := range state.RateLimitPolicies {
			policies[policy.ID] = policy.Settings
		}
		for i := range state.Rules {
			state.Rules[i].UpdatedAt = time.Time{}
			// A referenced policy overrides the legacy inline parameters. Older
			// renderers wrote resolved values into applied snapshots only.
			if settings, ok := policies[state.Rules[i].RateLimitPolicyID]; ok {
				state.Rules[i].RateLimit = settings
			}
		}
		for i := range state.StreamRules {
			state.StreamRules[i].UpdatedAt = time.Time{}
		}
		normalizeEmptyConfigSlices(reflect.ValueOf(&state).Elem())
		return state
	}
	return reflect.DeepEqual(clean(left), clean(right))
}

// Older JSON snapshots use null or omit lists that newer writers emit as [].
// Normalize only empty slices; retain every nonempty value and its order.
func normalizeEmptyConfigSlices(value reflect.Value) {
	switch value.Kind() {
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if value.Field(i).CanSet() {
				normalizeEmptyConfigSlices(value.Field(i))
			}
		}
	case reflect.Slice:
		if value.Len() == 0 {
			value.SetZero()
			return
		}
		for i := 0; i < value.Len(); i++ {
			normalizeEmptyConfigSlices(value.Index(i))
		}
	case reflect.Pointer:
		if !value.IsNil() {
			normalizeEmptyConfigSlices(value.Elem())
		}
	}
}
