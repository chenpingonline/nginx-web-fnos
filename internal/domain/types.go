package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	AppName       = "nginx-web"
	AppVersion    = "0.1.5"
	BuildIdentity = "nginx-web 0.1.5"
	NginxVersion  = "1.30.4"
	SchemaVersion = 4
)

type Settings struct {
	DefaultHTTPPort    int                    `json:"default_http_port"`
	DefaultHTTPSPort   int                    `json:"default_https_port"`
	RevisionLimit      int                    `json:"revision_limit"`
	WorkerProcesses    int                    `json:"worker_processes"`
	WorkerConnections  int                    `json:"worker_connections"`
	WorkerRlimitNofile int                    `json:"worker_rlimit_nofile"`
	MultiAccept        bool                   `json:"multi_accept"`
	FileAIO            bool                   `json:"file_aio"`
	ThreadPoolThreads  int                    `json:"thread_pool_threads"`
	ThreadPoolQueue    int                    `json:"thread_pool_queue"`
	RealIP             RealIPSettings         `json:"real_ip"`
	Gzip               GzipSettings           `json:"gzip"`
	TLS                TLSSettings            `json:"tls"`
	Logging            LoggingSettings        `json:"logging"`
	Routing            DynamicRoutingSettings `json:"routing"`
}

type ProxyRule struct {
	ID                    string            `json:"id"`
	Name                  string            `json:"name"`
	Enabled               bool              `json:"enabled"`
	ListenPort            int               `json:"listen_port"`
	Domains               []string          `json:"domains"`
	TLS                   bool              `json:"tls"`
	HTTP2                 bool              `json:"http2"`
	CertificateID         string            `json:"certificate_id,omitempty"`
	UpstreamScheme        string            `json:"upstream_scheme"`
	UpstreamHost          string            `json:"upstream_host"`
	UpstreamPort          int               `json:"upstream_port"`
	PreserveHost          bool              `json:"preserve_host"`
	WebSocket             bool              `json:"websocket"`
	Streaming             bool              `json:"streaming"`
	VerifyUpstreamTLS     bool              `json:"verify_upstream_tls"`
	ConnectTimeoutSeconds int               `json:"connect_timeout_seconds"`
	ReadTimeoutSeconds    int               `json:"read_timeout_seconds"`
	SendTimeoutSeconds    int               `json:"send_timeout_seconds"`
	ClientMaxBodyMB       int               `json:"client_max_body_mb"`
	UpstreamPoolID        string            `json:"upstream_pool_id,omitempty"`
	RateLimit             RateLimitSettings `json:"rate_limit"`
	RootLocation          LocationSettings  `json:"root_location"`
	Locations             []LocationRule    `json:"locations"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
}

type CertificateMeta struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Subject      string    `json:"subject"`
	DNSNames     []string  `json:"dns_names"`
	IPAddresses  []string  `json:"ip_addresses"`
	SerialNumber string    `json:"serial_number"`
	NotBefore    time.Time `json:"not_before"`
	NotAfter     time.Time `json:"not_after"`
	Fingerprint  string    `json:"fingerprint"`
	CreatedAt    time.Time `json:"created_at"`
}

type State struct {
	SchemaVersion    int               `json:"schema_version"`
	Settings         Settings          `json:"settings"`
	Rules            []ProxyRule       `json:"rules"`
	Certificates     []CertificateMeta `json:"certificates"`
	UpstreamPools    []UpstreamPool    `json:"upstream_pools"`
	StreamRules      []StreamRule      `json:"stream_rules"`
	Dirty            bool              `json:"dirty"`
	LastAppliedAt    *time.Time        `json:"last_applied_at,omitempty"`
	LastApplyMessage string            `json:"last_apply_message,omitempty"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

type Revision struct {
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	Summary      string    `json:"summary"`
	RuleCount    int       `json:"rule_count"`
	EnabledCount int       `json:"enabled_count"`
	State        State     `json:"state"`
}

type NginxStatus struct {
	Running    bool   `json:"running"`
	PID        int    `json:"pid,omitempty"`
	Version    string `json:"version"`
	Ports      []int  `json:"ports"`
	ConfigPath string `json:"config_path"`
	LastError  string `json:"last_error,omitempty"`
}

type ApplyResult struct {
	Action  string `json:"action"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
}

func DefaultState() State {
	now := time.Now().UTC()
	return State{
		SchemaVersion: SchemaVersion,
		Settings: Settings{
			DefaultHTTPPort:   9080,
			DefaultHTTPSPort:  9443,
			RevisionLimit:     20,
			WorkerConnections: 1024,
			MultiAccept:       true,
			ThreadPoolQueue:   65536,
			RealIP:            defaultRealIPSettings(),
			Gzip:              defaultGzipSettings(),
			TLS:               defaultTLSSettings(),
			Logging:           defaultLoggingSettings(),
			Routing:           DynamicRoutingSettings{Maps: []MapDefinition{}, Geos: []GeoDefinition{}, Splits: []SplitDefinition{}},
		},
		Rules:         []ProxyRule{},
		Certificates:  []CertificateMeta{},
		UpstreamPools: []UpstreamPool{},
		StreamRules:   []StreamRule{},
		Dirty:         true,
		UpdatedAt:     now,
	}
}

func CloneState(in State) State {
	data, _ := json.Marshal(in)
	var out State
	_ = json.Unmarshal(data, &out)
	return out
}

var (
	hostLabelPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
)

func NormalizeRule(rule *ProxyRule, settings Settings) {
	rule.Name = strings.TrimSpace(rule.Name)
	rule.UpstreamScheme = strings.ToLower(strings.TrimSpace(rule.UpstreamScheme))
	rule.UpstreamHost = strings.TrimSpace(strings.Trim(rule.UpstreamHost, "[]"))
	rule.CertificateID = strings.TrimSpace(rule.CertificateID)
	rule.UpstreamPoolID = strings.TrimSpace(rule.UpstreamPoolID)
	if rule.Locations == nil {
		rule.Locations = []LocationRule{}
	}
	if rule.RootLocation.BackendType == "" {
		rule.RootLocation = defaultLocationSettings()
		rule.RootLocation.UpstreamScheme = rule.UpstreamScheme
		rule.RootLocation.UpstreamPoolID = rule.UpstreamPoolID
		rule.RootLocation.UpstreamHost = rule.UpstreamHost
		rule.RootLocation.UpstreamPort = rule.UpstreamPort
	}
	NormalizeHTTPLocations(rule)

	seen := make(map[string]struct{})
	normalized := make([]string, 0, len(rule.Domains))
	for _, domain := range rule.Domains {
		for _, item := range strings.FieldsFunc(domain, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		}) {
			item = strings.ToLower(strings.TrimSpace(item))
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			normalized = append(normalized, item)
		}
	}
	sort.Strings(normalized)
	rule.Domains = normalized

	if rule.ListenPort == 0 {
		if rule.TLS {
			rule.ListenPort = settings.DefaultHTTPSPort
		} else {
			rule.ListenPort = settings.DefaultHTTPPort
		}
	}
	if rule.UpstreamScheme == "" {
		rule.UpstreamScheme = "http"
	}
	if rule.ConnectTimeoutSeconds == 0 {
		rule.ConnectTimeoutSeconds = 10
	}
	if rule.ReadTimeoutSeconds == 0 {
		rule.ReadTimeoutSeconds = 3600
	}
	if rule.SendTimeoutSeconds == 0 {
		rule.SendTimeoutSeconds = 3600
	}
	if rule.TLS && !rule.HTTP2 {
		// HTTP/2 remains opt-in in the API, but new UI rules set it to true.
	}
}

func ValidateRule(rule ProxyRule, certs map[string]CertificateMeta, pools ...map[string]UpstreamPool) error {
	if !idPattern.MatchString(rule.ID) {
		return errors.New("规则 ID 格式不正确")
	}
	if len([]rune(rule.Name)) < 1 || len([]rune(rule.Name)) > 80 {
		return errors.New("规则名称长度必须为 1 到 80 个字符")
	}
	if rule.ListenPort < 1024 || rule.ListenPort > 65535 {
		return errors.New("第一版仅允许监听 1024 到 65535 的非特权端口")
	}
	if len(rule.Domains) == 0 {
		return errors.New("至少需要填写一个访问域名或 IP")
	}
	catchAll := false
	for _, domain := range rule.Domains {
		if domain == "*" {
			catchAll = true
			continue
		}
		if err := validateHostName(domain, true); err != nil {
			return fmt.Errorf("访问域名 %q 不合法: %w", domain, err)
		}
	}
	if catchAll && len(rule.Domains) != 1 {
		return errors.New("通配符 * 必须单独使用")
	}
	if rule.TLS {
		if rule.CertificateID == "" {
			return errors.New("HTTPS 规则必须选择证书")
		}
		if _, ok := certs[rule.CertificateID]; !ok {
			return errors.New("HTTPS 规则引用的证书不存在")
		}
	} else if rule.CertificateID != "" {
		return errors.New("HTTP 规则不能绑定 HTTPS 证书")
	}
	poolMap := map[string]UpstreamPool{}
	if len(pools) > 0 {
		poolMap = pools[0]
	}
	if rule.UpstreamPoolID != "" {
		pool, ok := poolMap[rule.UpstreamPoolID]
		if !ok {
			return errors.New("引用的目标服务池不存在")
		}
		if pool.Protocol != "http" {
			return errors.New("HTTP 规则只能引用 HTTP 目标服务池")
		}
	}
	if rule.UpstreamScheme != "http" && rule.UpstreamScheme != "https" {
		return errors.New("目标服务协议只能是 http 或 https")
	}
	if rule.UpstreamPoolID == "" {
		if err := validateHostName(rule.UpstreamHost, false); err != nil {
			return fmt.Errorf("目标主机不合法: %w", err)
		}
	}
	if rule.UpstreamPoolID == "" && (rule.UpstreamPort < 1 || rule.UpstreamPort > 65535) {
		return errors.New("目标端口必须为 1 到 65535")
	}
	if rule.ConnectTimeoutSeconds < 1 || rule.ConnectTimeoutSeconds > 600 {
		return errors.New("连接超时必须为 1 到 600 秒")
	}
	if rule.ReadTimeoutSeconds < 1 || rule.ReadTimeoutSeconds > 86400 {
		return errors.New("读取超时必须为 1 到 86400 秒")
	}
	if rule.SendTimeoutSeconds < 1 || rule.SendTimeoutSeconds > 86400 {
		return errors.New("发送超时必须为 1 到 86400 秒")
	}
	if rule.ClientMaxBodyMB < 0 || rule.ClientMaxBodyMB > 102400 {
		return errors.New("请求体上限必须为 0 到 102400 MB，0 表示不限制")
	}
	if err := validateRateLimit(rule.RateLimit); err != nil {
		return err
	}
	if err := ValidateHTTPLocations(rule, poolMap); err != nil {
		return err
	}
	return nil
}

func ValidateState(state State) error {
	ApplyStateDefaults(&state)
	if state.Settings.DefaultHTTPPort < 1024 || state.Settings.DefaultHTTPPort > 65535 {
		return errors.New("默认 HTTP 端口不合法")
	}
	if state.Settings.DefaultHTTPSPort < 1024 || state.Settings.DefaultHTTPSPort > 65535 {
		return errors.New("默认 HTTPS 端口不合法")
	}
	if state.Settings.RevisionLimit < 1 || state.Settings.RevisionLimit > 100 {
		return errors.New("配置历史保留数量必须为 1 到 100")
	}
	if err := validateSettingsAdvanced(state.Settings); err != nil {
		return err
	}

	certs := make(map[string]CertificateMeta, len(state.Certificates))
	for _, cert := range state.Certificates {
		if !idPattern.MatchString(cert.ID) {
			return errors.New("证书 ID 格式不正确")
		}
		if _, exists := certs[cert.ID]; exists {
			return errors.New("存在重复的证书 ID")
		}
		certs[cert.ID] = cert
	}
	pools := make(map[string]UpstreamPool, len(state.UpstreamPools))
	poolNames := make(map[string]struct{}, len(state.UpstreamPools))
	for _, pool := range state.UpstreamPools {
		if err := ValidateUpstreamPool(pool); err != nil {
			return fmt.Errorf("目标服务池 %q: %w", pool.Name, err)
		}
		if _, exists := pools[pool.ID]; exists {
			return errors.New("存在重复的目标服务池 ID")
		}
		key := strings.ToLower(pool.Name)
		if _, exists := poolNames[key]; exists {
			return errors.New("存在重复的目标服务池名称")
		}
		pools[pool.ID] = pool
		poolNames[key] = struct{}{}
	}

	type portInfo struct {
		tls      bool
		domains  map[string]string
		catchAll string
	}
	ports := make(map[int]*portInfo)
	ids := make(map[string]struct{})

	for _, rule := range state.Rules {
		if _, exists := ids[rule.ID]; exists {
			return errors.New("存在重复的规则 ID")
		}
		ids[rule.ID] = struct{}{}
		if err := ValidateRule(rule, certs, pools); err != nil {
			return fmt.Errorf("规则 %q: %w", rule.Name, err)
		}
		if !rule.Enabled {
			continue
		}
		info, ok := ports[rule.ListenPort]
		if !ok {
			info = &portInfo{tls: rule.TLS, domains: make(map[string]string)}
			ports[rule.ListenPort] = info
		} else if info.tls != rule.TLS {
			return fmt.Errorf("端口 %d 不能同时承载 HTTP 与 HTTPS 规则", rule.ListenPort)
		}
		for _, domain := range rule.Domains {
			if domain == "*" {
				if info.catchAll != "" && info.catchAll != rule.ID {
					return fmt.Errorf("端口 %d 只能有一条默认规则", rule.ListenPort)
				}
				info.catchAll = rule.ID
				continue
			}
			if previous, exists := info.domains[domain]; exists && previous != rule.ID {
				return fmt.Errorf("端口 %d 上的域名 %s 被多条规则重复使用", rule.ListenPort, domain)
			}
			info.domains[domain] = rule.ID
		}
	}
	streamPorts := make(map[string]string)
	for _, rule := range state.StreamRules {
		if err := ValidateStreamRule(rule, certs, pools); err != nil {
			return fmt.Errorf("Stream 规则 %q: %w", rule.Name, err)
		}
		if !rule.Enabled {
			continue
		}
		key := fmt.Sprintf("%s/%s/%d", rule.Protocol, rule.ListenAddress, rule.ListenPort)
		if previous, exists := streamPorts[key]; exists {
			return fmt.Errorf("Stream 规则 %q 与 %q 的监听地址冲突", rule.Name, previous)
		}
		streamPorts[key] = rule.Name
		if rule.Protocol == "tcp" && (rule.ListenAddress == "*" || rule.ListenAddress == "0.0.0.0" || rule.ListenAddress == "::") {
			if _, exists := ports[rule.ListenPort]; exists {
				return fmt.Errorf("Stream TCP 端口 %d 与 HTTP 规则冲突", rule.ListenPort)
			}
		}
	}
	return nil
}

func validateHostName(value string, allowWildcard bool) error {
	value = strings.TrimSpace(strings.Trim(value, "[]"))
	if value == "" {
		return errors.New("不能为空")
	}
	if len(value) > 253 {
		return errors.New("长度超过 253 个字符")
	}
	if net.ParseIP(value) != nil {
		return nil
	}
	if allowWildcard && strings.HasPrefix(value, "*.") {
		value = strings.TrimPrefix(value, "*.")
	}
	if strings.ContainsAny(value, "/:@?#\\;{}\"'") {
		return errors.New("包含不允许的字符")
	}
	labels := strings.Split(value, ".")
	for _, label := range labels {
		if !hostLabelPattern.MatchString(label) {
			return fmt.Errorf("标签 %q 格式不正确", label)
		}
	}
	return nil
}

func EnabledRuleCount(state State) int {
	count := 0
	for _, rule := range state.Rules {
		if rule.Enabled {
			count++
		}
	}
	for _, rule := range state.StreamRules {
		if rule.Enabled {
			count++
		}
	}
	return count
}

func ActivePorts(state State) []int {
	set := make(map[int]struct{})
	for _, rule := range state.Rules {
		if rule.Enabled {
			set[rule.ListenPort] = struct{}{}
		}
	}
	for _, rule := range state.StreamRules {
		if rule.Enabled {
			set[rule.ListenPort] = struct{}{}
		}
	}
	if len(set) == 0 {
		set[state.Settings.DefaultHTTPPort] = struct{}{}
	}
	ports := make([]int, 0, len(set))
	for port := range set {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports
}
