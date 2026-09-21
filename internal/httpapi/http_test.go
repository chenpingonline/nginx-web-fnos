package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/chenpingonline/nginx-web-fnos/internal/platform"
	appservice "github.com/chenpingonline/nginx-web-fnos/internal/service"
	webassets "github.com/chenpingonline/nginx-web-fnos/web"
)

func TestAPIRequiresAdministratorHeaders(t *testing.T) {
	t.Setenv("FNPROXY_DEV_ALLOW", "0")
	api := New(nil, webassets.Assets)

	request := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without admin header, got %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/apply", strings.NewReader(`{}`))
	request.Header.Set("X-Trim-Isadmin", "true")
	response = httptest.NewRecorder()
	api.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without mutation marker, got %d", response.Code)
	}
}

func TestDashboardRequiresAdminAndBoundsHistoryRange(t *testing.T) {
	t.Setenv("FNPROXY_DEV_ALLOW", "0")
	api := New(nil, webassets.Assets)
	r := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("unauthorized metrics request returned %d", w.Code)
	}
	for _, value := range []string{"bad", "0", "1000000", "-1"} {
		r := httptest.NewRequest(http.MethodGet, "/api/dashboard?minutes="+value, nil)
		r.Header.Set("X-Trim-Isadmin", "true")
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid range %s returned %d", value, w.Code)
		}
	}
}

func TestAPIHealthAndEmbeddedIndex(t *testing.T) {
	t.Setenv("FNPROXY_DEV_ALLOW", "0")
	api := New(nil, webassets.Assets)
	for _, target := range []string{"/healthz", "/"} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		response := httptest.NewRecorder()
		api.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", target, response.Code)
		}
	}
}

func TestIndexBootstrapIsAllowedByContentSecurityPolicy(t *testing.T) {
	api := New(nil, webassets.Assets)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected embedded index, got %d", response.Code)
	}
	csp := response.Header().Get("Content-Security-Policy")
	body := response.Body.String()
	for _, expression := range []string{`(?s)<style>(.*?)</style>`, `(?s)<script>(.*?)</script>`} {
		match := regexp.MustCompile(expression).FindStringSubmatch(body)
		if len(match) != 2 {
			t.Fatalf("missing inline bootstrap element matching %s", expression)
		}
		digest := sha256.Sum256([]byte(match[1]))
		hash := "'sha256-" + base64.StdEncoding.EncodeToString(digest[:]) + "'"
		if !strings.Contains(csp, hash) {
			t.Fatalf("content security policy does not allow bootstrap %s", hash)
		}
	}
}

func TestInternalBasicAuthUsesAppliedProfilesAndIsNotGatewayExposed(t *testing.T) {
	t.Setenv("FNPROXY_DEV_ALLOW", "1")
	root := t.TempDir()
	t.Setenv("FNPROXY_APPDEST", filepath.Join(root, "app"))
	t.Setenv("FNPROXY_ETC", filepath.Join(root, "etc"))
	t.Setenv("FNPROXY_VAR", filepath.Join(root, "var"))
	t.Setenv("FNPROXY_TMP", filepath.Join(root, "tmp"))
	paths, err := platform.LoadPaths()
	if err != nil {
		t.Fatal(err)
	}
	service, err := appservice.New(paths)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := service.CreateAuthProfile(appservice.AuthProfileInput{Name: "Family", Realm: "Private", Users: []appservice.AuthUserInput{{Username: "alice", Password: "correct horse", Enabled: true}}})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := json.Marshal(service.State())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.AppliedState(), applied, 0o600); err != nil {
		t.Fatal(err)
	}
	service, err = appservice.New(paths)
	if err != nil {
		t.Fatal(err)
	}
	api := New(service, webassets.Assets)

	request := httptest.NewRequest(http.MethodGet, "/internal/basic-auth/"+profile.ID, nil)
	request.SetBasicAuth("alice", "correct horse")
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("valid applied credentials returned %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, gatewayPrefix+"/internal/basic-auth/"+profile.ID, nil)
	request.SetBasicAuth("alice", "wrong")
	response = httptest.NewRecorder()
	api.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("gateway unexpectedly exposed internal auth endpoint: %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/state", nil)
	response = httptest.NewRecorder()
	api.ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "password_hash") || strings.Contains(response.Body.String(), "$2") {
		t.Fatalf("public state exposed password hash: %s", response.Body.String())
	}
}
