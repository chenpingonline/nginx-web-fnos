package acme

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/fileutil"
	"github.com/go-acme/lego/v5/certificate"
)

type DeployFunc func(id, name string, resource *certificate.Resource) (string, error)
type IssueFunc func(context.Context, *Record, func() error, func(string)) error
type Manager struct {
	mu      sync.Mutex
	dir     string
	records map[string]*Record
	active  string
	deploy  DeployFunc
	issue   IssueFunc
	now     func() time.Time
	wake    chan struct{}
}

func New(dir string, deploy DeployFunc) (*Manager, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return nil, err
	}
	m := &Manager{dir: dir, records: map[string]*Record{}, deploy: deploy, issue: issue, now: time.Now, wake: make(chan struct{}, 1)}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	for _, p := range files {
		b, e := os.ReadFile(p)
		if e != nil {
			return nil, e
		}
		var r Record
		if json.Unmarshal(b, &r) != nil || !domain.ValidID(r.Job.ID) || filepath.Base(p) != r.Job.ID+".json" {
			return nil, errors.New("ACME 任务文件损坏")
		}
		if err := validate(&r.Input); err != nil {
			return nil, fmt.Errorf("ACME 任务 %s 配置无效", r.Job.ID)
		}
		if r.Job.Status == "running" {
			r.Job.Status = "queued"
			r.Job.Message = "上次任务中断，等待恢复"
			r.Job.NextAttempt = m.now()
		}
		m.records[r.Job.ID] = &r
	}
	return m, nil
}
func clone(r *Record) *Record {
	b, _ := json.Marshal(r)
	var c Record
	_ = json.Unmarshal(b, &c)
	return &c
}
func (m *Manager) save(r *Record) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err = fileutil.WriteFileAtomic(filepath.Join(m.dir, r.Job.ID+".json"), b, 0600); err != nil {
		return err
	}
	m.records[r.Job.ID] = clone(r)
	return nil
}
func (m *Manager) signal() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}
func (m *Manager) Create(in Input) (Job, error) {
	if err := validate(&in); err != nil {
		return Job{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.records) >= 100 {
		return Job{}, errors.New("最多支持 100 个 ACME 任务")
	}
	r := &Record{Input: in, Job: Job{ID: domain.RandomID(), Name: in.Name, CA: in.CA, Domains: in.Domains, Provider: in.Provider, KeyType: in.KeyType, Enabled: true, Status: "queued", Message: "等待申请", NextAttempt: m.now(), UpdatedAt: m.now()}}
	if err := m.save(r); err != nil {
		return Job{}, err
	}
	m.signal()
	return r.Job, nil
}
func (m *Manager) List() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	jobs := []Job{}
	for _, r := range m.records {
		j := r.Job
		j.Domains = append([]string(nil), j.Domains...)
		jobs = append(jobs, j)
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].ID < jobs[j].ID })
	return jobs
}
func (m *Manager) Manages(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.records {
		if r.Job.CertificateID == id || r.Job.ID == id {
			return true
		}
	}
	return false
}
func (m *Manager) Action(id, action string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.records[id]
	if !ok {
		return errors.New("ACME 任务不存在")
	}
	if m.active == id {
		return errors.New("任务执行中，请等待完成后操作")
	}
	r = clone(r)
	switch action {
	case "retry":
		if !r.Job.Enabled {
			return errors.New("请先启用自动续期")
		}
		if m.now().Before(r.Job.UpdatedAt.Add(time.Minute)) {
			return errors.New("请至少间隔一分钟再重试")
		}
		r.Job.NextAttempt = m.now()
		r.Job.Status = "queued"
		r.Force = true
		r.Job.Message = "等待执行"
	case "pause":
		r.Job.Enabled = false
		r.Job.Message = "自动续期已暂停"
	case "resume":
		r.Job.Enabled = true
		r.Job.NextAttempt = m.now()
		r.Job.Message = "自动续期已启用"
	case "delete":
		if err := os.Remove(filepath.Join(m.dir, id+".json")); err != nil {
			return err
		}
		delete(m.records, id)
		return nil
	default:
		return errors.New("不支持的任务操作")
	}
	r.Job.UpdatedAt = m.now()
	if err := m.save(r); err != nil {
		return err
	}
	m.signal()
	return nil
}
func (m *Manager) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		m.step(ctx)
		select {
		case <-ctx.Done():
			return
		case <-m.wake:
		case <-ticker.C:
		}
	}
}
func (m *Manager) step(ctx context.Context) {
	m.mu.Lock()
	var r *Record
	for _, candidate := range m.records {
		if candidate.Job.Enabled && !candidate.Job.NextAttempt.After(m.now()) && (r == nil || candidate.Job.NextAttempt.Before(r.Job.NextAttempt)) {
			r = clone(candidate)
		}
	}
	if r == nil || m.active != "" {
		m.mu.Unlock()
		return
	}
	m.active = r.Job.ID
	r.Job.Status = "running"
	r.Job.Message = "准备申请"
	r.Job.UpdatedAt = m.now()
	if err := m.save(r); err != nil {
		m.active = ""
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	checkpoint := func() error { m.mu.Lock(); defer m.mu.Unlock(); return m.save(r) }
	progress := func(message string) { r.Job.Message = message; r.Job.UpdatedAt = m.now(); _ = checkpoint() }
	workCtx, cancel := context.WithTimeout(ctx, 35*time.Minute)
	defer cancel()
	var err error
	if r.Pending {
		if c, e := leaf(r.Resource); e != nil || !c.NotAfter.After(m.now()) {
			r.Pending = false
			r.Force = true
		}
	}
	if !r.Pending {
		err = m.issue(workCtx, r, checkpoint, progress)
	}
	if err == nil && workCtx.Err() != nil {
		err = workCtx.Err()
	}
	if err == nil && r.Pending {
		progress("部署证书")
		var id string
		targetID := r.Job.CertificateID
		if targetID == "" {
			targetID = r.Job.ID
		}
		id, err = m.deploy(targetID, r.Input.Name, r.Resource)
		if err == nil {
			r.Job.CertificateID = id
			r.Pending = false
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = ""
	r.Job.UpdatedAt = m.now()
	if err != nil {
		// Never publish raw provider responses: they may contain credentials or validation tokens.
		r.Job.Status = "failed"
		r.Job.Failures++
		stage := r.Job.Message
		r.Job.Message = stage + "失败；请检查 DNS 凭据、域名解析、CA/EAB 配置及 Nginx 状态"
		if r.Pending {
			r.Job.Message = "证书已签发，部署失败；旧证书保留，将重试部署"
		}
		delay := time.Minute * 5 * time.Duration(1<<min(r.Job.Failures-1, 9))
		if delay > 24*time.Hour {
			delay = 24 * time.Hour
		}
		r.Job.NextAttempt = m.now().Add(delay)
	} else {
		r.Job.Status = "ready"
		r.Job.Message = "证书已保存，自动续期已启用"
		r.Job.Failures = 0
		r.Force = false
		if c, e := leaf(r.Resource); e == nil {
			r.Job.NotAfter = &c.NotAfter
			r.Job.NextAttempt = c.NotAfter.Add(-c.NotAfter.Sub(c.NotBefore) / 3)
		}
		if r.RenewAt != nil {
			r.Job.NextAttempt = *r.RenewAt
		}
		if r.Job.NextAttempt.After(m.now().Add(6 * time.Hour)) {
			r.Job.NextAttempt = m.now().Add(6 * time.Hour)
		}
		if !r.Job.NextAttempt.After(m.now()) {
			r.Job.NextAttempt = m.now().Add(time.Hour)
		}
	}
	if e := m.save(r); e != nil {
		// Keep the successful in-memory result so a transient disk failure does not immediately reissue.
		r.Job.Status = "failed"
		r.Job.Message = "无法保存 ACME 任务状态，请检查磁盘空间与权限"
		m.records[r.Job.ID] = clone(r)
	}
}
