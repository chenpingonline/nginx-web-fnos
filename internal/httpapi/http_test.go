package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
