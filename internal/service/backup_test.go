package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
)

func TestBackupRoundTripPreservesActiveFilesAndRemapsCertificates(t *testing.T) {
	source := testService(t)
	id := domain.RandomID()
	original := acmeResource(t, 71)
	if _, err := source.deployACME(id, "backup certificate", original); err != nil {
		t.Fatal(err)
	}
	group, err := source.SaveRuleGroup("", domain.RuleGroup{Name: "secure group", TLS: true, ListenPort: 19443, CertificateID: id, HTTP2: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.CreateRule(ProxyRule{GroupID: group.ID, InheritFields: []string{"certificate_id", "http2", "tls", "listen_port"}, Name: "secure", Enabled: true, ListenPort: 19443, Domains: []string{"example.com"}, TLS: true, CertificateID: id, UpstreamScheme: "http", UpstreamHost: "127.0.0.1", UpstreamPort: 8080}); err != nil {
		t.Fatal(err)
	}
	backup, err := source.ExportBackup()
	if err != nil {
		t.Fatal(err)
	}
	if len(backup.Certificates) != 1 {
		t.Fatal("certificate missing")
	}

	// Restoring to a different device adds a certificate rather than replacing active material.
	target := testService(t)
	changed := acmeResource(t, 72)
	if _, err := target.deployACME(id, "active", changed); err != nil {
		t.Fatal(err)
	}
	active := []byte("# untouched active config")
	if err := os.WriteFile(target.paths.NginxMaster, active, 0600); err != nil {
		t.Fatal(err)
	}
	restored, err := target.RestoreBackup(backup)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Dirty || len(restored.Rules) != 1 || restored.Rules[0].CertificateID == id {
		t.Fatal("expected remapped draft", restored)
	}
	if len(restored.RuleGroups) != 1 || restored.RuleGroups[0].CertificateID != restored.Rules[0].CertificateID || restored.Rules[0].GroupID != group.ID || len(restored.Rules[0].InheritFields) != 4 {
		t.Fatal("group inheritance or certificate remapping lost", restored.RuleGroups)
	}
	old, _ := os.ReadFile(filepath.Join(target.paths.CertificateDir, id, "fullchain.pem"))
	if string(old) != string(changed.Certificate) {
		t.Fatal("active certificate overwritten")
	}
	installed, _ := os.ReadFile(filepath.Join(target.paths.CertificateDir, restored.Rules[0].CertificateID, "fullchain.pem"))
	if string(installed) != string(original.Certificate) {
		t.Fatal("restored certificate differs")
	}
	info, _ := os.Stat(filepath.Join(target.paths.CertificateDir, restored.Rules[0].CertificateID, "privkey.pem"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("private key permissions")
	}
	master, _ := os.ReadFile(target.paths.NginxMaster)
	if string(master) != string(active) {
		t.Fatal("active config changed")
	}
	revisions, err := target.ListRevisions()
	if err != nil || len(revisions) != 1 {
		t.Fatal("missing recovery checkpoint", err)
	}
	again, err := target.RestoreBackup(backup)
	if err != nil || len(again.Certificates) != len(restored.Certificates) {
		t.Fatal("identical restore duplicated certificates", err)
	}
}

func TestInvalidBackupLeavesStateUntouched(t *testing.T) {
	s := testService(t)
	backup, err := s.ExportBackup()
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(s.State())
	for _, mutate := range []func(*Backup){
		func(b *Backup) { b.Version = 99 },
		func(b *Backup) { b.State.SchemaVersion = 999 },
		func(b *Backup) { b.State.Settings.DefaultHTTPPort = 65536 },
		func(b *Backup) { b.Certificates = []BackupCertificate{{ID: "../../escape"}} },
	} {
		data, _ := json.Marshal(backup)
		var candidate Backup
		_ = json.Unmarshal(data, &candidate)
		mutate(&candidate)
		if _, err := s.RestoreBackup(candidate); err == nil {
			t.Fatal("invalid backup accepted")
		}
		after, _ := json.Marshal(s.State())
		if string(before) != string(after) {
			t.Fatal("failed restore mutated state")
		}
	}
}

func TestBackupCertificateValidationAndFailedCommitCleanup(t *testing.T) {
	source := testService(t)
	id := domain.RandomID()
	resource := acmeResource(t, 81)
	if _, err := source.deployACME(id, "source", resource); err != nil {
		t.Fatal(err)
	}
	backup, err := source.ExportBackup()
	if err != nil {
		t.Fatal(err)
	}
	target := testService(t)
	before, _ := json.Marshal(target.State())
	invalid := backup
	invalid.Certificates = append([]BackupCertificate(nil), backup.Certificates...)
	invalid.Certificates[0].PrivateKey = "invalid key"
	if _, err := target.RestoreBackup(invalid); err == nil {
		t.Fatal("mismatched key accepted")
	}
	// Make the atomic state rename fail after certificate staging.
	if err := os.Remove(target.paths.StateFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target.paths.StateFile, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := target.RestoreBackup(backup); err == nil {
		t.Fatal("expected state commit failure")
	}
	after, _ := json.Marshal(target.State())
	if string(before) != string(after) {
		t.Fatal("failed commit changed state")
	}
	files, err := os.ReadDir(target.paths.CertificateDir)
	if err != nil || len(files) != 0 {
		t.Fatal("failed restore left staged certificates", files, err)
	}
}
