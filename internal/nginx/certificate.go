package nginx

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/fileutil"
)

// DeployCertificate switches a managed certificate as one symlink transaction.
// It only reloads the installed configuration; draft rules are never rendered.
func (m *Manager) DeployCertificate(id string, chain, key []byte, commit func() error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !domain.ValidID(id) {
		return errors.New("证书 ID 无效")
	}
	versions := filepath.Join(m.paths.CertificateDir, ".acme-versions", id)
	if err := os.MkdirAll(versions, 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(versions, "version-")
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(stage)
		}
	}()
	if err = fileutil.WriteFileAtomic(filepath.Join(stage, "fullchain.pem"), chain, 0600); err != nil {
		return err
	}
	if err = fileutil.WriteFileAtomic(filepath.Join(stage, "privkey.pem"), key, 0600); err != nil {
		return err
	}
	final := filepath.Join(m.paths.CertificateDir, id)
	old, err := os.Readlink(final)
	if err != nil {
		if _, e := os.Lstat(final); !os.IsNotExist(e) {
			return errors.New("证书路径不是 ACME 管理的链接")
		}
		old = ""
	}
	switchTo := func(target string) error {
		if target == "" {
			return os.Remove(final)
		}
		link := stage + "-link"
		_ = os.Remove(link)
		defer os.Remove(link)
		if e := os.Symlink(target, link); e != nil {
			return e
		}
		return os.Rename(link, final)
	}
	if err = switchTo(stage); err != nil {
		return err
	}
	_, running := m.runningPID()
	rollback := func(cause error) error {
		if e := switchTo(old); e != nil {
			keep = true
			return errors.Join(cause, errors.New("恢复旧证书失败"), e)
		}
		if running {
			if _, e := m.reloadUnlocked(); e != nil {
				return errors.Join(cause, errors.New("恢复证书后的重载失败"), e)
			}
		}
		return cause
	}
	if running {
		if _, err = m.reloadUnlocked(); err != nil {
			return rollback(err)
		}
	}
	if err = commit(); err != nil {
		return rollback(err)
	}
	keep = true
	// Retain the previous version for diagnostics; older versions contain private material.
	entries, _ := os.ReadDir(versions)
	for _, e := range entries {
		p := filepath.Join(versions, e.Name())
		if e.IsDir() && p != stage && p != old {
			_ = os.RemoveAll(p)
		}
	}
	return nil
}
