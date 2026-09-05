package platform

import (
	"errors"
	"os"
	"path/filepath"
)

type Paths struct {
	AppDest        string
	EtcDir         string
	VarDir         string
	TmpDir         string
	SocketPath     string
	NginxBin       string
	MimeTypes      string
	StateFile      string
	CertificateDir string
	RevisionDir    string
	NginxPrefix    string
	NginxConfigDir string
	NginxConfD     string
	NginxMaster    string
	NginxRunDir    string
	NginxPID       string
	NginxLogDir    string
	NginxAccessLog string
	NginxErrorLog  string
	NginxStreamLog string
	NginxTempDir   string
	NginxCacheDir  string
	BackendLog     string
}

// Monitoring stays inside this application's private runtime and data directories.
func (p Paths) StatusSocket() string   { return filepath.Join(p.NginxRunDir, "status.sock") }
func (p Paths) MetricsLog() string     { return filepath.Join(p.NginxLogDir, "http-metrics.log") }
func (p Paths) MetricsHistory() string { return filepath.Join(p.VarDir, "metrics", "history.json") }
func (p Paths) AppliedState() string   { return filepath.Join(p.VarDir, "applied-state.json") }

func LoadPaths() (Paths, error) {
	appDest := firstNonEmpty(os.Getenv("FNPROXY_APPDEST"), os.Getenv("TRIM_APPDEST"))
	etcDir := firstNonEmpty(os.Getenv("FNPROXY_ETC"), os.Getenv("TRIM_PKGETC"))
	varDir := firstNonEmpty(os.Getenv("FNPROXY_VAR"), os.Getenv("TRIM_PKGVAR"))
	tmpDir := firstNonEmpty(os.Getenv("FNPROXY_TMP"), os.Getenv("TRIM_PKGTMP"))

	if appDest == "" || etcDir == "" || varDir == "" || tmpDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return Paths{}, err
		}
		devRoot := filepath.Join(cwd, ".fnproxy-dev")
		if appDest == "" {
			appDest = filepath.Join(devRoot, "app")
		}
		if etcDir == "" {
			etcDir = filepath.Join(devRoot, "etc")
		}
		if varDir == "" {
			varDir = filepath.Join(devRoot, "var")
		}
		if tmpDir == "" {
			tmpDir = filepath.Join(devRoot, "tmp")
		}
	}

	appDest, _ = filepath.Abs(appDest)
	etcDir, _ = filepath.Abs(etcDir)
	varDir, _ = filepath.Abs(varDir)
	tmpDir, _ = filepath.Abs(tmpDir)

	nginxPrefix := filepath.Join(varDir, "nginx")
	configDir := filepath.Join(etcDir, "nginx")
	logDir := filepath.Join(varDir, "logs")
	runDir := filepath.Join(nginxPrefix, "run")

	paths := Paths{
		AppDest:        appDest,
		EtcDir:         etcDir,
		VarDir:         varDir,
		TmpDir:         tmpDir,
		SocketPath:     firstNonEmpty(os.Getenv("FNPROXY_SOCKET"), filepath.Join(appDest, "app.sock")),
		NginxBin:       filepath.Join(appDest, "bin", "nginx"),
		MimeTypes:      filepath.Join(appDest, "etc", "mime.types"),
		StateFile:      filepath.Join(varDir, "fnproxy.json"),
		CertificateDir: filepath.Join(varDir, "certificates"),
		RevisionDir:    filepath.Join(configDir, "revisions"),
		NginxPrefix:    nginxPrefix,
		NginxConfigDir: configDir,
		NginxConfD:     filepath.Join(configDir, "conf.d"),
		NginxMaster:    filepath.Join(configDir, "nginx.conf"),
		NginxRunDir:    runDir,
		NginxPID:       filepath.Join(runDir, "nginx.pid"),
		NginxLogDir:    logDir,
		NginxAccessLog: filepath.Join(logDir, "nginx-access.log"),
		NginxErrorLog:  filepath.Join(logDir, "nginx-error.log"),
		NginxStreamLog: filepath.Join(logDir, "nginx-stream-access.log"),
		NginxTempDir:   filepath.Join(tmpDir, "nginx"),
		NginxCacheDir:  filepath.Join(varDir, "cache"),
		BackendLog:     filepath.Join(logDir, "nginx-web-server.log"),
	}
	return paths, paths.Ensure()
}

func (p Paths) Ensure() error {
	if p.AppDest == "" || p.EtcDir == "" || p.VarDir == "" || p.TmpDir == "" {
		return errors.New("运行目录未正确配置")
	}
	dirs := []struct {
		path string
		mode os.FileMode
	}{
		{p.EtcDir, 0o750}, {p.VarDir, 0o750}, {p.TmpDir, 0o750},
		{p.CertificateDir, 0o700}, {p.RevisionDir, 0o700},
		{p.NginxPrefix, 0o750}, {p.NginxConfigDir, 0o750}, {p.NginxConfD, 0o750},
		{p.NginxRunDir, 0o700}, {p.NginxLogDir, 0o750}, {p.NginxTempDir, 0o750},
		// Nginx opens its compile-time default error log before reading -c.
		// Keep these private compatibility directories even though the active
		// configuration writes logs and temp files elsewhere.
		{filepath.Join(p.NginxPrefix, "logs"), 0o750},
		{filepath.Join(p.NginxPrefix, "temp", "body"), 0o750},
		{filepath.Join(p.NginxPrefix, "temp", "proxy"), 0o750},
		{filepath.Join(p.NginxPrefix, "temp", "fastcgi"), 0o750},
		{filepath.Join(p.NginxPrefix, "temp", "scgi"), 0o750},
		{filepath.Join(p.NginxPrefix, "temp", "uwsgi"), 0o750},
		{filepath.Join(p.NginxTempDir, "body"), 0o750},
		{filepath.Join(p.NginxTempDir, "proxy"), 0o750},
		{filepath.Join(p.NginxTempDir, "fastcgi"), 0o750},
		{filepath.Join(p.NginxTempDir, "scgi"), 0o750},
		{filepath.Join(p.NginxTempDir, "uwsgi"), 0o750},
		{p.NginxCacheDir, 0o750},
	}
	for _, dir := range dirs {
		if dir.path == "" {
			continue
		}
		if err := os.MkdirAll(dir.path, dir.mode); err != nil {
			return err
		}
		_ = os.Chmod(dir.path, dir.mode)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
