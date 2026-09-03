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
}

type LoggingSettings struct {
	AccessEnabled  bool   `json:"access_enabled"`
	ErrorLevel     string `json:"error_level"`
	AccessBufferKB int    `json:"access_buffer_kb"`
	AccessFlushSec int    `json:"access_flush_seconds"`
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

var variablePattern = regexp.MustCompile(`^\$[A-Za-z0-9_]+$`)

func defaultRealIPSettings() RealIPSettings {
	return RealIPSettings{Header: "X-Forwarded-For", TrustedProxies: []string{}}
}

func defaultGzipSettings() GzipSettings {
	return GzipSettings{Enabled: true, Level: 5, MinLength: 1024, Types: []string{"text/plain", "text/css", "application/json", "application/javascript", "application/xml", "image/svg+xml"}, Static: true, Gunzip: true}
}

func defaultTLSSettings() TLSSettings {
	return TLSSettings{Protocols: []string{"TLSv1.2", "TLSv1.3"}, SessionCacheMB: 10, SessionTimeoutMinutes: 10}
}

func defaultLoggingSettings() LoggingSettings {
	return LoggingSettings{AccessEnabled: true, ErrorLevel: "notice", AccessBufferKB: 32, AccessFlushSec: 5}
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
	if state.Settings.RealIP.Header == "" {
		state.Settings.RealIP = defaults.RealIP
	}
	if state.Settings.Gzip.Level == 0 {
		state.Settings.Gzip = defaults.Gzip
	}
	if len(state.Settings.TLS.Protocols) == 0 {
		state.Settings.TLS = defaults.TLS
	}
	if state.Settings.Logging.ErrorLevel == "" {
		state.Settings.Logging = defaults.Logging
	}
	if state.Rules == nil {
		state.Rules = []ProxyRule{}
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
	for routeIndex := range rule.SNIRoutes {
		route := &rule.SNIRoutes[routeIndex]
		route.UpstreamPoolID = strings.TrimSpace(route.UpstreamPoolID)
		route.UpstreamHost = strings.TrimSpace(strings.Trim(route.UpstreamHost, "[]"))
		for index := range route.ServerNames {
			route.ServerNames[index] = strings.ToLower(strings.TrimSpace(route.ServerNames[index]))
		}
	}
}

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
			return errors.New("引用的 Stream 上游池不存在")
		}
		if pool.Protocol != "stream" {
			return errors.New("Stream 规则只能引用 Stream 上游池")
		}
		return nil
	}
	if err := validateHostName(host, false); err != nil {
		return fmt.Errorf("Stream 上游主机不合法: %w", err)
	}
	if port < 1 || port > 65535 {
		return errors.New("Stream 上游端口必须为 1 到 65535")
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
		return errors.New("上游池 ID 格式不正确")
	}
	if len([]rune(pool.Name)) < 1 || len([]rune(pool.Name)) > 80 {
		return errors.New("上游池名称长度必须为 1 到 80 个字符")
	}
	if pool.Protocol != "http" && pool.Protocol != "stream" {
		return errors.New("上游池协议只能是 http 或 stream")
	}
	allowedStrategy := map[string]bool{"round_robin": true, "least_conn": true, "ip_hash": true, "hash": true, "random": true}
	if !allowedStrategy[pool.Strategy] {
		return errors.New("上游池负载均衡算法不支持")
	}
	if pool.Strategy == "hash" && !variablePattern.MatchString(pool.HashKey) {
		return errors.New("Hash 算法必须使用安全的 Nginx 变量，例如 $request_uri")
	}
	if len(pool.Servers) == 0 || len(pool.Servers) > 64 {
		return errors.New("上游池需要 1 到 64 个服务器")
	}
	if pool.Keepalive < 0 || pool.Keepalive > 4096 || pool.KeepaliveRequests < 1 || pool.KeepaliveRequests > 100000 || pool.KeepaliveTime < 1 || pool.KeepaliveTime > 86400 || pool.KeepaliveTimeout < 1 || pool.KeepaliveTimeout > 3600 {
		return errors.New("上游连接池参数超出允许范围")
	}
	for _, server := range pool.Servers {
		if err := validateHostName(server.Host, false); err != nil {
			return fmt.Errorf("上游服务器 %q 不合法: %w", server.Host, err)
		}
		if server.Port < 1 || server.Port > 65535 {
			return errors.New("上游服务器端口必须为 1 到 65535")
		}
		if server.Weight < 1 || server.Weight > 1000 || server.MaxFails < 0 || server.MaxFails > 100 || server.FailTimeout < 1 || server.FailTimeout > 86400 {
			return errors.New("上游服务器权重或故障参数超出允许范围")
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
	levels := map[string]bool{"debug": true, "info": true, "notice": true, "warn": true, "error": true, "crit": true, "alert": true, "emerg": true}
	if !levels[settings.Logging.ErrorLevel] {
		return errors.New("错误日志级别不支持")
	}
	if settings.Logging.AccessBufferKB < 0 || settings.Logging.AccessBufferKB > 1024 || settings.Logging.AccessFlushSec < 1 || settings.Logging.AccessFlushSec > 3600 {
		return errors.New("访问日志缓冲参数超出允许范围")
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
