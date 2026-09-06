package service

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/fileutil"
)

const MaxBackupBytes = 64 * 1024 * 1024

type BackupCertificate struct {
	ID          string `json:"id"`
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"private_key"`
}
type Backup struct {
	Format       string              `json:"format"`
	Version      int                 `json:"version"`
	AppVersion   string              `json:"app_version"`
	CreatedAt    time.Time           `json:"created_at"`
	State        State               `json:"state"`
	Certificates []BackupCertificate `json:"certificate_files"`
}

func (s *AppService) ExportBackup() (Backup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	backup := Backup{Format: "nginx-web-backup", Version: 1, AppVersion: domain.AppVersion, CreatedAt: time.Now().UTC(), State: s.store.Snapshot(), Certificates: []BackupCertificate{}}
	for _, meta := range backup.State.Certificates {
		if !domain.ValidID(meta.ID) {
			return Backup{}, errors.New("证书 ID 不合法")
		}
		dir := filepath.Join(s.paths.CertificateDir, meta.ID)
		cert, err := readCertificateFile(filepath.Join(dir, "fullchain.pem"), 2*1024*1024)
		if err != nil {
			return Backup{}, fmt.Errorf("无法备份证书 %s: %w", meta.Name, err)
		}
		key, err := readCertificateFile(filepath.Join(dir, "privkey.pem"), 512*1024)
		if err != nil {
			return Backup{}, fmt.Errorf("无法备份证书 %s 的私钥: %w", meta.Name, err)
		}
		backup.Certificates = append(backup.Certificates, BackupCertificate{ID: meta.ID, Certificate: cert, PrivateKey: key})
	}
	data, err := json.Marshal(backup)
	if err != nil {
		return Backup{}, err
	}
	if len(data) > MaxBackupBytes {
		return Backup{}, errors.New("备份超过 64 MB 上限")
	}
	return backup, nil
}

// RestoreBackup stages new certificate IDs so active configuration and existing ACME tasks
// keep their original files. Only the saved draft is replaced; activation stays explicit.
func (s *AppService) RestoreBackup(backup Backup) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if backup.Format != "nginx-web-backup" || backup.Version != 1 || backup.State.SchemaVersion != domain.SchemaVersion {
		return State{}, errors.New("不支持的备份格式或配置版本")
	}
	next := domain.CloneState(backup.State)
	domain.ApplyStateDefaults(&next)
	if err := domain.ValidateState(next); err != nil {
		return State{}, err
	}
	if len(backup.Certificates) != len(next.Certificates) {
		return State{}, errors.New("备份证书文件不完整")
	}
	material := map[string]BackupCertificate{}
	for _, item := range backup.Certificates {
		if !domain.ValidID(item.ID) {
			return State{}, errors.New("备份证书 ID 不合法")
		}
		if _, exists := material[item.ID]; exists {
			return State{}, errors.New("备份含重复证书")
		}
		if len(item.Certificate) > 2*1024*1024 || len(item.PrivateKey) > 512*1024 {
			return State{}, errors.New("备份证书文件过大")
		}
		if _, err := tls.X509KeyPair([]byte(item.Certificate), []byte(item.PrivateKey)); err != nil {
			return State{}, errors.New("备份证书与私钥不匹配")
		}
		material[item.ID] = item
	}
	current := s.store.Snapshot()
	remap := map[string]string{}
	added := []CertificateMeta{}
	created := []string{}
	committed := false
	defer func() {
		if !committed {
			for _, dir := range created {
				_ = os.RemoveAll(dir)
			}
		}
	}()
	for _, meta := range next.Certificates {
		item, ok := material[meta.ID]
		if !ok {
			return State{}, errors.New("备份缺少证书文件")
		}
		// Reuse only byte-identical material, preserving renewal links on same-device restores.
		for _, old := range current.Certificates {
			cert, _ := os.ReadFile(filepath.Join(s.paths.CertificateDir, old.ID, "fullchain.pem"))
			key, _ := os.ReadFile(filepath.Join(s.paths.CertificateDir, old.ID, "privkey.pem"))
			if string(cert) == item.Certificate && string(key) == item.PrivateKey {
				remap[meta.ID] = old.ID
				break
			}
		}
		if remap[meta.ID] != "" {
			continue
		}
		pair, _ := tls.X509KeyPair([]byte(item.Certificate), []byte(item.PrivateKey))
		leaf, err := x509.ParseCertificate(pair.Certificate[0])
		if err != nil {
			return State{}, errors.New("无法读取备份证书")
		}
		id := domain.RandomID()
		dir := filepath.Join(s.paths.CertificateDir, id)
		if err := os.Mkdir(dir, 0700); err != nil {
			return State{}, err
		}
		created = append(created, dir)
		if err := fileutil.WriteFileAtomic(filepath.Join(dir, "fullchain.pem"), []byte(item.Certificate), 0600); err != nil {
			return State{}, err
		}
		if err := fileutil.WriteFileAtomic(filepath.Join(dir, "privkey.pem"), []byte(item.PrivateKey), 0600); err != nil {
			return State{}, err
		}
		remap[meta.ID] = id
		meta.ID = id
		digest := sha256.Sum256(leaf.Raw)
		encoded := strings.ToUpper(hex.EncodeToString(digest[:]))
		parts := []string{}
		for i := 0; i < len(encoded); i += 2 {
			parts = append(parts, encoded[i:i+2])
		}
		meta.Fingerprint = strings.Join(parts, ":")
		meta.Subject = leaf.Subject.String()
		meta.DNSNames = leaf.DNSNames
		meta.NotBefore = leaf.NotBefore
		meta.NotAfter = leaf.NotAfter
		meta.SerialNumber = leaf.SerialNumber.String()
		meta.IPAddresses = nil
		for _, ip := range leaf.IPAddresses {
			meta.IPAddresses = append(meta.IPAddresses, ip.String())
		}
		added = append(added, meta)
	}
	for i := range next.Rules {
		if id := next.Rules[i].CertificateID; id != "" {
			next.Rules[i].CertificateID = remap[id]
		}
	}
	for i := range next.RuleGroups {
		if id := next.RuleGroups[i].CertificateID; id != "" {
			next.RuleGroups[i].CertificateID = remap[id]
		}
	}
	for i := range next.StreamRules {
		if id := next.StreamRules[i].CertificateID; id != "" {
			next.StreamRules[i].CertificateID = remap[id]
		}
	}
	if err := s.saveRevision(current, "导入备份前的配置"); err != nil {
		return State{}, fmt.Errorf("保存恢复前配置失败: %w", err)
	}
	err := s.store.Update(func(state *State) error {
		state.Settings = next.Settings
		state.RuleGroups = next.RuleGroups
		state.Rules = next.Rules
		state.StreamRules = next.StreamRules
		state.UpstreamPools = next.UpstreamPools
		state.RateLimitPolicies = next.RateLimitPolicies
		state.Certificates = append(state.Certificates, added...)
		state.Dirty = true
		state.DraftRevisionID = ""
		state.LastApplyError = ""
		state.LastApplyMessage = "备份已恢复为草稿，尚未应用"
		return nil
	})
	if err != nil {
		return State{}, err
	}
	committed = true
	return s.store.Snapshot(), nil
}
