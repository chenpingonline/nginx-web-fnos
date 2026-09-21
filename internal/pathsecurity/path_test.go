package pathsecurity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
)

func TestAuthorizedPathBoundaries(t *testing.T) {
	root := t.TempDir()
	allowed := filepath.Join(root, "allowed")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(filepath.Join(allowed, "static"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o750); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(allowed, "auth.txt")
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(file, []byte("user:hash\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsideFile, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	prefixDir := filepath.Join(root, "allowed-other")
	if err := os.MkdirAll(prefixDir, 0o750); err != nil {
		t.Fatal(err)
	}
	prefixFile := filepath.Join(prefixDir, "auth.txt")
	if err := os.WriteFile(prefixFile, []byte("user:hash\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRIM_DATA_ACCESSIBLE_PATHS", allowed)
	t.Setenv("TRIM_DATA_SHARE_PATHS", "")

	if err := ValidateFile(file); err != nil {
		t.Fatalf("authorized file rejected: %v", err)
	}
	if err := ValidateDirectory(filepath.Join(allowed, "static"), true); err != nil {
		t.Fatalf("authorized directory rejected: %v", err)
	}
	for name, candidate := range map[string]string{
		"relative":         "auth.txt",
		"parent traversal": allowed + "/static/../auth.txt",
		"outside":          outsideFile,
		"prefix collision": prefixFile,
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateFile(candidate); err == nil {
				t.Fatalf("unsafe path accepted: %s", candidate)
			}
		})
	}
	if err := os.Symlink(outsideFile, filepath.Join(allowed, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFile(filepath.Join(allowed, "escape")); err == nil {
		t.Fatal("symlink escaping the authorized root was accepted")
	}
	if err := ValidateFile(filepath.Join(allowed, "static")); err == nil {
		t.Fatal("directory accepted as a regular file")
	}
}

func TestValidateStateChecksExternalNginxPaths(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "www")
	authFile := filepath.Join(root, ".htpasswd")
	caFile := filepath.Join(root, "client-ca.pem")
	if err := os.Mkdir(staticDir, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{authFile, caFile} {
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TRIM_DATA_ACCESSIBLE_PATHS", root)
	state := domain.DefaultState()
	state.Settings.TLS.ClientVerify = "on"
	state.Settings.TLS.ClientCAFile = caFile
	rule := domain.ProxyRule{Name: "static", RootLocation: domain.LocationSettings{BackendType: "static", StaticPath: staticDir, BasicAuth: true, BasicAuthFile: authFile}}
	state.Rules = []domain.ProxyRule{rule}
	if err := ValidateState(state); err != nil {
		t.Fatalf("authorized state rejected: %v", err)
	}
	state.Rules[0].RootLocation.StaticPath = filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(state.Rules[0].RootLocation.StaticPath, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := ValidateState(state); err == nil {
		t.Fatal("state with unauthorized static root was accepted")
	}
}
