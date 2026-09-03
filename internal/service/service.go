package service

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chenpingonline/fn-nginx-web/internal/domain"
	"github.com/chenpingonline/fn-nginx-web/internal/fileutil"
	nginxmanager "github.com/chenpingonline/fn-nginx-web/internal/nginx"
	"github.com/chenpingonline/fn-nginx-web/internal/platform"
	storepkg "github.com/chenpingonline/fn-nginx-web/internal/store"
)

type Paths = platform.Paths
type Store = storepkg.Store
type NginxManager = nginxmanager.Manager
type State = domain.State
type Settings = domain.Settings
type ProxyRule = domain.ProxyRule
type CertificateMeta = domain.CertificateMeta
type UpstreamPool = domain.UpstreamPool
type StreamRule = domain.StreamRule
type Revision = domain.Revision
type NginxStatus = domain.NginxStatus
type ApplyResult = domain.ApplyResult

type AppService struct {
	paths Paths
	store *Store
	nginx *NginxManager
	mu    sync.Mutex
}

type Overview struct {
	AppName          string      `json:"app_name"`
	AppVersion       string      `json:"app_version"`
	NginxVersion     string      `json:"nginx_version"`
	Nginx            NginxStatus `json:"nginx"`
	RuleCount        int         `json:"rule_count"`
	EnabledCount     int         `json:"enabled_count"`
	CertificateCount int         `json:"certificate_count"`
	Dirty            bool        `json:"dirty"`
	LastAppliedAt    *time.Time  `json:"last_applied_at,omitempty"`
	LastApplyMessage string      `json:"last_apply_message,omitempty"`
	Settings         Settings    `json:"settings"`
}

