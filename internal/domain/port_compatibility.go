package domain

import "fmt"

// Stored configuration is portable between editions. Only enabled listeners
// require this edition's binding capability; dormant defaults/groups are kept.
func ValidateRuntimePorts(state State) error {
	ApplyStateDefaults(&state)
	edition := "标准版"
	if MinListenPort == 1 {
		edition = "全端口版"
	}
	for _, rule := range state.Rules {
		if rule.Enabled && rule.ListenPort < MinListenPort {
			return fmt.Errorf("%s仅支持 %d–65535 的监听端口", edition, MinListenPort)
		}
	}
	for _, rule := range state.StreamRules {
		if rule.Enabled && rule.ListenPort < MinListenPort {
			return fmt.Errorf("%s仅支持 %d–65535 的监听端口", edition, MinListenPort)
		}
	}
	return nil
}

// PauseUnsupportedPorts preserves all addresses and never automatically enables
// rules when returning to the full-ports edition.
func PauseUnsupportedPorts(state *State) bool {
	ApplyStateDefaults(state)
	changed := state.RuntimeMinListenPort != MinListenPort
	incompatible := false
	for i := range state.Rules {
		if state.Rules[i].ListenPort < MinListenPort {
			incompatible = true
			if state.Rules[i].Enabled {
				state.Rules[i].Enabled = false
				changed = true
			}
		}
	}
	for i := range state.StreamRules {
		if state.StreamRules[i].ListenPort < MinListenPort {
			incompatible = true
			if state.StreamRules[i].Enabled {
				state.StreamRules[i].Enabled = false
				changed = true
			}
		}
	}
	incompatible = incompatible || state.Settings.DefaultHTTPPort < MinListenPort || state.Settings.DefaultHTTPSPort < MinListenPort
	// Also covers pre-marker packages. Rebuild once before using old generated files.
	if changed && MinListenPort > 1 && (state.RuntimeMinListenPort == 1 || incompatible) {
		state.PortMigrationPending = true
		state.Dirty = true
		state.LastApplyMessage = "已暂停当前版本不支持的低端口规则，原始配置已保留"
	}
	state.RuntimeMinListenPort = MinListenPort
	return changed
}
