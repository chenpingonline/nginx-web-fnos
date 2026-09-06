package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenpingonline/fn-nginx-web/internal/platform"
	"github.com/chenpingonline/fn-nginx-web/internal/service"
	webassets "github.com/chenpingonline/fn-nginx-web/web"
)

func TestACMEAPIProtectsCredentialsAndRequiresAdmin(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FNPROXY_APPDEST", filepath.Join(root, "app"))
	t.Setenv("FNPROXY_ETC", filepath.Join(root, "etc"))
	t.Setenv("FNPROXY_VAR", filepath.Join(root, "var"))
	t.Setenv("FNPROXY_TMP", filepath.Join(root, "tmp"))
	t.Setenv("FNPROXY_DEV_ALLOW", "0")
	paths, e := platform.LoadPaths()
	if e != nil {
		t.Fatal(e)
	}
	s, e := service.New(paths)
	if e != nil {
		t.Fatal(e)
	}
	api := New(s, webassets.Assets)
	body := `{"name":"test","ca":"staging","email":"admin@example.com","domains":["example.com"],"provider":"cloudflare","key_type":"ec256","accept_terms":true,"dns_config":{"CF_DNS_API_TOKEN":"never-return-this"}}`
	call := func(method, path, body string, admin bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if admin {
			r.Header.Set("X-Trim-Isadmin", "true")
			r.Header.Set("X-FnProxy-Request", "1")
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	if w := call(http.MethodPost, "/api/acme", body, false); w.Code != 403 {
		t.Fatal("unauthorized request accepted", w.Code)
	}
	catalogResponse := call(http.MethodGet, "/api/acme/providers", "", true)
	if catalogResponse.Code != 200 || !strings.Contains(catalogResponse.Body.String(), "dnsla") || strings.Contains(catalogResponse.Body.String(), "stackpath") {
		t.Fatal("wrong provider catalog")
	}
	w := call(http.MethodPost, "/api/acme", body, true)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var job struct {
		ID string `json:"id"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &job); e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{"/api/acme", "/api/state", "/api/revisions"} {
		w = call(http.MethodGet, path, "", true)
		if w.Code != 200 || strings.Contains(w.Body.String(), "never-return-this") || strings.Contains(w.Body.String(), "credentials") {
			t.Fatal("public API exposed credentials", path)
		}
	}
	if w = call(http.MethodPost, "/api/acme/"+job.ID+"/pause", "{}", true); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = call(http.MethodDelete, "/api/acme/"+job.ID, "", true); w.Code != 200 {
		t.Fatal(w.Code)
	}
}