type CertificateInput struct {
	Name        string `json:"name"`
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"private_key"`
}

type ConfigSnapshot struct {
	Master string            `json:"master"`
	Files  map[string]string `json:"files"`
}

func New(paths Paths) (*AppService, error) {
	store, err := storepkg.New(paths.StateFile)
	if err != nil {
		return nil, fmt.Errorf("加载应用数据失败: %w", err)
	}
	return &AppService{
		paths: paths,
		store: store,
		nginx: nginxmanager.New(paths),
	}, nil
}

func (s *AppService) Prepare() (ApplyResult, error) {
	if err := s.nginx.CheckBinary(); err != nil {
		return ApplyResult{}, err
	}
	return s.nginx.Prepare(s.store.Snapshot())
}

func (s *AppService) Overview() Overview {
	state := s.store.Snapshot()
	return Overview{
		AppName:          domain.AppName,
		AppVersion:       domain.AppVersion,
		NginxVersion:     domain.NginxVersion,
		Nginx:            s.nginx.Status(state),
		RuleCount:        len(state.Rules) + len(state.StreamRules),
		EnabledCount:     domain.EnabledRuleCount(state),
		CertificateCount: len(state.Certificates),
		Dirty:            state.Dirty,
		LastAppliedAt:    state.LastAppliedAt,
		LastApplyMessage: state.LastApplyMessage,
		Settings:         state.Settings,
	}
}

func (s *AppService) State() State {
	return s.store.Snapshot()
}

func (s *AppService) UpdateSettings(settings Settings) error {
	return s.store.Update(func(state *State) error {
		candidate := State{Settings: settings}
		domain.ApplyStateDefaults(&candidate)
		state.Settings = candidate.Settings
		state.Dirty = true
		return nil
	})
}

func (s *AppService) CreateUpstreamPool(input UpstreamPool) (UpstreamPool, error) {
	now := time.Now().UTC()
	input.ID = domain.RandomID()
	input.CreatedAt = now
	input.UpdatedAt = now
	domain.NormalizeUpstreamPool(&input)
	err := s.store.Update(func(state *State) error {
		state.UpstreamPools = append(state.UpstreamPools, input)
		state.Dirty = true
		return nil
	})
	return input, err
}

func (s *AppService) UpdateUpstreamPool(id string, input UpstreamPool) (UpstreamPool, error) {
	if !domain.ValidID(id) {
		return UpstreamPool{}, errors.New("上游池 ID 不合法")
	}
	var updated UpstreamPool
	err := s.store.Update(func(state *State) error {
		for index := range state.UpstreamPools {
			if state.UpstreamPools[index].ID != id {
				continue
			}
			input.ID = id
			input.CreatedAt = state.UpstreamPools[index].CreatedAt
			input.UpdatedAt = time.Now().UTC()
			domain.NormalizeUpstreamPool(&input)
			state.UpstreamPools[index] = input
			state.Dirty = true
			updated = input
			return nil
		}
		return errors.New("找不到指定上游池")
	})
	return updated, err
}

func (s *AppService) DeleteUpstreamPool(id string) error {
	if !domain.ValidID(id) {
		return errors.New("上游池 ID 不合法")
	}
	return s.store.Update(func(state *State) error {
		for _, rule := range state.Rules {
			if rule.UpstreamPoolID == id {
				return fmt.Errorf("上游池仍被规则 %q 使用", rule.Name)
			}
		}
		for _, rule := range state.StreamRules {
			if rule.UpstreamPoolID == id {
				return fmt.Errorf("上游池仍被 Stream 规则 %q 使用", rule.Name)
			}
			for _, route := range rule.SNIRoutes {
				if route.UpstreamPoolID == id {
					return fmt.Errorf("上游池仍被 Stream SNI 规则 %q 使用", rule.Name)
				}
			}
		}
		for index := range state.UpstreamPools {
			if state.UpstreamPools[index].ID == id {
				state.UpstreamPools = append(state.UpstreamPools[:index], state.UpstreamPools[index+1:]...)
				state.Dirty = true
				return nil
			}
		}
		return errors.New("找不到指定上游池")
	})
}

func (s *AppService) CreateStreamRule(input StreamRule) (StreamRule, error) {
	now := time.Now().UTC()
	input.ID = domain.RandomID()
	input.CreatedAt = now
	input.UpdatedAt = now
	domain.NormalizeStreamRule(&input)
	err := s.store.Update(func(state *State) error {
		state.StreamRules = append(state.StreamRules, input)
		state.Dirty = true
		return nil
	})
	return input, err
}

func (s *AppService) UpdateStreamRule(id string, input StreamRule) (StreamRule, error) {
	if !domain.ValidID(id) {
		return StreamRule{}, errors.New("Stream 规则 ID 不合法")
	}
	var updated StreamRule
	err := s.store.Update(func(state *State) error {
		for index := range state.StreamRules {
			if state.StreamRules[index].ID != id {
				continue
			}
			input.ID = id
			input.CreatedAt = state.StreamRules[index].CreatedAt
			input.UpdatedAt = time.Now().UTC()
			domain.NormalizeStreamRule(&input)
			state.StreamRules[index] = input
			state.Dirty = true
			updated = input
			return nil
		}
		return errors.New("找不到指定 Stream 规则")
	})
	return updated, err
}

func (s *AppService) DeleteStreamRule(id string) error {
	if !domain.ValidID(id) {
		return errors.New("Stream 规则 ID 不合法")
	}
	return s.store.Update(func(state *State) error {
		for index := range state.StreamRules {
			if state.StreamRules[index].ID == id {
				state.StreamRules = append(state.StreamRules[:index], state.StreamRules[index+1:]...)
				state.Dirty = true
				return nil
			}
		}
		return errors.New("找不到指定 Stream 规则")
	})
}

func (s *AppService) CheckNginxBinary() error {
	return s.nginx.CheckBinary()
}

func (s *AppService) CreateRule(input ProxyRule) (ProxyRule, error) {
	now := time.Now().UTC()
	input.ID = domain.RandomID()
	input.CreatedAt = now
	input.UpdatedAt = now
	domain.NormalizeRule(&input, s.store.Snapshot().Settings)
	if err := s.store.Update(func(state *State) error {
		state.Rules = append(state.Rules, input)
		state.Dirty = true
		return nil
	}); err != nil {
		return ProxyRule{}, err
	}
	return input, nil
}

func (s *AppService) UpdateRule(id string, input ProxyRule) (ProxyRule, error) {
	if !domain.ValidID(id) {
		return ProxyRule{}, errors.New("规则 ID 不合法")
	}
	var updated ProxyRule
	err := s.store.Update(func(state *State) error {
		for index := range state.Rules {
			if state.Rules[index].ID != id {
				continue
			}
			input.ID = id
			input.CreatedAt = state.Rules[index].CreatedAt
			input.UpdatedAt = time.Now().UTC()
			domain.NormalizeRule(&input, state.Settings)
			state.Rules[index] = input
			state.Dirty = true
			updated = input
			return nil
		}
		return errors.New("找不到指定规则")
	})
	return updated, err
}

func (s *AppService) DeleteRule(id string) error {
	if !domain.ValidID(id) {
		return errors.New("规则 ID 不合法")
	}
	return s.store.Update(func(state *State) error {
		for index := range state.Rules {
			if state.Rules[index].ID != id {
				continue
			}
			state.Rules = append(state.Rules[:index], state.Rules[index+1:]...)
			state.Dirty = true
			return nil
		}
		return errors.New("找不到指定规则")
	})
}

func (s *AppService) ImportCertificate(input CertificateInput) (CertificateMeta, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Certificate = strings.TrimSpace(input.Certificate)
	input.PrivateKey = strings.TrimSpace(input.PrivateKey)
	if input.Certificate == "" || input.PrivateKey == "" {
		return CertificateMeta{}, errors.New("证书链和私钥不能为空")
	}
	if len(input.Certificate) > 2*1024*1024 || len(input.PrivateKey) > 512*1024 {
		return CertificateMeta{}, errors.New("证书文件过大")
	}
	if _, err := tls.X509KeyPair([]byte(input.Certificate), []byte(input.PrivateKey)); err != nil {
		return CertificateMeta{}, fmt.Errorf("证书与私钥不匹配: %w", err)
	}
	block, _ := pem.Decode([]byte(input.Certificate))
	if block == nil || block.Type != "CERTIFICATE" {
		return CertificateMeta{}, errors.New("未找到有效的 PEM 证书")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return CertificateMeta{}, fmt.Errorf("解析证书失败: %w", err)
	}
	fingerprintBytes := sha256.Sum256(cert.Raw)
	fingerprintHex := strings.ToUpper(hex.EncodeToString(fingerprintBytes[:]))
	fingerprintParts := make([]string, 0, len(fingerprintHex)/2)
	for index := 0; index < len(fingerprintHex); index += 2 {
		fingerprintParts = append(fingerprintParts, fingerprintHex[index:index+2])
	}
	fingerprint := strings.Join(fingerprintParts, ":")

	state := s.store.Snapshot()
	for _, existing := range state.Certificates {
		if existing.Fingerprint == fingerprint {
			return CertificateMeta{}, fmt.Errorf("该证书已经存在: %s", existing.Name)
		}
	}
	if input.Name == "" {
		input.Name = cert.Subject.CommonName
		if input.Name == "" && len(cert.DNSNames) > 0 {
			input.Name = cert.DNSNames[0]
		}
		if input.Name == "" {
			input.Name = "手动证书"
		}
	}
	if len([]rune(input.Name)) > 80 {
		return CertificateMeta{}, errors.New("证书名称不能超过 80 个字符")
	}

	meta := CertificateMeta{
		ID:           domain.RandomID(),
		Name:         input.Name,
		Subject:      cert.Subject.String(),
		DNSNames:     append([]string(nil), cert.DNSNames...),
		SerialNumber: cert.SerialNumber.String(),
		NotBefore:    cert.NotBefore.UTC(),
		NotAfter:     cert.NotAfter.UTC(),
		Fingerprint:  fingerprint,
		CreatedAt:    time.Now().UTC(),
	}
	for _, ip := range cert.IPAddresses {
		meta.IPAddresses = append(meta.IPAddresses, ip.String())
	}

	finalDir := filepath.Join(s.paths.CertificateDir, meta.ID)
	tempDir, err := os.MkdirTemp(s.paths.CertificateDir, ".cert-new-")
	if err != nil {
		return CertificateMeta{}, err
	}
	defer os.RemoveAll(tempDir)
	if err := os.Chmod(tempDir, 0o700); err != nil {
		return CertificateMeta{}, err
	}
	if err := fileutil.WriteFileAtomic(filepath.Join(tempDir, "fullchain.pem"), append([]byte(input.Certificate), '\n'), 0o600); err != nil {
		return CertificateMeta{}, err
	}
	if err := fileutil.WriteFileAtomic(filepath.Join(tempDir, "privkey.pem"), append([]byte(input.PrivateKey), '\n'), 0o600); err != nil {
		return CertificateMeta{}, err
	}
	if err := os.Rename(tempDir, finalDir); err != nil {
		return CertificateMeta{}, err
	}
	if err := s.store.Update(func(state *State) error {
		state.Certificates = append(state.Certificates, meta)
		state.Dirty = true
		return nil
	}); err != nil {
		_ = os.RemoveAll(finalDir)
		return CertificateMeta{}, err
	}
	return meta, nil
}

func (s *AppService) DeleteCertificate(id string) error {
	if !domain.ValidID(id) {
		return errors.New("证书 ID 不合法")
	}
	state := s.store.Snapshot()
	found := false
	for _, cert := range state.Certificates {
		if cert.ID == id {
			found = true
			break
		}
	}
	if !found {
		return errors.New("找不到指定证书")
	}
	for _, rule := range state.Rules {
		if rule.CertificateID == id {
			return fmt.Errorf("证书仍被规则 %q 使用", rule.Name)
		}
	}
	for _, rule := range state.StreamRules {
		if rule.CertificateID == id {
			return fmt.Errorf("证书仍被 Stream 规则 %q 使用", rule.Name)
		}
	}
	revisions, _ := s.ListRevisions()
	for _, revision := range revisions {
		for _, rule := range revision.State.Rules {
			if rule.CertificateID == id {
				return fmt.Errorf("证书仍被配置历史 %s 引用，请先删除相关历史", revision.ID)
			}
		}
		for _, rule := range revision.State.StreamRules {
			if rule.CertificateID == id {
				return fmt.Errorf("证书仍被配置历史 %s 的 Stream 规则引用，请先删除相关历史", revision.ID)
			}
		}
	}
	if err := s.store.Update(func(state *State) error {
		for index := range state.Certificates {
			if state.Certificates[index].ID == id {
				state.Certificates = append(state.Certificates[:index], state.Certificates[index+1:]...)
				state.Dirty = true
				return nil
			}
		}
		return errors.New("找不到指定证书")
	}); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(s.paths.CertificateDir, id))
}

func (s *AppService) Apply(summary string) (ApplyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.store.Snapshot()
	result, err := s.nginx.Apply(state)
	if err != nil {
		_ = s.store.Update(func(current *State) error {
			current.LastApplyMessage = err.Error()
			return nil
		})
		return ApplyResult{}, err
	}
	now := time.Now().UTC()
	if err := s.store.Update(func(current *State) error {
		current.Dirty = false
		current.LastAppliedAt = &now
		current.LastApplyMessage = result.Message
		return nil
	}); err != nil {
		return result, fmt.Errorf("Nginx 已应用，但保存应用状态失败: %w", err)
	}
	applied := s.store.Snapshot()
	if strings.TrimSpace(summary) == "" {
		summary = "保存并应用配置"
	}
	if err := s.saveRevision(applied, summary); err != nil {
		result.Message += "；配置历史保存失败: " + err.Error()
	}
	return result, nil
}

func (s *AppService) NginxStart() (ApplyResult, error) {
	return s.nginx.Start()
}

func (s *AppService) NginxStop() (ApplyResult, error) {
	return s.nginx.Stop()
}

func (s *AppService) NginxReload() (ApplyResult, error) {
	return s.nginx.Reload()
}

func (s *AppService) NginxTest() (ApplyResult, error) {
	output, err := s.nginx.TestCurrent()
	if err != nil {
		return ApplyResult{}, fmt.Errorf("配置校验失败: %w\n%s", err, output)
	}
	return ApplyResult{Action: "test", Message: "Nginx 配置校验通过", Output: output}, nil
}

func (s *AppService) saveRevision(state State, summary string) error {
	revision := Revision{
		ID:           time.Now().UTC().Format("20060102T150405Z") + "-" + domain.RandomID()[:8],
		CreatedAt:    time.Now().UTC(),
		Summary:      strings.TrimSpace(summary),
		RuleCount:    len(state.Rules) + len(state.StreamRules),
		EnabledCount: domain.EnabledRuleCount(state),
		State:        domain.CloneState(state),
	}
	data, err := json.MarshalIndent(revision, "", "  ")
	if err != nil {
		return err
	}
	if err := fileutil.WriteFileAtomic(filepath.Join(s.paths.RevisionDir, revision.ID+".json"), append(data, '\n'), 0o600); err != nil {
		return err
	}
	return s.pruneRevisions(state.Settings.RevisionLimit)
}

func (s *AppService) ListRevisions() ([]Revision, error) {
	entries, err := os.ReadDir(s.paths.RevisionDir)
	if errors.Is(err, os.ErrNotExist) {
		return []Revision{}, nil
	}
	if err != nil {
		return nil, err
	}
	revisions := make([]Revision, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(s.paths.RevisionDir, entry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		var revision Revision
		if err := json.Unmarshal(data, &revision); err != nil {
			return nil, fmt.Errorf("读取配置历史 %s 失败: %w", entry.Name(), err)
		}
		revisions = append(revisions, revision)
	}
	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].CreatedAt.After(revisions[j].CreatedAt)
	})
	return revisions, nil
}

func (s *AppService) RestoreRevision(id string) (State, error) {
	if strings.ContainsAny(id, `/\\`) || len(id) > 80 {
		return State{}, errors.New("配置历史 ID 不合法")
	}
	data, err := os.ReadFile(filepath.Join(s.paths.RevisionDir, id+".json"))
	if err != nil {
		return State{}, errors.New("找不到指定配置历史")
	}
	var revision Revision
	if err := json.Unmarshal(data, &revision); err != nil {
		return State{}, err
	}
	current := s.store.Snapshot()
	next := domain.CloneState(revision.State)
	// Imported certificate material is global and secret; restoring an old
	// revision must never silently delete certificates added later.
	next.Certificates = current.Certificates
	next.Dirty = true
	next.LastApplyMessage = "已从配置历史恢复为草稿，尚未应用"
	next.LastAppliedAt = current.LastAppliedAt
	if err := s.store.Replace(next); err != nil {
		return State{}, err
	}
	return s.store.Snapshot(), nil
}

func (s *AppService) DeleteRevision(id string) error {
	if strings.ContainsAny(id, `/\\`) || len(id) > 80 {
		return errors.New("配置历史 ID 不合法")
	}
	if err := os.Remove(filepath.Join(s.paths.RevisionDir, id+".json")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("找不到指定配置历史")
		}
		return err
	}
	return nil
}

func (s *AppService) pruneRevisions(limit int) error {
	revisions, err := s.ListRevisions()
	if err != nil {
		return err
	}
	for index := limit; index < len(revisions); index++ {
		if err := os.Remove(filepath.Join(s.paths.RevisionDir, revisions[index].ID+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *AppService) Logs(kind string, limit int) ([]string, error) {
	var path string
	switch kind {
	case "access":
		path = s.paths.NginxAccessLog
	case "error":
		path = s.paths.NginxErrorLog
	case "backend":
		path = s.paths.BackendLog
	case "stream":
		path = s.paths.NginxStreamLog
	default:
		return nil, errors.New("日志类型必须是 access、error、stream 或 backend")
	}
	return fileutil.TailLines(path, limit)
}

func (s *AppService) Config() (ConfigSnapshot, error) {
	master, err := os.ReadFile(s.paths.NginxMaster)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return ConfigSnapshot{}, err
	}
	files := make(map[string]string)
	entries, err := os.ReadDir(s.paths.NginxConfD)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return ConfigSnapshot{}, err
	}
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".conf") && !strings.HasSuffix(entry.Name(), ".stream")) {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(s.paths.NginxConfD, entry.Name()))
		if readErr != nil {
			return ConfigSnapshot{}, readErr
		}
		files[entry.Name()] = string(data)
	}
	return ConfigSnapshot{Master: string(master), Files: files}, nil
}
