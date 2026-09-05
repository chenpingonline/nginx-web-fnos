package domain

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

type RealIPSettings struct {
	Enabled        bool     `json:"enabled"`
	Header         string   `json:"header"`
	TrustedProxies []string `json:"trusted_proxies"`
	Recursive      bool     `json:"recursive"`
}

type GzipSettings struct {
	Enabled   bool     `json:"enabled"`
	Level     int      `json:"level"`
	MinLength int      `json:"min_length"`
	Types     []string `json:"types"`
	Static    bool     `json:"static"`
	Gunzip    bool     `json:"gunzip"`
}

type TLSSettings struct {
	Protocols             []string `json:"protocols"`
	Ciphers               string   `json:"ciphers"`
	SessionCacheMB        int      `json:"session_cache_mb"`
	SessionTimeoutMinutes int      `json:"session_timeout_minutes"`
	OCSPStapling          bool     `json:"ocsp_stapling"`
	ClientVerify          string   `json:"client_verify"`
	ClientCAFile          string   `json:"client_ca_file,omitempty"`
	ClientVerifyDepth     int      `json:"client_verify_depth"`
}

type LoggingSettings struct {
	AccessEnabled  bool   `json:"access_enabled"`
	ErrorLevel     string `json:"error_level"`
	AccessBufferKB int    `json:"access_buffer_kb"`
	AccessFlushSec int    `json:"access_flush_seconds"`
	CustomFormat   string `json:"custom_format"`
	RotateSizeMB   int    `json:"rotate_size_mb"`
	RotateKeep     int    `json:"rotate_keep"`
}

type UpstreamServer struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Weight      int    `json:"weight"`
	MaxFails    int    `json:"max_fails"`
	FailTimeout int    `json:"fail_timeout_seconds"`
	Backup      bool   `json:"backup"`
	Down        bool   `json:"down"`
}

type UpstreamPool struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	Protocol          string           `json:"protocol"`
	Strategy          string           `json:"strategy"`
	HashKey           string           `json:"hash_key,omitempty"`
	Keepalive         int              `json:"keepalive"`
	KeepaliveRequests int              `json:"keepalive_requests"`
	KeepaliveTime     int              `json:"keepalive_time_seconds"`
	KeepaliveTimeout  int              `json:"keepalive_timeout_seconds"`
	Servers           []UpstreamServer `json:"servers"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

type RateLimitSettings struct {
	Enabled           bool `json:"enabled"`
	RequestsPerSecond int  `json:"requests_per_second"`
	Burst             int  `json:"burst"`
	NoDelay           bool `json:"no_delay"`
	Connections       int  `json:"connections"`
	DownloadKBps      int  `json:"download_kbps"`
}

type RateLimitPolicy struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Settings  RateLimitSettings `json:"settings"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type MapDefinition struct {
	Name      string     `json:"name"`
	Source    string     `json:"source"`
	Variable  string     `json:"variable"`
	Hostnames bool       `json:"hostnames"`
	Default   string     `json:"default"`
	Entries   []KeyValue `json:"entries"`
}

type GeoDefinition struct {
	Name     string     `json:"name"`
	Source   string     `json:"source"`
	Variable string     `json:"variable"`
	Default  string     `json:"default"`
	Entries  []KeyValue `json:"entries"`
}

type SplitDefinition struct {
	Name     string     `json:"name"`
	Source   string     `json:"source"`
	Variable string     `json:"variable"`
	Entries  []KeyValue `json:"entries"`
}

type DynamicRoutingSettings struct {
	Maps   []MapDefinition   `json:"maps"`
	Geos   []GeoDefinition   `json:"geos"`
	Splits []SplitDefinition `json:"splits"`
}

type SNIRoute struct {
	ServerNames    []string `json:"server_names"`
	UpstreamPoolID string   `json:"upstream_pool_id,omitempty"`
	UpstreamHost   string   `json:"upstream_host,omitempty"`
	UpstreamPort   int      `json:"upstream_port,omitempty"`
}

