package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

const AuthenticationModeBasic = "basic"

type RuleAuthentication struct {
	Enabled              bool   `json:"enabled"`
	Mode                 string `json:"mode,omitempty"`
	ProfileID            string `json:"profile_id,omitempty"`
	ForwardAuthorization bool   `json:"forward_authorization"`
}

type AuthProfile struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Realm     string     `json:"realm"`
	Users     []AuthUser `json:"users"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type AuthUser struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash,omitempty"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func NormalizeAuthProfile(profile *AuthProfile) {
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Realm = strings.TrimSpace(profile.Realm)
	if profile.Realm == "" {
		profile.Realm = "Restricted"
	}
	if profile.Users == nil {
		profile.Users = []AuthUser{}
	}
	for index := range profile.Users {
		profile.Users[index].Username = strings.TrimSpace(profile.Users[index].Username)
	}
}

func ValidateAuthProfile(profile AuthProfile) error {
	if !ValidID(profile.ID) {
		return errors.New("认证策略 ID 格式不正确")
	}
	if profile.Name == "" || len([]rune(profile.Name)) > 60 {
		return errors.New("认证策略名称长度必须为 1 到 60 个字符")
	}
	if profile.Realm == "" || len([]rune(profile.Realm)) > 100 || containsControl(profile.Realm) {
		return errors.New("认证提示名称长度必须为 1 到 100 个字符，且不能包含控制字符")
	}
	userIDs := make(map[string]struct{}, len(profile.Users))
	usernames := make(map[string]struct{}, len(profile.Users))
	for _, user := range profile.Users {
		if !ValidID(user.ID) {
			return errors.New("认证用户 ID 格式不正确")
		}
		if _, exists := userIDs[user.ID]; exists {
			return errors.New("存在重复的认证用户 ID")
		}
		userIDs[user.ID] = struct{}{}
		if err := ValidateAuthUsername(user.Username); err != nil {
			return err
		}
		key := strings.ToLower(user.Username)
		if _, exists := usernames[key]; exists {
			return fmt.Errorf("认证用户名 %q 重复", user.Username)
		}
		usernames[key] = struct{}{}
		if user.PasswordHash == "" {
			return fmt.Errorf("认证用户 %q 缺少密码", user.Username)
		}
	}
	return nil
}

func ValidateAuthUsername(username string) error {
	username = strings.TrimSpace(username)
	if username == "" || len([]rune(username)) > 64 {
		return errors.New("认证用户名长度必须为 1 到 64 个字符")
	}
	for _, char := range username {
		if unicode.IsControl(char) || unicode.IsSpace(char) || char == ':' {
			return errors.New("认证用户名不能包含空白、冒号或控制字符")
		}
	}
	return nil
}

func containsControl(value string) bool {
	for _, char := range value {
		if unicode.IsControl(char) {
			return true
		}
	}
	return false
}

// PublicState removes password hashes from API responses while retaining them
// in private persistence, applied snapshots, backups, and revisions.
func PublicState(state State) State {
	result := CloneState(state)
	for profileIndex := range result.AuthProfiles {
		for userIndex := range result.AuthProfiles[profileIndex].Users {
			result.AuthProfiles[profileIndex].Users[userIndex].PasswordHash = ""
		}
	}
	return result
}

func PublicRevision(revision Revision) Revision {
	revision.State = PublicState(revision.State)
	return revision
}
