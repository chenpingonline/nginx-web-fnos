package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/fileutil"
)

const runtimeConfigVersion = 1

// Read the previous edition's applied state before checking its ports. A low
// listener is a migration input, not evidence that the snapshot is invalid.
func (s *AppService) migrationBaseline(draft State) (State, bool, error) {
	matches := func(candidate State) bool {
		return candidate.LastAppliedAt != nil && (draft.LastAppliedAt == nil || candidate.LastAppliedAt.Equal(*draft.LastAppliedAt))
	}
	for _, path := range []string{s.paths.AppliedState(), s.paths.StateFile + ".migration-baseline"} {
		raw, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return State{}, false, err
		}
		var candidate State
		if err := json.Unmarshal(raw, &candidate); err != nil {
			return State{}, false, fmt.Errorf("读取迁移基准失败: %w", err)
		}
		if matches(candidate) {
			return candidate, true, nil
		}
	}
	revisions, err := s.ListRevisions()
	if err != nil {
		return State{}, false, err
	}
	for _, revision := range revisions {
		if matches(revision.State) {
			return revision.State, true, nil
		}
	}
	if !draft.Dirty && draft.LastAppliedAt != nil {
		return draft, true, nil
	}
	return State{}, false, nil
}

// Used by both the installation callback and startup. Only the applied baseline
// is regenerated; the user's saved draft is compared and retained separately.
func (s *AppService) migrateRuntimeConfiguration() (ApplyResult, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft := s.State()
	if draft.RuntimeConfigVersion >= runtimeConfigVersion && !draft.PortMigrationPending {
		// Repair representation-only dirty flags left by earlier migrations,
		// including installations that already have the completion marker.
		// This does not activate a draft or regenerate the running configuration.
		if draft.Dirty && draft.LastApplyError == "" {
			if applied, known := s.appliedState(draft); known && sameToggleConfiguration(draft, applied) {
				err := s.store.Update(func(current *State) error {
					if sameToggleConfiguration(*current, applied) {
						current.Dirty = false
						current.DraftRevisionID = ""
					}
					return nil
				})
				if err != nil {
					return ApplyResult{}, true, fmt.Errorf("修正配置同步状态失败: %w", err)
				}
			}
		}
		return ApplyResult{}, false, nil
	}
	base, known, err := s.migrationBaseline(draft)
	if err != nil {
		return ApplyResult{}, true, err
	}
	if !known {
		if draft.PortMigrationPending {
			return ApplyResult{}, true, errors.New("无法确认版本切换前的已应用配置，已保留原始数据，未自动应用草稿")
		}
		return ApplyResult{}, false, nil
	}
	candidate := domain.CloneState(base)
	domain.PauseUnsupportedPorts(&candidate)
	candidate.RuntimeMinListenPort = domain.MinListenPort
	candidate.RuntimeConfigVersion = runtimeConfigVersion
	candidate.PortMigrationPending = false
	candidate.Dirty = false
	candidate.DraftRevisionID = ""
	candidate.LastApplyError = ""
	candidate.LastApplyMessage = "已自动迁移已应用配置"
	// Retain the activation identity so an interrupted metadata write is retryable.
	candidate.LastAppliedAt = base.LastAppliedAt
	var result ApplyResult
	if s.nginx.IsRunning() {
		result, err = s.nginx.Apply(candidate)
	} else {
		result, err = s.nginx.Prepare(candidate)
	}
	if err != nil {
		return result, true, fmt.Errorf("自动迁移配置失败: %w", err)
	}
	raw, err := json.Marshal(candidate)
	if err == nil {
		err = fileutil.WriteFileAtomic(s.paths.AppliedState(), raw, 0600)
	}
	if err != nil {
		return result, true, fmt.Errorf("迁移配置已生成，保存已应用快照失败，可重试: %w", err)
	}
	revisions, err := s.ListRevisions()
	if err != nil {
		return result, true, err
	}
	recorded := false
	for _, revision := range revisions {
		if revision.State.RuntimeConfigVersion == runtimeConfigVersion && revision.State.RuntimeMinListenPort == domain.MinListenPort && revision.State.LastAppliedAt != nil && revision.State.LastAppliedAt.Equal(*candidate.LastAppliedAt) && sameToggleConfiguration(revision.State, candidate) {
			recorded = true
			break
		}
	}
	if !recorded {
		if err := s.saveRevision(candidate, candidate.LastApplyMessage); err != nil {
			return result, true, fmt.Errorf("保存迁移历史失败，可重试: %w", err)
		}
	}
	err = s.store.Update(func(current *State) error {
		current.RuntimeConfigVersion = runtimeConfigVersion
		current.PortMigrationPending = false
		current.LastAppliedAt = candidate.LastAppliedAt
		current.LastApplyMessage = candidate.LastApplyMessage
		current.LastApplyError = ""
		current.Dirty = !sameToggleConfiguration(*current, candidate)
		if !current.Dirty {
			current.DraftRevisionID = ""
		}
		return nil
	})
	if err != nil {
		return result, true, fmt.Errorf("保存迁移状态失败，可重试: %w", err)
	}
	s.setAppliedAuthProfiles(candidate.AuthProfiles)
	result.Message = candidate.LastApplyMessage
	return result, true, nil
}
