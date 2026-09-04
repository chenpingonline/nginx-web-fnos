package nginx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenpingonline/fn-nginx-web/internal/domain"
)

func TestRenderUsesOnlyFnProxyPaths(t *testing.T) {
	root := t.TempDir()
	paths := Paths{
		AppDest:        filepath.Join(root, "app"),
		EtcDir:         filepath.Join(root, "etc"),
		VarDir:         filepath.Join(root, "var"),
		TmpDir:         filepath.Join(root, "tmp"),
		NginxBin:       filepath.Join(root, "app", "bin", "nginx"),
		MimeTypes:      filepath.Join(root, "app", "etc", "mime.types"),
		StateFile:      filepath.Join(root, "var", "fnproxy.json"),
		CertificateDir: filepath.Join(root, "var", "certificates"),
		RevisionDir:    filepath.Join(root, "etc", "nginx", "revisions"),
		NginxPrefix:    filepath.Join(root, "var", "nginx"),
		NginxConfigDir: filepath.Join(root, "etc", "nginx"),
		NginxConfD:     filepath.Join(root, "etc", "nginx", "conf.d"),
		NginxMaster:    filepath.Join(root, "etc", "nginx", "nginx.conf"),
		NginxRunDir:    filepath.Join(root, "var", "nginx", "run"),
		NginxPID:       filepath.Join(root, "var", "nginx", "run", "nginx.pid"),
		NginxLogDir:    filepath.Join(root, "var", "logs"),
		NginxAccessLog: filepath.Join(root, "var", "logs", "nginx-access.log"),
		NginxErrorLog:  filepath.Join(root, "var", "logs", "nginx-error.log"),
		NginxTempDir:   filepath.Join(root, "tmp", "nginx"),
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.MimeTypes), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.MimeTypes, []byte("types { text/plain txt; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := domain.DefaultState()
	state.Settings.DefaultHTTPPort = 19080
	rule := domain.ProxyRule{
		ID: "0123456789ab", Name: "Demo", Enabled: true, ListenPort: 19080,
		Domains: []string{"proxy.example.com"}, UpstreamScheme: "http",
		UpstreamHost: "127.0.0.1", UpstreamPort: 8080, PreserveHost: true, WebSocket: true,
		ConnectTimeoutSeconds: 10, ReadTimeoutSeconds: 60, SendTimeoutSeconds: 60,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	rule.UpstreamHost = "::1"
	state.Rules = []domain.ProxyRule{rule}

	manager := New(paths)
	master, files, err := manager.render(state, paths.NginxConfD)
	if err != nil {
		t.Fatal(err)
	}
	all := master
	for _, content := range files {
		all += content
	}
	for _, forbidden := range []string{"include \"/etc/nginx", "/usr/trim/nginx", "pid \"/var/run/nginx.pid"} {
		if strings.Contains(all, forbidden) {
			t.Fatalf("generated config contains system path %s", forbidden)
		}
	}
	for _, expected := range []string{paths.NginxPID, paths.NginxErrorLog, "listen 19080", "proxy_pass http://[::1]:8080", "proxy_set_header Upgrade"} {
		if !strings.Contains(all, expected) {
			t.Fatalf("generated config missing %q:\n%s", expected, all)
		}
	}
}

func TestLastNginxErrorIgnoresNotice(t *testing.T) {
	lines := []string{
		"2026/01/01 [notice] start worker process",
		"2026/01/01 [error] upstream timed out",
		"2026/01/01 [notice] signal process started",
	}
	if got := lastNginxError(lines); !strings.Contains(got, "upstream timed out") {
		t.Fatalf("unexpected error line: %q", got)
	}
}

func TestRenderAdvancedRuntimePoolAndRateLimit(t *testing.T) {
	root := t.TempDir()
	paths := Paths{AppDest: filepath.Join(root, "app"), EtcDir: filepath.Join(root, "etc"), VarDir: filepath.Join(root, "var"), TmpDir: filepath.Join(root, "tmp"), NginxBin: filepath.Join(root, "app/bin/nginx"), MimeTypes: filepath.Join(root, "app/etc/mime.types"), CertificateDir: filepath.Join(root, "var/certificates"), RevisionDir: filepath.Join(root, "etc/revisions"), NginxPrefix: filepath.Join(root, "var/nginx"), NginxConfigDir: filepath.Join(root, "etc/nginx"), NginxConfD: filepath.Join(root, "etc/nginx/conf.d"), NginxMaster: filepath.Join(root, "etc/nginx/nginx.conf"), NginxRunDir: filepath.Join(root, "var/nginx/run"), NginxPID: filepath.Join(root, "var/nginx/run/nginx.pid"), NginxLogDir: filepath.Join(root, "var/logs"), NginxAccessLog: filepath.Join(root, "var/logs/access.log"), NginxErrorLog: filepath.Join(root, "var/logs/error.log"), NginxStreamLog: filepath.Join(root, "var/logs/stream.log"), NginxTempDir: filepath.Join(root, "tmp/nginx")}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.MimeTypes), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.MimeTypes, []byte("types {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := domain.DefaultState()
	state.Settings.WorkerProcesses = 2
	state.Settings.WorkerConnections = 2048
	state.Settings.WorkerRlimitNofile = 4096
	state.Settings.ThreadPoolThreads = 4
	state.Settings.ThreadPoolQueue = 1024
	state.Settings.Logging.CustomFormat = `$remote_addr "$request" $status`
	state.Settings.Routing.Maps = []domain.MapDefinition{{Name: "host route", Source: "$host", Variable: "$backend", Default: "stable", Entries: []domain.KeyValue{{Key: "canary.test", Value: "canary"}}}}
	state.Settings.Routing.Geos = []domain.GeoDefinition{{Name: "lan", Source: "$remote_addr", Variable: "$network", Default: "wan", Entries: []domain.KeyValue{{Key: "192.168.0.0/16", Value: "lan"}}}}
	state.Settings.Routing.Splits = []domain.SplitDefinition{{Name: "canary", Source: "$request_id", Variable: "$variant", Entries: []domain.KeyValue{{Key: "10%", Value: "canary"}, {Key: "*", Value: "stable"}}}}
	state.Settings.RealIP = domain.RealIPSettings{Enabled: true, Header: "X-Forwarded-For", TrustedProxies: []string{"10.0.0.0/8"}, Recursive: true}
	pool := domain.UpstreamPool{ID: "0123456789ab", Name: "web", Protocol: "http", Strategy: "least_conn", Keepalive: 32, Servers: []domain.UpstreamServer{{Host: "127.0.0.1", Port: 8080}}}
	domain.NormalizeUpstreamPool(&pool)
	state.UpstreamPools = []domain.UpstreamPool{pool}
	streamPool := domain.UpstreamPool{ID: "111111111111", Name: "mqtt", Protocol: "stream", Strategy: "least_conn", Servers: []domain.UpstreamServer{{Host: "10.0.0.2", Port: 1883}}}
	domain.NormalizeUpstreamPool(&streamPool)
	state.UpstreamPools = append(state.UpstreamPools, streamPool)
	state.RateLimitPolicies = []domain.RateLimitPolicy{{ID: "444444444444", Name: "公开接口", Settings: domain.RateLimitSettings{Enabled: true, RequestsPerSecond: 20, Burst: 40, NoDelay: true, Connections: 10, DownloadKBps: 1024}}}
	state.Rules = []domain.ProxyRule{{ID: "abcdef012345", Name: "demo", Enabled: true, ListenPort: 19080, Domains: []string{"demo.test"}, UpstreamScheme: "http", UpstreamHost: "127.0.0.1", UpstreamPort: 8080, UpstreamPoolID: pool.ID, RateLimitPolicyID: "444444444444", ConnectTimeoutSeconds: 10, ReadTimeoutSeconds: 60, SendTimeoutSeconds: 60}}
	domain.NormalizeRule(&state.Rules[0], state.Settings)
	secondRule := state.Rules[0]
	secondRule.ID = "abcdef012346"
	secondRule.Name = "demo two"
	secondRule.ListenPort = 19082
	secondRule.Domains = []string{"demo-two.test"}
	state.Rules = append(state.Rules, secondRule)
	state.Rules[0].RootLocation.Cache.Enabled = true
	state.Rules[0].RootLocation.Cache.SliceKB = 1024
	state.Rules[0].RootLocation.Rewrites = []domain.RewriteRule{{Pattern: "^/old/(.*)$", Replacement: "/new/$1", Flag: "permanent"}}
	state.Rules[0].RootLocation.Allow = []string{"192.168.0.0/16"}
	state.Rules[0].RootLocation.RequestHeaders = []domain.HeaderSetting{{Name: "X-App", Value: "nginx-web"}}
	state.Rules[0].RootLocation.ResponseHeaders = []domain.HeaderSetting{{Name: "X-Frame-Options", Value: "DENY", Always: true}}
	state.Rules[0].RootLocation.AuthRequest = "/_auth"
	state.Rules[0].RootLocation.SecureLink = domain.SecureLinkSettings{Enabled: true, Secret: "test-secret", Argument: "md5"}
	state.Rules[0].RootLocation.SubFilters = []domain.SubFilterSetting{{Search: "old", Replacement: "new"}}
	staticSettings := state.Rules[0].RootLocation
	staticSettings.BackendType = "static"
	staticSettings.StaticPath = "/vol1/data/www"
	staticSettings.Cache.Enabled = false
	staticSettings.Rewrites = nil
	staticSettings.SecureLink.Enabled = false
	staticSettings.SubFilters = nil
	state.Rules[0].Locations = []domain.LocationRule{{ID: "333333333333", Name: "assets", Enabled: true, Path: "/assets/", Match: "prefix", Settings: staticSettings}}
	state.StreamRules = []domain.StreamRule{{ID: "222222222222", Name: "mqtt tls", Enabled: true, Protocol: "tcp", ListenAddress: "0.0.0.0", ListenPort: 19081, UpstreamPoolID: streamPool.ID, ConnectTimeoutSeconds: 10, ProxyTimeoutSeconds: 3600, TLSMode: "passthrough", AccessLog: true, MaxConnections: 20, SNIRoutes: []domain.SNIRoute{{ServerNames: []string{"mqtt.example.com"}, UpstreamPoolID: streamPool.ID}}}}
	master, files, err := New(paths).render(state, paths.NginxConfD)
	if err != nil {
		t.Fatal(err)
	}
	all := master
	for _, content := range files {
		all += content
	}
	if !strings.Contains(all, "zone=fnproxy_req_abcdef012345:1m rate=20r/s") {
		t.Fatalf("expected policy values in a rule-scoped request zone:\n%s", all)
	}
	if !strings.Contains(all, "zone=fnproxy_req_abcdef012346:1m rate=20r/s") {
		t.Fatalf("expected reused policy to create an independent request zone:\n%s", all)
	}
	for _, expected := range []string{"worker_processes 2", "worker_rlimit_nofile 4096", "worker_connections 2048", "thread_pool fnproxy threads=4 max_queue=1024", "aio threads=fnproxy", `log_format fnproxy "$remote_addr \"$request\" $status"`, "map $host $backend", "geo $remote_addr $network", `split_clients "$request_id" $variant`, "set_real_ip_from 10.0.0.0/8", "gzip on", "upstream fnproxy_up_0123456789ab", "least_conn", "keepalive 32", "limit_req_zone", "limit_conn_zone", "proxy_pass http://fnproxy_up_0123456789ab", "limit_rate 1024k", "proxy_cache_path", "proxy_cache fnproxy_cache_abcdef012345", "slice 1024k", `rewrite "^/old/(.*)$" "/new/$1" permanent`, "allow 192.168.0.0/16", "auth_request \"/_auth\"", "secure_link_md5", `proxy_set_header X-App "nginx-web"`, `add_header X-Frame-Options "DENY" always`, `sub_filter "old" "new"`, `location "/assets/"`, `root "/vol1/data/www"`, "stream {", "upstream fnproxy_stream_111111111111", "map $ssl_preread_server_name", "listen 0.0.0.0:19081", "ssl_preread on", "limit_conn fnproxy_stream_conn_222222222222 20", "stream.log"} {
		if !strings.Contains(all, expected) {
			t.Fatalf("missing %q:\n%s", expected, all)
		}
	}
}

func TestAdvancedConfigWithRealNginx(t *testing.T) {
	nginxBin := os.Getenv("NGINX_TEST_BIN")
	if nginxBin == "" {
		t.Skip("set NGINX_TEST_BIN to run nginx -t against the generated advanced configuration")
	}
	root := t.TempDir()
	paths := Paths{
		AppDest: filepath.Join(root, "app"), EtcDir: filepath.Join(root, "etc"), VarDir: filepath.Join(root, "var"), TmpDir: filepath.Join(root, "tmp"),
		NginxBin: nginxBin, MimeTypes: filepath.Join(root, "app/etc/mime.types"), StateFile: filepath.Join(root, "var/state.json"),
		CertificateDir: filepath.Join(root, "var/certificates"), RevisionDir: filepath.Join(root, "etc/revisions"), NginxPrefix: filepath.Join(root, "var/nginx"),
		NginxConfigDir: filepath.Join(root, "etc/nginx"), NginxConfD: filepath.Join(root, "etc/nginx/conf.d"), NginxMaster: filepath.Join(root, "etc/nginx/nginx.conf"),
		NginxRunDir: filepath.Join(root, "var/nginx/run"), NginxPID: filepath.Join(root, "var/nginx/run/nginx.pid"), NginxLogDir: filepath.Join(root, "var/logs"),
		NginxAccessLog: filepath.Join(root, "var/logs/access.log"), NginxErrorLog: filepath.Join(root, "var/logs/error.log"), NginxStreamLog: filepath.Join(root, "var/logs/stream.log"),
		NginxTempDir: filepath.Join(root, "tmp/nginx"), NginxCacheDir: filepath.Join(root, "var/cache"),
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.MimeTypes), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.MimeTypes, []byte("types { text/html html; application/json json; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	htpasswd := filepath.Join(root, "users.htpasswd")
	if err := os.WriteFile(htpasswd, []byte("admin:{SHA}qUqP5cyxm6YcTAhz05Hph5gvu9M=\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state := domain.DefaultState()
	state.Settings.ThreadPoolThreads = 2
	state.Settings.Routing.Maps = []domain.MapDefinition{{Name: "host", Source: "$host", Variable: "$route", Default: "stable", Entries: []domain.KeyValue{{Key: "canary.test", Value: "canary"}}}}
	state.Settings.Routing.Geos = []domain.GeoDefinition{{Name: "network", Source: "$remote_addr", Variable: "$network", Default: "wan", Entries: []domain.KeyValue{{Key: "192.168.0.0/16", Value: "lan"}}}}
	state.Settings.Routing.Splits = []domain.SplitDefinition{{Name: "canary", Source: "$request_id", Variable: "$variant", Entries: []domain.KeyValue{{Key: "10%", Value: "canary"}, {Key: "*", Value: "stable"}}}}
	rule := domain.ProxyRule{ID: "abcdef012345", Name: "advanced", Enabled: true, ListenPort: 19080, Domains: []string{"advanced.test"}, UpstreamScheme: "http", UpstreamHost: "127.0.0.1", UpstreamPort: 8080, PreserveHost: true, ConnectTimeoutSeconds: 10, ReadTimeoutSeconds: 60, SendTimeoutSeconds: 60}
	domain.NormalizeRule(&rule, state.Settings)
	rule.RootLocation.Cache.Enabled = true
	rule.RootLocation.Cache.SliceKB = 1024
	rule.RootLocation.BasicAuth = true
	rule.RootLocation.BasicAuthFile = htpasswd
	rule.RootLocation.AuthRequest = "/_auth"
	rule.RootLocation.SecureLink = domain.SecureLinkSettings{Enabled: true, Secret: "secret", Argument: "md5"}
	rule.RootLocation.SubFilters = []domain.SubFilterSetting{{Search: "old", Replacement: "new"}}
	rule.RootLocation.RequestHeaders = []domain.HeaderSetting{{Name: "X-App", Value: "$route"}}
	rule.RootLocation.ResponseHeaders = []domain.HeaderSetting{{Name: "X-Test", Value: "yes", Always: true}}
	rule.RootLocation.AdditionBefore = "/_before"
	rule.RootLocation.AdditionAfter = "/_after"
	rule.RootLocation.Mirror = "/_mirror"
	rule.RootLocation.SSI = true
	rule.RootLocation.ValidReferers = []string{"none", "blocked", "server_names"}
	staticSettings := rule.RootLocation
	staticSettings.BackendType = "static"
	staticSettings.StaticPath = filepath.Join(root, "www")
	staticSettings.Cache.Enabled = false
	staticSettings.BasicAuth = false
	staticSettings.SecureLink.Enabled = false
	staticSettings.DAV = domain.DAVSettings{Enabled: true, Methods: []string{"PUT", "DELETE"}, CreateFullPutPath: true, MinDeleteDepth: 1}
	rule.Locations = []domain.LocationRule{{ID: "333333333333", Name: "dav", Enabled: true, Path: "/dav/", Match: "prefix", Settings: staticSettings}}
	state.Rules = []domain.ProxyRule{rule}
	if _, err := New(paths).Prepare(state); err != nil {
		t.Fatal(err)
	}
}
