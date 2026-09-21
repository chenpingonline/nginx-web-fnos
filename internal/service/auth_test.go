package service

import "testing"

func TestAuthProfileStoresHashAndAuthenticatesAppliedSnapshot(t *testing.T) {
	service := testService(t)
	profile, err := service.CreateAuthProfile(AuthProfileInput{Name: "Family", Realm: "Private", Users: []AuthUserInput{{Username: "alice", Password: "correct horse", Enabled: true}}})
	if err != nil {
		t.Fatal(err)
	}
	persisted := service.State().AuthProfiles[0]
	if persisted.Users[0].PasswordHash == "" || persisted.Users[0].PasswordHash == "correct horse" {
		t.Fatal("password was not stored as a hash")
	}
	if profile.Users[0].PasswordHash != "" {
		t.Fatal("service response exposed the password hash")
	}
	if _, ok := service.AuthenticateBasic(profile.ID, "alice", "correct horse"); ok {
		t.Fatal("draft credentials became active before apply")
	}
	service.setAppliedAuthProfiles(service.State().AuthProfiles)
	if _, ok := service.AuthenticateBasic(profile.ID, "alice", "correct horse"); !ok {
		t.Fatal("valid applied credentials were rejected")
	}
	if _, ok := service.AuthenticateBasic(profile.ID, "alice", "wrong"); ok {
		t.Fatal("invalid password was accepted")
	}
}

func TestUpdatingAuthProfileKeepsBlankPassword(t *testing.T) {
	service := testService(t)
	created, err := service.CreateAuthProfile(AuthProfileInput{Name: "Family", Users: []AuthUserInput{{Username: "alice", Password: "correct horse", Enabled: true}}})
	if err != nil {
		t.Fatal(err)
	}
	userID := created.Users[0].ID
	if _, err := service.UpdateAuthProfile(created.ID, AuthProfileInput{Name: "Family 2", Realm: "Private", Users: []AuthUserInput{{ID: userID, Username: "alice", Enabled: true}}}); err != nil {
		t.Fatal(err)
	}
	service.setAppliedAuthProfiles(service.State().AuthProfiles)
	if _, ok := service.AuthenticateBasic(created.ID, "alice", "correct horse"); !ok {
		t.Fatal("blank update unexpectedly replaced the password")
	}
}
