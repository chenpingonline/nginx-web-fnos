package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPathsUsesExplicitEnvironment(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FNPROXY_APPDEST", filepath.Join(root, "app"))
	t.Setenv("FNPROXY_ETC", filepath.Join(root, "etc"))
	t.Setenv("FNPROXY_VAR", filepath.Join(root, "var"))
	t.Setenv("FNPROXY_TMP", filepath.Join(root, "tmp"))
	t.Setenv("FNPROXY_SOCKET", filepath.Join(root, "socket", "app.sock"))

	paths, err := LoadPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.SocketPath != filepath.Join(root, "socket", "app.sock") {
		t.Fatalf("unexpected socket path: %s", paths.SocketPath)
	}
	if paths.BackendLog != filepath.Join(root, "var", "logs", "nginx-web-server.log") {
		t.Fatalf("unexpected backend log path: %s", paths.BackendLog)
	}
	for _, dir := range []string{paths.EtcDir, paths.VarDir, paths.TmpDir, paths.CertificateDir, paths.NginxConfD} {
		info, statErr := os.Stat(dir)
		if statErr != nil || !info.IsDir() {
			t.Fatalf("expected directory %s: %v", dir, statErr)
		}
	}
}
