package adminauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testManager(t *testing.T) *Manager {
	t.Helper()
	t.Setenv("FNPROXY_ADMIN_USER", "admin")
	t.Setenv("FNPROXY_ADMIN_PASSWORD", "correct-password")
	t.Setenv("FNPROXY_ADMIN_PASSWORD_FILE", "")
	t.Setenv("FNPROXY_COOKIE_SECURE", "0")
	m, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func authRequest(m *Manager, method, path, body string, cookie *http.Cookie, csrf, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://example.test"+path, strings.NewReader(body))
	r.Header.Set("X-FnProxy-Request", "1")
	r.Header.Set("X-CSRF-Token", csrf)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && !Authenticated(r.Context()) {
			panic("request did not carry authenticated context")
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(w, r)
	return w
}

func loginSession(t *testing.T, m *Manager) (*http.Cookie, string) {
	t.Helper()
	origin := "http://example.test"
	if m.secure {
		origin = "https://example.test"
	}
	w := authRequest(m, "POST", "/api/auth/login", `{"username":"admin","password":"correct-password"}`, nil, "", origin)
	if w.Code != http.StatusOK {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("expected one session cookie")
	}
	var body struct {
		CSRF string `json:"csrf_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.CSRF == "" || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("missing cookie protections or CSRF token")
	}
	return cookies[0], body.CSRF
}

func TestSessionCSRFLogoutAndExpiry(t *testing.T) {
	m := testManager(t)
	cookie, csrf := loginSession(t, m)
	for _, test := range []struct {
		name, method, path, csrf, origin string
		cookie                           *http.Cookie
		status                           int
	}{
		{"unauthenticated", "GET", "/api/state", "", "", nil, 401},
		{"authenticated", "GET", "/api/state", "", "", cookie, 204},
		{"CSRF missing", "POST", "/api/apply", "", "", cookie, 403},
		{"CSRF wrong", "POST", "/api/apply", "wrong", "", cookie, 403},
		{"origin wrong", "POST", "/api/apply", csrf, "http://evil.test", cookie, 403},
		{"origin null", "POST", "/api/apply", csrf, "null", cookie, 403},
		{"valid mutation", "POST", "/api/apply", csrf, "http://example.test", cookie, 204},
		{"private callback", "GET", "/internal/basic-auth/id", "", "", cookie, 404},
		{"prefixed callback", "GET", "/app/nginx-web/internal/basic-auth/id", "", "", cookie, 404},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := authRequest(m, test.method, test.path, "{}", test.cookie, test.csrf, test.origin)
			if w.Code != test.status {
				t.Fatalf("got %d, want %d: %s", w.Code, test.status, w.Body.String())
			}
		})
	}
	w := authRequest(m, "POST", "/api/auth/logout", "{}", cookie, csrf, "")
	if w.Code != 200 || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not clear cookie")
	}
	if authRequest(m, "GET", "/api/state", "", cookie, "", "").Code != 401 {
		t.Fatal("logged-out session remains valid")
	}
	cookie, _ = loginSession(t, m)
	now := time.Now().Add(sessionLifetime + time.Second)
	m.now = func() time.Time { return now }
	if authRequest(m, "GET", "/api/state", "", cookie, "", "").Code != 401 {
		t.Fatal("expired session remains valid")
	}
}

func TestRejectLoginForgeryLimitAttemptsAndRotateSession(t *testing.T) {
	m := testManager(t)
	if authRequest(m, "POST", "/api/auth/login", `{}`, nil, "", "http://evil.test").Code != 403 {
		t.Fatal("cross-origin login accepted")
	}
	r := httptest.NewRequest("POST", "http://example.test/api/auth/login", strings.NewReader(`{}`))
	r.Header.Set("X-Trim-Isadmin", "true")
	w := httptest.NewRecorder()
	m.Handler(http.NotFoundHandler()).ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("login accepted without request marker")
	}
	cookie, _ := loginSession(t, m)
	w = authRequest(m, "POST", "/api/auth/login", `{"username":"admin","password":"correct-password"}`, cookie, "", "")
	if w.Code != 200 || w.Result().Cookies()[0].Value == cookie.Value {
		t.Fatal("session did not rotate")
	}
	if authRequest(m, "GET", "/api/state", "", cookie, "", "").Code != 401 {
		t.Fatal("old session not revoked")
	}
	for range 8 {
		if authRequest(m, "POST", "/api/auth/login", `{"username":"admin","password":"incorrect"}`, nil, "", "").Code != 401 {
			t.Fatal("wrong password accepted or prematurely rate limited")
		}
	}
	if authRequest(m, "POST", "/api/auth/login", `{}`, nil, "", "").Code != 429 {
		t.Fatal("login limit missing")
	}
}

func TestBootstrapPersistResetAndSecretFile(t *testing.T) {
	t.Setenv("FNPROXY_ADMIN_PASSWORD", "")
	t.Setenv("FNPROXY_ADMIN_PASSWORD_FILE", "")
	t.Setenv("FNPROXY_ADMIN_USER", "")
	t.Setenv("FNPROXY_COOKIE_SECURE", "1")
	root := t.TempDir()
	if _, err := Load(root); err == nil {
		t.Fatal("empty bootstrap accepted")
	}
	secret := filepath.Join(root, "password")
	if err := os.WriteFile(secret, []byte("correct-password\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FNPROXY_ADMIN_PASSWORD_FILE", secret)
	m, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	cookie, _ := loginSession(t, m)
	if !cookie.Secure {
		t.Fatal("secure cookie flag missing")
	}
	data, err := os.ReadFile(filepath.Join(root, "admin.json"))
	if err != nil || strings.Contains(string(data), "correct-password") {
		t.Fatal("plaintext password persisted")
	}
	info, _ := os.Stat(filepath.Join(root, "admin.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("credential file permissions")
	}
	t.Setenv("FNPROXY_ADMIN_PASSWORD_FILE", "")
	m, err = Load(root)
	if err != nil {
		t.Fatal(err)
	}
	loginSession(t, m)
	t.Setenv("FNPROXY_ADMIN_PASSWORD", "new-password")
	m, err = Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if authRequest(m, "POST", "/api/auth/login", `{"username":"admin","password":"correct-password"}`, nil, "", "").Code != 401 {
		t.Fatal("old password survived reset")
	}
	if authRequest(m, "POST", "/api/auth/login", `{"username":"admin","password":"new-password"}`, nil, "", "").Code != 200 {
		t.Fatal("reset password rejected")
	}
	t.Setenv("FNPROXY_ADMIN_PASSWORD", "short")
	if _, err := Load(root); err == nil {
		t.Fatal("short password accepted")
	}
	t.Setenv("FNPROXY_ADMIN_PASSWORD", "")
	t.Setenv("FNPROXY_ADMIN_PASSWORD_FILE", secret)
	os.WriteFile(secret, []byte(strings.Repeat("x", 72)+"\r\nextra"), 0600)
	if _, err := Load(root); err == nil {
		t.Fatal("oversized secret truncated and accepted")
	}
}
