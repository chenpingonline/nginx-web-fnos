package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/platform"
	"github.com/chenpingonline/nginx-web-fnos/internal/service"
	webassets "github.com/chenpingonline/nginx-web-fnos/web"
	"github.com/gorilla/websocket"
)

func gatewayProxyTestAPI(t *testing.T, upstream *httptest.Server) (*API, domain.ProxyRule) {
	t.Helper()
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
	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	port := target.Port()
	rule, err := svc.CreateRule(domain.ProxyRule{
		Name: "FN Connect Test", Enabled: true, ListenPort: 18418,
		Domains:        []string{"fn-connect.test"},
		UpstreamScheme: target.Scheme, UpstreamHost: target.Hostname(),
		UpstreamPort: mustAtoi(t, port), PreserveHost: false, WebSocket: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return New(svc, webassets.Assets), rule
}

func mustAtoi(t *testing.T, value string) int {
	t.Helper()
	var result int
	if _, err := fmt.Sscanf(value, "%d", &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestGatewayProxyForwardsPathQueryMethodAndProtectsIdentity(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Trim-Username") != "" || r.Header.Get("X-Trim-Isadmin") != "" {
			t.Error("fnOS identity leaked to upstream")
		}
		if r.Header.Get("X-Forwarded-Prefix") == "" {
			t.Error("missing forwarded prefix")
		}
		body, _ := io.ReadAll(r.Body)
		fmt.Fprintf(w, "%s %s?%s %s", r.Method, r.URL.Path, r.URL.RawQuery, body)
	}))
	defer upstream.Close()
	api, rule := gatewayProxyTestAPI(t, upstream)

	req := httptest.NewRequest(http.MethodPost, gatewayProxyPath+rule.ID+"/api/items?q=1", strings.NewReader("payload"))
	req.Header.Set("X-Trim-Isadmin", "true")
	req.Header.Set("X-Trim-Username", "admin")
	res := httptest.NewRecorder()
	api.ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Body.String() != "POST /api/items?q=1 payload" {
		t.Fatalf("unexpected proxy response: %d %q", res.Code, res.Body.String())
	}
}

func TestGatewayProxyRequiresAdminAndEnabledRule(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer upstream.Close()
	api, rule := gatewayProxyTestAPI(t, upstream)

	for _, target := range []string{gatewayProxyPath + rule.ID + "/", gatewayProxyPath + "invalid/"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		res := httptest.NewRecorder()
		api.ServeHTTP(res, req)
		if res.Code != http.StatusForbidden {
			t.Fatalf("expected admin protection for %s, got %d", target, res.Code)
		}
	}
}

func TestGatewayProxyAcceptsIPv6ListenMode(t *testing.T) {
	rule := domain.ProxyRule{ID: "fc1f74c20c59cd9d", Enabled: true, ListenType: "ipv6"}
	if _, found := findGatewayProxyRule([]domain.ProxyRule{rule}, rule.ID); !found {
		t.Fatal("IPv6 listen mode must not be treated as an application protocol")
	}
}

func TestGatewayProxyRewritesRedirectAndCookiePath(t *testing.T) {
	var upstreamURL string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", upstreamURL+"/login?next=%2F")
		w.Header().Add("Set-Cookie", "session=abc; Path=/; HttpOnly; SameSite=Lax")
		w.WriteHeader(http.StatusFound)
	}))
	defer upstream.Close()
	upstreamURL = upstream.URL
	api, rule := gatewayProxyTestAPI(t, upstream)
	routePrefix := gatewayProxyPath + rule.ID

	req := httptest.NewRequest(http.MethodGet, routePrefix+"/", nil)
	req.Header.Set("X-Trim-Isadmin", "true")
	res := httptest.NewRecorder()
	api.ServeHTTP(res, req)
	if got := res.Header().Get("Location"); got != routePrefix+"/login?next=%2F" {
		t.Fatalf("unexpected rewritten location: %q", got)
	}
	if got := res.Header().Get("Set-Cookie"); !strings.Contains(got, "Path="+routePrefix+"/") {
		t.Fatalf("cookie path was not rewritten: %q", got)
	}
}

func TestGatewayProxyRewritesHTMLRootPaths(t *testing.T) {
	var upstreamURL string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head>
<link rel="stylesheet" href="/assets/app.css"><style>.logo{background:url('/assets/bg.png')}</style>
</head><body><a href="/user/login">login</a><img src="/assets/logo.svg" srcset="/assets/one.png 1x, https://cdn.example/two.png 2x">
<form action="%s/session"></form><a href="https://docs.example/help">docs</a></body></html>`, upstreamURL)
	}))
	defer upstream.Close()
	upstreamURL = upstream.URL
	api, rule := gatewayProxyTestAPI(t, upstream)
	routePrefix := gatewayProxyPath + rule.ID

	req := httptest.NewRequest(http.MethodGet, routePrefix+"/", nil)
	req.Header.Set("X-Trim-Isadmin", "true")
	res := httptest.NewRecorder()
	api.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", res.Code)
	}
	body := res.Body.String()
	for _, expected := range []string{
		`href="` + routePrefix + `/assets/app.css"`,
		`href="` + routePrefix + `/user/login"`,
		`src="` + routePrefix + `/assets/logo.svg"`,
		`srcset="` + routePrefix + `/assets/one.png 1x, https://cdn.example/two.png 2x"`,
		`action="` + routePrefix + `/session"`,
		`url('` + routePrefix + `/assets/bg.png')`,
		`href="https://docs.example/help"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("rewritten HTML is missing %q:\n%s", expected, body)
		}
	}
}

func TestGatewayProxyForwardsWebSocketUpgrade(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close()
		messageType, payload, err := connection.ReadMessage()
		if err != nil {
			t.Error(err)
			return
		}
		if err := connection.WriteMessage(messageType, append([]byte(r.URL.Path+":"), payload...)); err != nil {
			t.Error(err)
		}
	}))
	defer upstream.Close()
	api, rule := gatewayProxyTestAPI(t, upstream)
	gateway := httptest.NewServer(api)
	defer gateway.Close()

	websocketURL := "ws" + strings.TrimPrefix(gateway.URL, "http") + gatewayProxyPath + rule.ID + "/socket"
	header := http.Header{"X-Trim-Isadmin": []string{"true"}}
	connection, response, err := websocket.DefaultDialer.Dial(websocketURL, header)
	if err != nil {
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		t.Fatalf("websocket dial failed with status %d: %v", status, err)
	}
	defer connection.Close()
	if err := connection.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	_, payload, err := connection.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "/socket:hello" {
		t.Fatalf("unexpected websocket payload: %q", payload)
	}
}
