package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	webassets "github.com/chenpingonline/fn-nginx-web/web"
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
