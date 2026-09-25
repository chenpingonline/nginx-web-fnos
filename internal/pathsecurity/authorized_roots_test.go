package pathsecurity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
)

type authorizationTestTransport struct {
	target string
	base   http.RoundTripper
}

func (t authorizationTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	u := *r.URL
	clone.URL = &u
	clone.URL.Scheme = "http"
	clone.URL.Host = strings.TrimPrefix(t.target, "http://")
	return t.base.RoundTrip(clone)
}

func TestLiveAuthorizationChanges(t *testing.T) {
	root := t.TempDir()
	site := filepath.Join(root, "site")
	if err := os.Mkdir(site, 0750); err != nil {
		t.Fatal(err)
	}
	var paths = []string{}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/api/v1/trimapp" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("incorrect API request")
		}
		var body struct {
			Req     string `json:"req"`
			AppName string `json:"appName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Req != "trim.file.getSharedAccessibleFolders" || body.AppName != "nginx-web" {
			t.Error("incorrect method or app")
		}
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"paths": paths}})
	}))
	defer server.Close()
	old := authorizationClient
	authorizationClient = &http.Client{Transport: authorizationTestTransport{server.URL, http.DefaultTransport}, Timeout: time.Second}
	t.Cleanup(func() { authorizationClient = old })
	t.Setenv("TRIM_API_TOKEN", "test-token")
	t.Setenv("TRIM_DATA_ACCESSIBLE_PATHS", root) // Deliberately stale: must not grant access.
	t.Setenv("TRIM_DATA_SHARE_PATHS", "")
	if err := ValidateDirectory(site, false); err == nil {
		t.Fatal("stale environment grant was accepted")
	}
	paths = []string{root}
	state := domain.DefaultState()
	state.Rules = []domain.ProxyRule{{Name: "one", RootLocation: domain.LocationSettings{BackendType: "static", StaticPath: site}}, {Name: "two", RootLocation: domain.LocationSettings{BackendType: "static", StaticPath: site}}}
	before := calls
	if err := ValidateState(state); err != nil {
		t.Fatalf("new grant requires restart: %v", err)
	}
	if calls-before != 1 {
		t.Fatal("expected one query per state validation")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(site, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDirectory(filepath.Join(site, "escape"), false); err == nil {
		t.Fatal("symlink escape accepted")
	}
	if os.Geteuid() != 0 {
		if err := os.Chmod(site, 0000); err != nil {
			t.Fatal(err)
		}
		err := ValidateDirectory(site, false)
		os.Chmod(site, 0750)
		if err == nil {
			t.Fatal("unreadable authorized directory accepted")
		}
	}
	paths = []string{}
	if err := ValidateDirectory(site, false); err == nil {
		t.Fatal("revoked grant accepted")
	}
}

func TestAuthorizationFailuresDoNotExposeResponse(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"http", "secret-token", 403},
		{"business", `{"code":1,"msg":"secret-token"}`, 200},
		{"missing code", `{"data":{"paths":[]}}`, 200},
		{"missing paths", `{"code":0,"data":{}}`, 200},
		{"malformed", `not-json-secret-token`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer server.Close()
			client := &http.Client{Transport: authorizationTestTransport{server.URL, http.DefaultTransport}, Timeout: time.Second}
			_, err := queryAuthorizedRoots(client, "secret-token")
			if err == nil {
				t.Fatal("invalid response accepted")
			}
			if strings.Contains(err.Error(), "secret-token") {
				t.Fatal("response or token leaked")
			}
		})
	}
}
