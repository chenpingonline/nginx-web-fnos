package service

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

const dummyPasswordHash = "$2a$10$7EqJtq98hPqEX7fNZaFWoO5qB2lYqoV4Q1R4Z8F1w7v5jM8mP2h4K"

type AuthUserInput struct {
	ID       string `json:"id,omitempty"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Enabled  bool   `json:"enabled"`
}

type AuthProfileInput struct {
	Name  string          `json:"name"`
	Realm string          `json:"realm"`
	Users []AuthUserInput `json:"users"`
}

func (s *AppService) initializeAppliedAuth() {
	state := s.store.Snapshot()
	if data, err := os.ReadFile(s.paths.AppliedState()); err == nil {
		var applied State
		if json.Unmarshal(data, &applied) == nil {
			state = applied
		}
	} else if state.Dirty {
		state.AuthProfiles = nil
	}
	s.setAppliedAuthProfiles(state.AuthProfiles)
}

func (s *AppService) setAppliedAuthProfiles(profiles []domain.AuthProfile) {
	clone := domain.CloneState(domain.State{AuthProfiles: profiles}).AuthProfiles
	s.authMu.Lock()
	s.appliedAuthProfiles = clone
	s.authMu.Unlock()
}

func (s *AppService) CreateAuthProfile(input AuthProfileInput) (domain.AuthProfile, error) {
	profile, err := buildAuthProfile(domain.AuthProfile{}, input)
	if err != nil {
		return domain.AuthProfile{}, err
	}
	now := time.Now().UTC()
	profile.ID = domain.RandomID()
	profile.CreatedAt = now
	profile.UpdatedAt = now
	for index := range profile.Users {
		profile.Users[index].ID = domain.RandomID()
		profile.Users[index].CreatedAt = now
		profile.Users[index].UpdatedAt = now
	}
	if err := s.store.Update(func(state *State) error {
		state.AuthProfiles = append(state.AuthProfiles, profile)
		if err := domain.ValidateState(*state); err != nil {
			return err
		}
		state.Dirty = true
		return nil
	}); err != nil {
		return domain.AuthProfile{}, err
	}
	return publicAuthProfile(profile), nil
}

func (s *AppService) UpdateAuthProfile(id string, input AuthProfileInput) (domain.AuthProfile, error) {
	if !domain.ValidID(id) {
		return domain.AuthProfile{}, errors.New("认证策略 ID 不合法")
	}
	var result domain.AuthProfile
	err := s.store.Update(func(state *State) error {
		for index := range state.AuthProfiles {
			if state.AuthProfiles[index].ID != id {
				continue
			}
			updated, err := buildAuthProfile(state.AuthProfiles[index], input)
			if err != nil {
				return err
			}
			updated.ID = id
			updated.CreatedAt = state.AuthProfiles[index].CreatedAt
			updated.UpdatedAt = time.Now().UTC()
			state.AuthProfiles[index] = updated
			if err := domain.ValidateState(*state); err != nil {
				return err
			}
			state.Dirty = true
			result = updated
			return nil
		}
		return errors.New("找不到指定认证策略")
	})
	return publicAuthProfile(result), err
}

func (s *AppService) DeleteAuthProfile(id string) error {
	if !domain.ValidID(id) {
		return errors.New("认证策略 ID 不合法")
	}
	return s.store.Update(func(state *State) error {
		for _, rule := range state.Rules {
			if rule.Authentication.Enabled && rule.Authentication.ProfileID == id {
				return errors.New("认证策略仍被规则使用，请先关闭或切换规则认证")
			}
		}
		for index := range state.AuthProfiles {
			if state.AuthProfiles[index].ID == id {
				state.AuthProfiles = append(state.AuthProfiles[:index], state.AuthProfiles[index+1:]...)
				state.Dirty = true
				return nil
			}
		}
		return errors.New("找不到指定认证策略")
	})
}

func buildAuthProfile(existing domain.AuthProfile, input AuthProfileInput) (domain.AuthProfile, error) {
	profile := domain.AuthProfile{Name: input.Name, Realm: input.Realm, Users: make([]domain.AuthUser, 0, len(input.Users))}
	existingUsers := make(map[string]domain.AuthUser, len(existing.Users))
	for _, user := range existing.Users {
		existingUsers[user.ID] = user
	}
	now := time.Now().UTC()
	for _, userInput := range input.Users {
		username := strings.TrimSpace(userInput.Username)
		if err := domain.ValidateAuthUsername(username); err != nil {
			return domain.AuthProfile{}, err
		}
		if len([]byte(userInput.Password)) > 72 {
			return domain.AuthProfile{}, errors.New("认证密码不能超过 72 字节")
		}
		user, exists := existingUsers[userInput.ID]
		if userInput.ID != "" && !exists {
			return domain.AuthProfile{}, errors.New("认证用户不存在")
		}
		if !exists {
			if len(userInput.Password) < 8 {
				return domain.AuthProfile{}, errors.New("新认证用户的密码至少需要 8 个字符")
			}
			user = domain.AuthUser{ID: domain.RandomID(), CreatedAt: now}
		}
		user.Username = username
		user.Enabled = userInput.Enabled
		user.UpdatedAt = now
		if userInput.Password != "" {
			hash, err := bcrypt.GenerateFromPassword([]byte(userInput.Password), bcrypt.DefaultCost)
			if err != nil {
				return domain.AuthProfile{}, errors.New("生成认证密码失败")
			}
			user.PasswordHash = string(hash)
		}
		profile.Users = append(profile.Users, user)
	}
	domain.NormalizeAuthProfile(&profile)
	return profile, nil
}

func publicAuthProfile(profile domain.AuthProfile) domain.AuthProfile {
	return domain.PublicState(domain.State{AuthProfiles: []domain.AuthProfile{profile}}).AuthProfiles[0]
}

// AuthenticateBasic validates against the last applied snapshot. Draft user or
// password changes therefore take effect only after the same Apply action as
// the generated Nginx configuration.
func (s *AppService) AuthenticateBasic(profileID, username, password string) (string, bool) {
	s.authMu.RLock()
	profiles := s.appliedAuthProfiles
	var selected domain.AuthProfile
	foundProfile := false
	for _, profile := range profiles {
		if subtle.ConstantTimeCompare([]byte(profile.ID), []byte(profileID)) == 1 {
			selected = profile
			foundProfile = true
			break
		}
	}
	s.authMu.RUnlock()
	realm := "Restricted"
	hash := dummyPasswordHash
	foundUser := false
	if foundProfile {
		realm = selected.Realm
		for _, user := range selected.Users {
			if user.Enabled && subtle.ConstantTimeCompare([]byte(user.Username), []byte(username)) == 1 {
				hash = user.PasswordHash
				foundUser = true
				break
			}
		}
	}
	valid := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	return realm, foundProfile && foundUser && valid
}
