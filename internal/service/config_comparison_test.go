package service

import (
	"fmt"
	"testing"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
)

func TestConfigurationIgnoresLegacyRepresentation(t *testing.T) {
	for _, kind := range []string{"schema", "empty-groups", "empty-routing"} {
		t.Run(kind, func(t *testing.T) {
			old := migrationState()
			current := domain.CloneState(old)
			switch kind {
			case "schema":
				old.SchemaVersion = 4
			case "empty-groups":
				current.RuleGroups = []domain.RuleGroup{}
			case "empty-routing":
				old.Settings.Routing.Geos = nil
			}
			if !sameToggleConfiguration(old, current) {
				t.Fatal("representation-only difference treated as a draft")
			}
			current.Settings.WorkerConnections++
			if sameToggleConfiguration(old, current) {
				t.Fatal("real settings change ignored")
			}
		})
	}
}

func TestLegacyRepresentationMigrationAndRepair(t *testing.T) {
	for _, migrated := range []bool{false, true} {
		for _, dirty := range []bool{false, true} {
			name := fmt.Sprintf("migrated=%v/draft=%v", migrated, dirty)
			t.Run(name, func(t *testing.T) {
				s := testService(t)
				base := migrationState()
				domain.PauseUnsupportedPorts(&base)
				base.PortMigrationPending = false
				base.Dirty = false
				base.SchemaVersion = 4
				base.Settings.Routing.Geos = nil
				if migrated {
					base.RuntimeConfigVersion = runtimeConfigVersion
				}
				writeMigrationJSON(t, s.paths.AppliedState(), base)
				draft := domain.CloneState(base)
				draft.SchemaVersion = domain.SchemaVersion
				draft.RuleGroups = []domain.RuleGroup{}
				draft.Settings.Routing.Geos = []domain.GeoDefinition{}
				draft.Dirty = true
				if dirty {
					draft.Settings.WorkerConnections = 4321
				}
				writeMigrationJSON(t, s.paths.StateFile, draft)
				s, err := New(s.paths)
				if err != nil {
					t.Fatal(err)
				}
				migrationNginx(t, s)
				if _, err := s.Prepare(); err != nil {
					t.Fatal(err)
				}
				if err := s.Initialize(); err != nil {
					t.Fatal(err)
				}
				if s.State().Dirty != dirty {
					t.Fatalf("dirty=%v want %v", s.State().Dirty, dirty)
				}
				applied, known := s.appliedState(s.State())
				if !known || applied.Settings.WorkerConnections == 4321 {
					t.Fatal("draft activated or applied baseline lost")
				}
				before, _ := s.ListRevisions()
				if migrated && len(before) != 0 {
					t.Fatal("status repair created a revision")
				}
				s, err = New(s.paths)
				if err != nil {
					t.Fatal(err)
				}
				if err := s.Initialize(); err != nil {
					t.Fatal(err)
				}
				if s.State().Dirty != dirty {
					t.Fatal("dirty repair did not persist")
				}
				after, _ := s.ListRevisions()
				if len(before) != len(after) {
					t.Fatal("repeated migration")
				}
			})
		}
	}
}

func TestRatePolicyMigrationDoesNotCreateDraft(t *testing.T) {
	s := testService(t)
	base := migrationState()
	base.RateLimitPolicies = []domain.RateLimitPolicy{{ID: "abcdef123459", Name: "limit", Settings: domain.RateLimitSettings{Enabled: true, RequestsPerSecond: 10, Burst: 20}}}
	base.Rules[0].RateLimitPolicyID = "abcdef123459"
	writeMigrationJSON(t, s.paths.AppliedState(), base)
	writeMigrationJSON(t, s.paths.StateFile, base)
	s, err := New(s.paths)
	if err != nil {
		t.Fatal(err)
	}
	migrationNginx(t, s)
	if _, err := s.Prepare(); err != nil {
		t.Fatal(err)
	}
	if s.State().Dirty {
		t.Fatal("rendering rate policy created a draft without any user edit")
	}
}

func TestRepairResolvedRatePolicySnapshot(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			s := testService(t)
			draft := migrationState()
			domain.PauseUnsupportedPorts(&draft)
			draft.PortMigrationPending = false
			draft.RuntimeConfigVersion = runtimeConfigVersion
			draft.Dirty = true
			draft.RateLimitPolicies = []domain.RateLimitPolicy{{ID: "abcdef123459", Name: "limit", Settings: domain.RateLimitSettings{Enabled: true, RequestsPerSecond: 10, Burst: 20}}}
			draft.Rules[0].RateLimitPolicyID = "abcdef123459"
			applied := domain.CloneState(draft)
			applied.Dirty = false
			applied.Rules[0].RateLimit = applied.RateLimitPolicies[0].Settings
			if changed {
				draft.RateLimitPolicies[0].Settings.Burst = 30
			}
			writeMigrationJSON(t, s.paths.StateFile, draft)
			writeMigrationJSON(t, s.paths.AppliedState(), applied)
			s, err := New(s.paths)
			if err != nil {
				t.Fatal(err)
			}
			migrationNginx(t, s)
			if _, err := s.Prepare(); err != nil {
				t.Fatal(err)
			}
			if s.State().Dirty != changed {
				t.Fatalf("dirty=%v want %v", s.State().Dirty, changed)
			}
			baseline, known := s.appliedState(s.State())
			if !known || baseline.RateLimitPolicies[0].Settings.Burst != 20 {
				t.Fatal("real policy draft activated")
			}
			revs, _ := s.ListRevisions()
			if len(revs) != 0 {
				t.Fatal("repair should not create history")
			}
		})
	}
}