type StreamRule struct {
	ID                    string     `json:"id"`
	Name                  string     `json:"name"`
	Enabled               bool       `json:"enabled"`
	Protocol              string     `json:"protocol"`
	ListenAddress         string     `json:"listen_address"`
	ListenPort            int        `json:"listen_port"`
	UpstreamPoolID        string     `json:"upstream_pool_id,omitempty"`
	UpstreamHost          string     `json:"upstream_host,omitempty"`
	UpstreamPort          int        `json:"upstream_port,omitempty"`
	ConnectTimeoutSeconds int        `json:"connect_timeout_seconds"`
	ProxyTimeoutSeconds   int        `json:"proxy_timeout_seconds"`
	UDPResponses          int        `json:"udp_responses"`
	ProxyProtocol         bool       `json:"proxy_protocol"`
	AcceptProxyProtocol   bool       `json:"accept_proxy_protocol"`
	TrustedProxies        []string   `json:"trusted_proxies"`
	TLSMode               string     `json:"tls_mode"`
	CertificateID         string     `json:"certificate_id,omitempty"`
	SNIRoutes             []SNIRoute `json:"sni_routes"`
	AccessLog             bool       `json:"access_log"`
	Allow                 []string   `json:"allow"`
	Deny                  []string   `json:"deny"`
	MaxConnections        int        `json:"max_connections"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type CacheSettings struct {
	Enabled         bool     `json:"enabled"`
	KeysZoneMB      int      `json:"keys_zone_mb"`
	MaxSizeMB       int      `json:"max_size_mb"`
	InactiveMinutes int      `json:"inactive_minutes"`
	ValidSeconds    int      `json:"valid_seconds"`
	SliceKB         int      `json:"slice_kb"`
	UseStale        bool     `json:"use_stale"`
	Key             string   `json:"key"`
	Bypass          []string `json:"bypass"`
}

type RewriteRule struct {
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
	Flag        string `json:"flag"`
}

type HeaderSetting struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Always bool   `json:"always"`
}

type SubFilterSetting struct {
	Search      string `json:"search"`
	Replacement string `json:"replacement"`
}

type SecureLinkSettings struct {
	Enabled  bool   `json:"enabled"`
	Secret   string `json:"secret,omitempty"`
	Argument string `json:"argument"`
}

type DAVSettings struct {
	Enabled           bool     `json:"enabled"`
	Methods           []string `json:"methods"`
	CreateFullPutPath bool     `json:"create_full_put_path"`
	MinDeleteDepth    int      `json:"min_delete_depth"`
}

type LocationSettings struct {
	BackendType        string             `json:"backend_type"`
	UpstreamScheme     string             `json:"upstream_scheme"`
	UpstreamPoolID     string             `json:"upstream_pool_id,omitempty"`
	UpstreamHost       string             `json:"upstream_host,omitempty"`
	UpstreamPort       int                `json:"upstream_port,omitempty"`
	StaticPath         string             `json:"static_path,omitempty"`
	StaticAlias        bool               `json:"static_alias"`
	IndexFiles         []string           `json:"index_files"`
	AutoIndex          bool               `json:"autoindex"`
	Expires            string             `json:"expires,omitempty"`
	TryFiles           []string           `json:"try_files"`
	ReturnCode         int                `json:"return_code"`
	ReturnTarget       string             `json:"return_target,omitempty"`
	RedirectToHTTPS    bool               `json:"redirect_to_https"`
	Rewrites           []RewriteRule      `json:"rewrites"`
	Cache              CacheSettings      `json:"cache"`
	Allow              []string           `json:"allow"`
	Deny               []string           `json:"deny"`
	RequestHeaders     []HeaderSetting    `json:"request_headers"`
	ResponseHeaders    []HeaderSetting    `json:"response_headers"`
	BasicAuth          bool               `json:"basic_auth"`
	BasicAuthRealm     string             `json:"basic_auth_realm"`
	BasicAuthFile      string             `json:"basic_auth_file,omitempty"`
	AuthRequest        string             `json:"auth_request,omitempty"`
	SecureLink         SecureLinkSettings `json:"secure_link"`
	DAV                DAVSettings        `json:"dav"`
	SubFilters         []SubFilterSetting `json:"sub_filters"`
	SubFilterOnce      bool               `json:"sub_filter_once"`
	SubFilterTypes     []string           `json:"sub_filter_types"`
	AdditionBefore     string             `json:"addition_before,omitempty"`
	AdditionAfter      string             `json:"addition_after,omitempty"`
	Mirror             string             `json:"mirror,omitempty"`
	MirrorRequestBody  bool               `json:"mirror_request_body"`
	SSI                bool               `json:"ssi"`
	ValidReferers      []string           `json:"valid_referers"`
	DenyInvalidReferer bool               `json:"deny_invalid_referer"`
}

type LocationRule struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Enabled  bool             `json:"enabled"`
	Path     string           `json:"path"`
	Match    string           `json:"match"`
	Settings LocationSettings `json:"settings"`
}

var variablePattern = regexp.MustCompile(`^\$[A-Za-z0-9_]+$`)

func defaultRealIPSettings() RealIPSettings {
	return RealIPSettings{Header: "X-Forwarded-For", TrustedProxies: []string{}}
}

func defaultGzipSettings() GzipSettings {
	return GzipSettings{Enabled: true, Level: 5, MinLength: 1024, Types: []string{"text/plain", "text/css", "application/json", "application/javascript", "application/xml", "image/svg+xml"}, Static: true, Gunzip: true}
}

func defaultTLSSettings() TLSSettings {
	return TLSSettings{Protocols: []string{"TLSv1.2", "TLSv1.3"}, SessionCacheMB: 10, SessionTimeoutMinutes: 10, ClientVerify: "off", ClientVerifyDepth: 1}
}

func defaultLoggingSettings() LoggingSettings {
	return LoggingSettings{AccessEnabled: true, ErrorLevel: "notice", AccessBufferKB: 32, AccessFlushSec: 5, RotateSizeMB: 50, RotateKeep: 5}
}

func ApplyStateDefaults(state *State) {
	defaults := DefaultState().Settings
	if state.Settings.DefaultHTTPPort == 0 {
		state.Settings.DefaultHTTPPort = defaults.DefaultHTTPPort
	}
	if state.Settings.DefaultHTTPSPort == 0 {
		state.Settings.DefaultHTTPSPort = defaults.DefaultHTTPSPort
	}
	if state.Settings.RevisionLimit == 0 {
		state.Settings.RevisionLimit = defaults.RevisionLimit
	}
	if state.Settings.WorkerConnections == 0 {
		state.Settings.WorkerConnections = defaults.WorkerConnections
	}
	if state.Settings.ThreadPoolQueue == 0 {
		state.Settings.ThreadPoolQueue = defaults.ThreadPoolQueue
	}
	if state.Settings.RealIP.Header == "" {
		state.Settings.RealIP = defaults.RealIP
	}
	if state.Settings.Gzip.Level == 0 {
		state.Settings.Gzip = defaults.Gzip
	}
	if len(state.Settings.TLS.Protocols) == 0 {
		state.Settings.TLS = defaults.TLS
	}
	if state.Settings.TLS.ClientVerify == "" {
		state.Settings.TLS.ClientVerify = "off"
	}
	if state.Settings.TLS.ClientVerifyDepth == 0 {
		state.Settings.TLS.ClientVerifyDepth = 1
	}
	if state.Settings.Logging.ErrorLevel == "" {
		state.Settings.Logging = defaults.Logging
	}
	if state.Settings.Logging.RotateSizeMB == 0 {
		state.Settings.Logging.RotateSizeMB = defaults.Logging.RotateSizeMB
	}
	if state.Settings.Logging.RotateKeep == 0 {
		state.Settings.Logging.RotateKeep = defaults.Logging.RotateKeep
	}
	if state.Settings.Routing.Maps == nil {
		state.Settings.Routing = DynamicRoutingSettings{Maps: []MapDefinition{}, Geos: []GeoDefinition{}, Splits: []SplitDefinition{}}
	}
	if state.Rules == nil {
		state.Rules = []ProxyRule{}
	}
	if state.RateLimitPolicies == nil {
		state.RateLimitPolicies = []RateLimitPolicy{}
	}
	policyNames := make(map[string]struct{}, len(state.RateLimitPolicies))
	for index := range state.RateLimitPolicies {
		NormalizeRateLimitPolicy(&state.RateLimitPolicies[index])
		policyNames[strings.ToLower(state.RateLimitPolicies[index].Name)] = struct{}{}
	}
	for index := range state.Rules {
		NormalizeRule(&state.Rules[index], state.Settings)
		rule := &state.Rules[index]
		if rule.RateLimitPolicyID == "" && rule.RateLimit.Enabled {
			name := strings.TrimSpace(rule.Name) + " 限流"
			if name == " 限流" {
				name = "迁移的限流策略"
			}
			base := name
			for suffix := 2; ; suffix++ {
				if _, exists := policyNames[strings.ToLower(name)]; !exists {
					break
				}
				name = fmt.Sprintf("%s %d", base, suffix)
			}
			createdAt := rule.CreatedAt
			if createdAt.IsZero() {
				createdAt = time.Now().UTC()
			}
			policy := RateLimitPolicy{
				ID:        RandomID(),
				Name:      name,
				Settings:  rule.RateLimit,
				CreatedAt: createdAt,
				UpdatedAt: rule.UpdatedAt,
			}
			if policy.UpdatedAt.IsZero() {
				policy.UpdatedAt = createdAt
			}
			NormalizeRateLimitPolicy(&policy)
			state.RateLimitPolicies = append(state.RateLimitPolicies, policy)
			policyNames[strings.ToLower(policy.Name)] = struct{}{}
			rule.RateLimitPolicyID = policy.ID
		}
	}
	if state.Certificates == nil {
		state.Certificates = []CertificateMeta{}
	}
	if state.UpstreamPools == nil {
		state.UpstreamPools = []UpstreamPool{}
	}
	if state.StreamRules == nil {
		state.StreamRules = []StreamRule{}
	}
	for index := range state.UpstreamPools {
		NormalizeUpstreamPool(&state.UpstreamPools[index])
	}
	for index := range state.StreamRules {
		NormalizeStreamRule(&state.StreamRules[index])
	}
}

func NormalizeRateLimitPolicy(policy *RateLimitPolicy) {
	policy.Name = strings.TrimSpace(policy.Name)
	policy.Settings.Enabled = true
}

func NormalizeStreamRule(rule *StreamRule) {
	rule.Name = strings.TrimSpace(rule.Name)
	rule.Protocol = strings.ToLower(strings.TrimSpace(rule.Protocol))
	rule.ListenAddress = strings.TrimSpace(strings.Trim(rule.ListenAddress, "[]"))
	rule.UpstreamPoolID = strings.TrimSpace(rule.UpstreamPoolID)
	rule.UpstreamHost = strings.TrimSpace(strings.Trim(rule.UpstreamHost, "[]"))
	rule.TLSMode = strings.ToLower(strings.TrimSpace(rule.TLSMode))
	rule.CertificateID = strings.TrimSpace(rule.CertificateID)
	if rule.Protocol == "" {
		rule.Protocol = "tcp"
	}
	if rule.ListenAddress == "" {
		rule.ListenAddress = "0.0.0.0"
	}
	if rule.ConnectTimeoutSeconds == 0 {
		rule.ConnectTimeoutSeconds = 10
	}
	if rule.ProxyTimeoutSeconds == 0 {
		rule.ProxyTimeoutSeconds = 3600
	}
	if rule.TLSMode == "" {
		rule.TLSMode = "off"
	}
	if rule.TrustedProxies == nil {
		rule.TrustedProxies = []string{}
	}
	if rule.Allow == nil {
		rule.Allow = []string{}
	}
	if rule.Deny == nil {
		rule.Deny = []string{}
	}
	if rule.SNIRoutes == nil {
		rule.SNIRoutes = []SNIRoute{}
	}
	for routeIndex := range rule.SNIRoutes {
		route := &rule.SNIRoutes[routeIndex]
		if route.ServerNames == nil {
			route.ServerNames = []string{}
		}
		route.UpstreamPoolID = strings.TrimSpace(route.UpstreamPoolID)
		route.UpstreamHost = strings.TrimSpace(strings.Trim(route.UpstreamHost, "[]"))
		for index := range route.ServerNames {
			route.ServerNames[index] = strings.ToLower(strings.TrimSpace(route.ServerNames[index]))
		}
	}
}

func defaultLocationSettings() LocationSettings {
	return LocationSettings{
		BackendType: "proxy", UpstreamScheme: "http",
		IndexFiles: []string{"index.html", "index.htm"}, TryFiles: []string{},
		Rewrites: []RewriteRule{}, Cache: CacheSettings{KeysZoneMB: 10, MaxSizeMB: 1024, InactiveMinutes: 60, ValidSeconds: 300, Key: "$scheme$request_method$host$request_uri", Bypass: []string{}},
		Allow: []string{}, Deny: []string{}, RequestHeaders: []HeaderSetting{}, ResponseHeaders: []HeaderSetting{},
		BasicAuthRealm: "Restricted", SecureLink: SecureLinkSettings{Argument: "md5"},
		DAV:        DAVSettings{Methods: []string{"PUT", "DELETE", "MKCOL", "COPY", "MOVE"}},
		SubFilters: []SubFilterSetting{}, SubFilterTypes: []string{"text/html"}, ValidReferers: []string{},
	}
}

func NormalizeLocationSettings(settings *LocationSettings) {
	settings.BackendType = strings.ToLower(strings.TrimSpace(settings.BackendType))
	settings.UpstreamScheme = strings.ToLower(strings.TrimSpace(settings.UpstreamScheme))
	settings.UpstreamPoolID = strings.TrimSpace(settings.UpstreamPoolID)
	settings.UpstreamHost = strings.TrimSpace(strings.Trim(settings.UpstreamHost, "[]"))
	settings.StaticPath = strings.TrimSpace(settings.StaticPath)
	settings.Expires = strings.TrimSpace(settings.Expires)
	settings.ReturnTarget = strings.TrimSpace(settings.ReturnTarget)
	if settings.BackendType == "" {
		settings.BackendType = "proxy"
	}
	if settings.UpstreamScheme == "" {
		settings.UpstreamScheme = "http"
	}
	if settings.IndexFiles == nil {
		settings.IndexFiles = []string{"index.html", "index.htm"}
	}
	if settings.TryFiles == nil {
		settings.TryFiles = []string{}
	}
	if settings.Rewrites == nil {
		settings.Rewrites = []RewriteRule{}
	}
	if settings.Allow == nil {
		settings.Allow = []string{}
	}
	if settings.Deny == nil {
		settings.Deny = []string{}
	}
	if settings.RequestHeaders == nil {
		settings.RequestHeaders = []HeaderSetting{}
	}
	if settings.ResponseHeaders == nil {
		settings.ResponseHeaders = []HeaderSetting{}
	}
	if settings.SubFilters == nil {
		settings.SubFilters = []SubFilterSetting{}
	}
	if settings.SubFilterTypes == nil {
		settings.SubFilterTypes = []string{"text/html"}
	}
	if settings.ValidReferers == nil {
		settings.ValidReferers = []string{}
	}
	if settings.BasicAuthRealm == "" {
		settings.BasicAuthRealm = "Restricted"
	}
	if settings.SecureLink.Argument == "" {
		settings.SecureLink.Argument = "md5"
	}
	if settings.DAV.Methods == nil {
		settings.DAV.Methods = []string{"PUT", "DELETE", "MKCOL", "COPY", "MOVE"}
	}
	if settings.Cache.KeysZoneMB == 0 {
		settings.Cache.KeysZoneMB = 10
	}
	if settings.Cache.MaxSizeMB == 0 {
		settings.Cache.MaxSizeMB = 1024
	}
	if settings.Cache.InactiveMinutes == 0 {
		settings.Cache.InactiveMinutes = 60
	}
	if settings.Cache.ValidSeconds == 0 {
		settings.Cache.ValidSeconds = 300
	}
	if settings.Cache.Key == "" {
		settings.Cache.Key = "$scheme$request_method$host$request_uri"
	}
	if settings.Cache.Bypass == nil {
		settings.Cache.Bypass = []string{}
	}
	for index := range settings.Rewrites {
		settings.Rewrites[index].Pattern = strings.TrimSpace(settings.Rewrites[index].Pattern)
		settings.Rewrites[index].Replacement = strings.TrimSpace(settings.Rewrites[index].Replacement)
		settings.Rewrites[index].Flag = strings.ToLower(strings.TrimSpace(settings.Rewrites[index].Flag))
		if settings.Rewrites[index].Flag == "" {
			settings.Rewrites[index].Flag = "last"
		}
	}
}

func NormalizeHTTPLocations(rule *ProxyRule) {
	NormalizeLocationSettings(&rule.RootLocation)
	for index := range rule.Locations {
		location := &rule.Locations[index]
		location.Name = strings.TrimSpace(location.Name)
		location.Path = strings.TrimSpace(location.Path)
		location.Match = strings.ToLower(strings.TrimSpace(location.Match))
		if location.ID == "" {
			location.ID = RandomID()
		}
		if location.Match == "" {
			location.Match = "prefix"
		}
		NormalizeLocationSettings(&location.Settings)
	}
}

func ValidateLocationSettings(settings LocationSettings, pools map[string]UpstreamPool) error {
	allowed := map[string]bool{"proxy": true, "static": true, "return": true, "grpc": true, "fastcgi": true, "uwsgi": true, "scgi": true, "memcached": true, "status": true}
	if !allowed[settings.BackendType] {
		return errors.New("Location 后端类型不支持")
	}
	if settings.BackendType == "proxy" || settings.BackendType == "grpc" || settings.BackendType == "fastcgi" || settings.BackendType == "uwsgi" || settings.BackendType == "scgi" || settings.BackendType == "memcached" {
		if settings.UpstreamPoolID != "" {
			pool, ok := pools[settings.UpstreamPoolID]
			if !ok || pool.Protocol != "http" {
				return errors.New("Location 引用的 HTTP 后端服务组不存在")
			}
		} else {
			if err := validateHostName(settings.UpstreamHost, false); err != nil {
				return err
			}
			if settings.UpstreamPort < 1 || settings.UpstreamPort > 65535 {
				return errors.New("Location 目标端口不合法")
			}
		}
	}
	if settings.UpstreamScheme != "http" && settings.UpstreamScheme != "https" {
		return errors.New("Location 后端服务协议不支持")
	}
	if settings.BackendType == "static" {
		if !strings.HasPrefix(settings.StaticPath, "/") || strings.Contains(settings.StaticPath, "..") || strings.ContainsAny(settings.StaticPath, "\x00\r\n") {
			return errors.New("静态文件目录必须是安全的绝对路径")
		}
		for _, name := range settings.IndexFiles {
			if !safeToken(name) {
				return errors.New("Index 文件名包含不允许的字符")
			}
		}
		for _, item := range settings.TryFiles {
			if strings.ContainsAny(item, "{};\r\n") {
				return errors.New("Try Files 包含不允许的字符")
			}
		}
	}
	if settings.BackendType == "return" {
		if settings.ReturnCode < 200 || settings.ReturnCode > 599 {
			return errors.New("返回状态码必须为 200 到 599")
		}
		if strings.ContainsAny(settings.ReturnTarget, "{};\r\n") {
			return errors.New("返回目标包含不允许的字符")
		}
	}
	if settings.Expires != "" && !regexp.MustCompile(`^(off|max|epoch|[+-]?[0-9]+[smhdwMy])$`).MatchString(settings.Expires) {
		return errors.New("Expires 格式不正确")
	}
	for _, rewrite := range settings.Rewrites {
		if rewrite.Pattern == "" || strings.ContainsAny(rewrite.Pattern, "{};\r\n") || strings.ContainsAny(rewrite.Replacement, "{};\r\n") {
			return errors.New("Rewrite 包含不允许的字符")
		}
		if rewrite.Flag != "last" && rewrite.Flag != "break" && rewrite.Flag != "redirect" && rewrite.Flag != "permanent" {
			return errors.New("Rewrite 标志不支持")
		}
	}
	if settings.Cache.Enabled {
		cache := settings.Cache
		if cache.KeysZoneMB < 1 || cache.KeysZoneMB > 1024 || cache.MaxSizeMB < 1 || cache.MaxSizeMB > 1048576 || cache.InactiveMinutes < 1 || cache.InactiveMinutes > 525600 || cache.ValidSeconds < 1 || cache.ValidSeconds > 31536000 || cache.SliceKB < 0 || cache.SliceKB > 1048576 {
			return errors.New("缓存参数超出允许范围")
		}
		if strings.ContainsAny(cache.Key, "{};\r\n") {
			return errors.New("缓存 Key 包含不允许的字符")
		}
		for _, value := range cache.Bypass {
			if !variablePattern.MatchString(value) {
				return errors.New("缓存绕过条件必须是安全的 Nginx 变量")
			}
		}
	}
	if err := validateCIDRs(append(append([]string{}, settings.Allow...), settings.Deny...)); err != nil {
		return err
	}
	for _, header := range append(append([]HeaderSetting{}, settings.RequestHeaders...), settings.ResponseHeaders...) {
		if !regexp.MustCompile(`^[A-Za-z0-9-]+$`).MatchString(header.Name) || strings.ContainsAny(header.Value, "{};\r\n") {
			return errors.New("自定义 Header 包含不允许的字符")
		}
	}
	if settings.BasicAuth {
		if !strings.HasPrefix(settings.BasicAuthFile, "/") || strings.ContainsAny(settings.BasicAuthFile, "\x00\r\n") {
			return errors.New("Basic Auth 密码文件必须是绝对路径")
		}
	}
	for _, uri := range []string{settings.AuthRequest, settings.AdditionBefore, settings.AdditionAfter, settings.Mirror} {
		if uri != "" && (!strings.HasPrefix(uri, "/") || strings.ContainsAny(uri, "{};\r\n")) {
			return errors.New("高级功能 URI 必须是安全的站内路径")
		}
	}
	if settings.SecureLink.Enabled {
		if settings.SecureLink.Secret == "" || strings.ContainsAny(settings.SecureLink.Secret, "{};\r\n") {
			return errors.New("Secure Link 密钥不合法")
		}
		if !safeToken(settings.SecureLink.Argument) {
			return errors.New("Secure Link 参数名不合法")
		}
	}
	if settings.DAV.Enabled {
		if settings.BackendType != "static" {
			return errors.New("WebDAV 只能用于静态文件 Location")
		}
		allowedDAV := map[string]bool{"PUT": true, "DELETE": true, "MKCOL": true, "COPY": true, "MOVE": true}
		if len(settings.DAV.Methods) == 0 {
			return errors.New("WebDAV 至少需要一个方法")
		}
		for _, method := range settings.DAV.Methods {
			if !allowedDAV[method] {
				return errors.New("WebDAV 方法不支持")
			}
		}
		if settings.DAV.MinDeleteDepth < 0 || settings.DAV.MinDeleteDepth > 100 {
			return errors.New("WebDAV 删除深度不合法")
		}
	}
	for _, filter := range settings.SubFilters {
		if filter.Search == "" || strings.ContainsAny(filter.Search, "\r\n") || strings.ContainsAny(filter.Replacement, "\r\n") {
			return errors.New("Sub Filter 内容不合法")
		}
	}
	for _, mime := range settings.SubFilterTypes {
		if strings.ContainsAny(mime, "{};\r\n\t ") {
			return errors.New("Sub Filter MIME 类型不合法")
		}
	}
	for _, referer := range settings.ValidReferers {
		if strings.ContainsAny(referer, "{};\r\n") {
			return errors.New("Referer 规则不合法")
		}
	}
	return nil
}

func ValidateHTTPLocations(rule ProxyRule, pools map[string]UpstreamPool) error {
	if err := ValidateLocationSettings(rule.RootLocation, pools); err != nil {
		return fmt.Errorf("根 Location: %w", err)
	}
	seen := map[string]struct{}{}
	for _, location := range rule.Locations {
		if !idPattern.MatchString(location.ID) {
			return errors.New("Location ID 格式不正确")
		}
		if len([]rune(location.Name)) < 1 || len([]rune(location.Name)) > 80 {
			return errors.New("Location 名称长度不正确")
		}
		if location.Match != "prefix" && location.Match != "exact" && location.Match != "regex" {
			return errors.New("Location 匹配类型不支持")
		}
		if !strings.HasPrefix(location.Path, "/") && location.Match != "regex" {
			return errors.New("Location 路径必须以 / 开头")
		}
		if strings.ContainsAny(location.Path, "{};\r\n") {
			return errors.New("Location 路径包含不允许的字符")
		}
		key := location.Match + "\x00" + location.Path
		if _, ok := seen[key]; ok {
			return errors.New("存在重复 Location")
		}
		seen[key] = struct{}{}
		if err := ValidateLocationSettings(location.Settings, pools); err != nil {
			return fmt.Errorf("Location %q: %w", location.Name, err)
		}
	}
	return nil
}

func safeToken(value string) bool { return value != "" && !strings.ContainsAny(value, "/\\{};\r\n\t ") }

func ValidateStreamRule(rule StreamRule, certs map[string]CertificateMeta, pools map[string]UpstreamPool) error {
	if !idPattern.MatchString(rule.ID) {
		return errors.New("Stream 规则 ID 格式不正确")
	}
	if len([]rune(rule.Name)) < 1 || len([]rune(rule.Name)) > 80 {
		return errors.New("Stream 规则名称长度必须为 1 到 80 个字符")
	}
	if rule.Protocol != "tcp" && rule.Protocol != "udp" {
		return errors.New("Stream 协议只能是 tcp 或 udp")
	}
	if rule.ListenPort < 1024 || rule.ListenPort > 65535 {
		return errors.New("Stream 监听端口必须为 1024 到 65535")
	}
	if rule.ListenAddress != "*" && net.ParseIP(rule.ListenAddress) == nil {
		return errors.New("Stream 监听地址必须是 IP 或 *")
	}
	if rule.ConnectTimeoutSeconds < 1 || rule.ConnectTimeoutSeconds > 600 || rule.ProxyTimeoutSeconds < 1 || rule.ProxyTimeoutSeconds > 86400 {
		return errors.New("Stream 超时参数超出允许范围")
	}
	if rule.UDPResponses < 0 || rule.UDPResponses > 1000 || rule.MaxConnections < 0 || rule.MaxConnections > 1000000 {
		return errors.New("Stream 响应或连接限制超出允许范围")
	}
	if err := validateCIDRs(rule.TrustedProxies); err != nil {
		return err
	}
	if err := validateCIDRs(rule.Allow); err != nil {
		return err
	}
	if err := validateCIDRs(rule.Deny); err != nil {
		return err
	}
	if rule.TLSMode != "off" && rule.TLSMode != "terminate" && rule.TLSMode != "passthrough" {
		return errors.New("Stream TLS 模式不支持")
	}
	if rule.Protocol == "udp" && rule.TLSMode != "off" {
		return errors.New("UDP 规则不能启用 TLS")
	}
	if rule.TLSMode == "terminate" {
		if _, ok := certs[rule.CertificateID]; !ok {
			return errors.New("Stream TLS 终止引用的证书不存在")
		}
	}
	if rule.TLSMode == "passthrough" && len(rule.SNIRoutes) > 0 {
		for _, route := range rule.SNIRoutes {
			if len(route.ServerNames) == 0 {
				return errors.New("SNI 路由至少需要一个域名")
			}
			for _, name := range route.ServerNames {
				if err := validateHostName(name, true); err != nil {
					return err
				}
			}
			if err := validateStreamTarget(route.UpstreamPoolID, route.UpstreamHost, route.UpstreamPort, pools); err != nil {
				return err
			}
		}
	} else if err := validateStreamTarget(rule.UpstreamPoolID, rule.UpstreamHost, rule.UpstreamPort, pools); err != nil {
		return err
	}
	return nil
}

func validateStreamTarget(poolID, host string, port int, pools map[string]UpstreamPool) error {
	if poolID != "" {
		pool, ok := pools[poolID]
		if !ok {
			return errors.New("引用的 Stream 后端服务组不存在")
		}
		if pool.Protocol != "stream" {
			return errors.New("Stream 规则只能引用 Stream 后端服务组")
		}
		return nil
	}
	if err := validateHostName(host, false); err != nil {
		return fmt.Errorf("Stream 目标主机不合法: %w", err)
	}
	if port < 1 || port > 65535 {
		return errors.New("Stream 目标端口必须为 1 到 65535")
	}
	return nil
}

func NormalizeUpstreamPool(pool *UpstreamPool) {
	pool.Name = strings.TrimSpace(pool.Name)
	pool.Protocol = strings.ToLower(strings.TrimSpace(pool.Protocol))
	pool.Strategy = strings.ToLower(strings.TrimSpace(pool.Strategy))
	pool.HashKey = strings.TrimSpace(pool.HashKey)
	if pool.Protocol == "" {
		pool.Protocol = "http"
	}
	if pool.Strategy == "" {
		pool.Strategy = "round_robin"
	}
	if pool.KeepaliveRequests == 0 {
		pool.KeepaliveRequests = 1000
	}
	if pool.KeepaliveTime == 0 {
		pool.KeepaliveTime = 3600
	}
	if pool.KeepaliveTimeout == 0 {
		pool.KeepaliveTimeout = 60
	}
	for index := range pool.Servers {
		server := &pool.Servers[index]
		server.Host = strings.TrimSpace(strings.Trim(server.Host, "[]"))
		if server.Weight == 0 {
			server.Weight = 1
		}
		if server.MaxFails == 0 {
			server.MaxFails = 1
		}
		if server.FailTimeout == 0 {
			server.FailTimeout = 10
		}
	}
}

func ValidateUpstreamPool(pool UpstreamPool) error {
	if !idPattern.MatchString(pool.ID) {
		return errors.New("后端服务组 ID 格式不正确")
	}
	if len([]rune(pool.Name)) < 1 || len([]rune(pool.Name)) > 80 {
		return errors.New("后端服务组名称长度必须为 1 到 80 个字符")
	}
	if pool.Protocol != "http" && pool.Protocol != "stream" {
		return errors.New("后端服务组协议只能是 http 或 stream")
	}
	allowedStrategy := map[string]bool{"round_robin": true, "least_conn": true, "ip_hash": true, "hash": true, "random": true}
	if !allowedStrategy[pool.Strategy] {
		return errors.New("后端服务组负载均衡算法不支持")
	}
	if pool.Protocol == "stream" && pool.Strategy == "ip_hash" {
		return errors.New("Stream 后端服务组不支持 IP Hash")
	}
	if pool.Strategy == "hash" && !variablePattern.MatchString(pool.HashKey) {
		return errors.New("Hash 算法必须使用安全的 Nginx 变量，例如 $request_uri")
	}
	if len(pool.Servers) == 0 || len(pool.Servers) > 64 {
		return errors.New("后端服务组需要 1 到 64 个服务节点")
	}
	if pool.Keepalive < 0 || pool.Keepalive > 4096 || pool.KeepaliveRequests < 1 || pool.KeepaliveRequests > 100000 || pool.KeepaliveTime < 1 || pool.KeepaliveTime > 86400 || pool.KeepaliveTimeout < 1 || pool.KeepaliveTimeout > 3600 {
		return errors.New("后端服务连接池参数超出允许范围")
	}
	for _, server := range pool.Servers {
		if err := validateHostName(server.Host, false); err != nil {
			return fmt.Errorf("后端服务节点 %q 不合法: %w", server.Host, err)
		}
		if server.Port < 1 || server.Port > 65535 {
			return errors.New("后端服务节点端口必须为 1 到 65535")
		}
		if server.Weight < 1 || server.Weight > 1000 || server.MaxFails < 0 || server.MaxFails > 100 || server.FailTimeout < 1 || server.FailTimeout > 86400 {
			return errors.New("后端服务节点权重或故障参数超出允许范围")
		}
	}
	return nil
}

func validateCIDRs(items []string) error {
	for _, item := range items {
		value := strings.TrimSpace(item)
		if net.ParseIP(value) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(value); err != nil {
			return fmt.Errorf("%q 不是有效 IP 或 CIDR", item)
		}
	}
	return nil
}

func validateSettingsAdvanced(settings Settings) error {
	if settings.WorkerProcesses < 0 || settings.WorkerProcesses > 128 {
		return errors.New("Worker 数量必须为 0 到 128，0 表示自动")
	}
	if settings.WorkerConnections < 128 || settings.WorkerConnections > 1048576 {
		return errors.New("Worker Connections 必须为 128 到 1048576")
	}
	if settings.WorkerRlimitNofile < 0 || settings.WorkerRlimitNofile > 1048576 {
		return errors.New("文件句柄限制超出允许范围")
	}
	if settings.ThreadPoolThreads < 0 || settings.ThreadPoolThreads > 1024 || settings.ThreadPoolQueue < 1 || settings.ThreadPoolQueue > 1048576 {
		return errors.New("线程池参数超出允许范围")
	}
	if settings.RealIP.Enabled {
		allowed := map[string]bool{"X-Real-IP": true, "X-Forwarded-For": true, "proxy_protocol": true}
		if !allowed[settings.RealIP.Header] {
			return errors.New("Real IP Header 不支持")
		}
		if len(settings.RealIP.TrustedProxies) == 0 {
			return errors.New("启用 Real IP 时至少需要一个可信代理地址")
		}
		if err := validateCIDRs(settings.RealIP.TrustedProxies); err != nil {
			return err
		}
	}
	if settings.Gzip.Level < 1 || settings.Gzip.Level > 9 || settings.Gzip.MinLength < 0 || settings.Gzip.MinLength > 10485760 {
		return errors.New("Gzip 参数超出允许范围")
	}
	if len(settings.Gzip.Types) > 64 {
		return errors.New("Gzip MIME 类型过多")
	}
	for _, item := range settings.Gzip.Types {
		if strings.ContainsAny(item, "{};\r\n\t ") {
			return errors.New("Gzip MIME 类型包含不允许的字符")
		}
	}
	if len(settings.TLS.Protocols) == 0 {
		return errors.New("至少启用一个 TLS 协议")
	}
	for _, protocol := range settings.TLS.Protocols {
		if protocol != "TLSv1.2" && protocol != "TLSv1.3" {
			return errors.New("只允许 TLSv1.2 和 TLSv1.3")
		}
	}
	if strings.ContainsAny(settings.TLS.Ciphers, "{};\r\n") {
		return errors.New("TLS 加密套件包含不允许的字符")
	}
	if settings.TLS.SessionCacheMB < 1 || settings.TLS.SessionCacheMB > 1024 || settings.TLS.SessionTimeoutMinutes < 1 || settings.TLS.SessionTimeoutMinutes > 1440 {
		return errors.New("TLS 会话参数超出允许范围")
	}
	if settings.TLS.ClientVerify != "off" && settings.TLS.ClientVerify != "on" && settings.TLS.ClientVerify != "optional" && settings.TLS.ClientVerify != "optional_no_ca" {
		return errors.New("客户端证书校验模式不支持")
	}
	if settings.TLS.ClientVerify != "off" && (!strings.HasPrefix(settings.TLS.ClientCAFile, "/") || strings.ContainsAny(settings.TLS.ClientCAFile, "\x00\r\n")) {
		return errors.New("客户端 CA 文件必须是绝对路径")
	}
	if settings.TLS.ClientVerifyDepth < 1 || settings.TLS.ClientVerifyDepth > 10 {
		return errors.New("客户端证书校验深度必须为 1 到 10")
	}
	levels := map[string]bool{"debug": true, "info": true, "notice": true, "warn": true, "error": true, "crit": true, "alert": true, "emerg": true}
	if !levels[settings.Logging.ErrorLevel] {
		return errors.New("错误日志级别不支持")
	}
	if settings.Logging.AccessBufferKB < 0 || settings.Logging.AccessBufferKB > 1024 || settings.Logging.AccessFlushSec < 1 || settings.Logging.AccessFlushSec > 3600 {
		return errors.New("访问日志缓冲参数超出允许范围")
	}
	if strings.ContainsAny(settings.Logging.CustomFormat, "{};\r\n") {
		return errors.New("自定义日志格式包含不允许的字符")
	}
	if settings.Logging.RotateSizeMB < 1 || settings.Logging.RotateSizeMB > 10240 || settings.Logging.RotateKeep < 1 || settings.Logging.RotateKeep > 100 {
		return errors.New("日志轮转参数超出允许范围")
	}
	if err := validateDynamicRouting(settings.Routing); err != nil {
		return err
	}
	return nil
}

func validateDynamicRouting(settings DynamicRoutingSettings) error {
	if len(settings.Maps)+len(settings.Geos)+len(settings.Splits) > 64 {
		return errors.New("动态路由定义不能超过 64 个")
	}
	checkVariable := func(value string) bool { return variablePattern.MatchString(value) }
	checkValue := func(value string) bool { return value != "" && !strings.ContainsAny(value, "{};\r\n") }
	for _, item := range settings.Maps {
		if item.Name == "" || !checkVariable(item.Source) || !checkVariable(item.Variable) || !checkValue(item.Default) {
			return errors.New("Map 定义不完整或包含不安全值")
		}
		for _, entry := range item.Entries {
			if !checkValue(entry.Key) || !checkValue(entry.Value) {
				return errors.New("Map 条目不合法")
			}
		}
	}
	for _, item := range settings.Geos {
		if item.Name == "" || (item.Source != "" && !checkVariable(item.Source)) || !checkVariable(item.Variable) || !checkValue(item.Default) {
			return errors.New("Geo 定义不完整或包含不安全值")
		}
		for _, entry := range item.Entries {
			if !checkValue(entry.Key) || !checkValue(entry.Value) {
				return errors.New("Geo 条目不合法")
			}
			if entry.Key != "default" && net.ParseIP(entry.Key) == nil {
				if _, _, err := net.ParseCIDR(entry.Key); err != nil {
					return errors.New("Geo 键必须是 IP 或 CIDR")
				}
			}
		}
	}
	percentage := regexp.MustCompile(`^(?:100|[0-9]{1,2}(?:\.[0-9]{1,2})?)%$`)
	for _, item := range settings.Splits {
		if item.Name == "" || !checkVariable(item.Source) || !checkVariable(item.Variable) || len(item.Entries) == 0 {
			return errors.New("Split Clients 定义不完整")
		}
		for index, entry := range item.Entries {
			if (!percentage.MatchString(entry.Key) && !(entry.Key == "*" && index == len(item.Entries)-1)) || !checkValue(entry.Value) {
				return errors.New("Split Clients 条目不合法")
			}
		}
	}
	return nil
}

func validateRateLimit(value RateLimitSettings) error {
	if !value.Enabled {
		return nil
	}
	if value.RequestsPerSecond < 1 || value.RequestsPerSecond > 100000 || value.Burst < 0 || value.Burst > 100000 || value.Connections < 0 || value.Connections > 100000 || value.DownloadKBps < 0 || value.DownloadKBps > 1048576 {
		return errors.New("限流参数超出允许范围")
	}
	return nil
}

func ValidateRateLimitPolicy(policy RateLimitPolicy) error {
	if !idPattern.MatchString(policy.ID) {
		return errors.New("限流策略 ID 格式不正确")
	}
	if len([]rune(policy.Name)) < 1 || len([]rune(policy.Name)) > 80 {
		return errors.New("限流策略名称长度必须为 1 到 80 个字符")
	}
	policy.Settings.Enabled = true
	return validateRateLimit(policy.Settings)
}
