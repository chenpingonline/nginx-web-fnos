package service

import (
	"errors"
	"os"
	"reflect"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
)

// Initialize runs before the management API opens, so the serving process owns
// both the initial activation and its in-memory/persisted application state.
func (s *AppService) Initialize() error {
	state := s.State()
	defaults := domain.DefaultState()
	domain.ApplyStateDefaults(&defaults)
	domain.ApplyStateDefaults(&state)
	if state.LastAppliedAt == nil && len(state.Rules) == 0 &&
		len(state.StreamRules) == 0 && len(state.Certificates) == 0 &&
		len(state.UpstreamPools) == 0 && len(state.RateLimitPolicies) == 0 &&
		reflect.DeepEqual(state.Settings, defaults.Settings) {
		// Also repairs installations that already prepared the default config
		// without recording activation. Never auto-apply an edited draft.
		_, err := s.Apply("首次初始化默认配置")
		return err
	}
	if _, err := os.Stat(s.paths.NginxMaster); errors.Is(err, os.ErrNotExist) {
		_, err = s.Prepare()
		return err
	} else {
		return err
	}
}
