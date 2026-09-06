package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenpingonline/nginx-web-fnos/internal/platform"
	"github.com/chenpingonline/nginx-web-fnos/internal/service"
	webassets "github.com/chenpingonline/nginx-web-fnos/web"
)

func TestBackupAPIRequiresAdminAndRestoreMarker(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FNPROXY_APPDEST", filepath.Join(root, "app"))
	t.Setenv("FNPROXY_ETC", filepath.Join(root, "etc"))
	t.Setenv("FNPROXY_VAR", filepath.Join(root, "var"))
	t.Setenv("FNPROXY_TMP", filepath.Join(root, "tmp"))
	t.Setenv("FNPROXY_DEV_ALLOW", "0")
	paths, err := platform.LoadPaths()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := service.New(paths)
	if err != nil {
		t.Fatal(err)
	}
	api := New(svc, webassets.Assets)
	call := func(method, path, body string, admin, marker bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if admin {
			req.Header.Set("X-Trim-Isadmin", "true")
		}
		if marker {
			req.Header.Set("X-FnProxy-Request", "1")
		}
		res := httptest.NewRecorder()
		api.ServeHTTP(res, req)
		return res
	}
	if r := call("GET", "/api/backup", "", false, false); r.Code != 403 {
		t.Fatal("export accepted without admin")
	}
	backup := call("GET", "/api/backup", "", true, false)
	if backup.Code != 200 || backup.Header().Get("Cache-Control") != "no-store" || !strings.Contains(backup.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("backup download invalid", backup.Code)
	}
	if r := call("POST", "/api/backup/restore", backup.Body.String(), true, false); r.Code != 403 {
		t.Fatal("restore accepted without mutation marker")
	}
	if r := call("POST", "/api/backup/restore", backup.Body.String()+" {}", true, true); r.Code != http.StatusBadRequest {
		t.Fatal("trailing JSON accepted", r.Code)
	}
	if r := call("POST", "/api/backup/restore", backup.Body.String(), true, true); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if !svc.State().Dirty {
		t.Fatal("restore should remain a draft")
	}
}
