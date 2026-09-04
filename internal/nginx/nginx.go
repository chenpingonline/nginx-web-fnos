package nginx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chenpingonline/fn-nginx-web/internal/domain"
	"github.com/chenpingonline/fn-nginx-web/internal/fileutil"
	"github.com/chenpingonline/fn-nginx-web/internal/platform"
)

type Paths = platform.Paths
type State = domain.State
type NginxStatus = domain.NginxStatus
type ApplyResult = domain.ApplyResult
type ProxyRule = domain.ProxyRule
type CertificateMeta = domain.CertificateMeta
type Settings = domain.Settings

type Manager struct {
	paths Paths
	mu    sync.Mutex
}

func New(paths Paths) *Manager {
	if paths.NginxCacheDir == "" {
		paths.NginxCacheDir = filepath.Join(paths.VarDir, "cache")
	}
	return &Manager{paths: paths}
}

func (m *Manager) CheckBinary() error {
	info, err := os.Stat(m.paths.NginxBin)
	if err != nil {
		return fmt.Errorf("找不到应用自带的 Nginx: %w", err)
	}
	if info.Mode()&0o111 == 0 {
		return errors.New("应用自带的 Nginx 没有执行权限")
	}
	version, err := m.Version()
	if err != nil {
		return err
	}
	if version != domain.NginxVersion {
		return fmt.Errorf("Nginx 版本不匹配，期望 %s，实际 %s", domain.NginxVersion, version)
	}
	return nil
}

func (m *Manager) Version() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := m.runCommand(ctx, "-v")
	text := strings.TrimSpace(string(output))
	match := regexp.MustCompile(`nginx/([^\s]+)`).FindStringSubmatch(text)
	if len(match) == 2 {
		return match[1], nil
	}
	if err != nil {
		return "", fmt.Errorf("读取 Nginx 版本失败: %w: %s", err, text)
	}
	return "", fmt.Errorf("无法识别 Nginx 版本: %s", text)
}

func (m *Manager) Status(state State) NginxStatus {
	pid, running := m.runningPID()
	version, _ := m.Version()
	lines, _ := fileutil.TailLines(m.paths.NginxErrorLog, 80)
	return NginxStatus{
		Running:    running,
		PID:        pid,
		Version:    version,
		Ports:      domain.ActivePorts(state),
		ConfigPath: m.paths.NginxMaster,
		LastError:  lastNginxError(lines),
	}
}

func lastNginxError(lines []string) string {
	for index := len(lines) - 1; index >= 0; index-- {
		lower := strings.ToLower(lines[index])
		if strings.Contains(lower, "[emerg]") || strings.Contains(lower, "[alert]") ||
			strings.Contains(lower, "[crit]") || strings.Contains(lower, "[error]") {
			return lines[index]
		}
	}
	return ""
}

func (m *Manager) IsRunning() bool {
	_, running := m.runningPID()
	return running
}

func (m *Manager) runningPID() (int, bool) {
	data, err := os.ReadFile(m.paths.NginxPID)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 1 {
		return 0, false
	}
	if err := syscall.Kill(pid, 0); err != nil {
		return 0, false
	}
	actual, err := filepath.EvalSymlinks(fmt.Sprintf("/proc/%d/exe", pid))
	if err == nil {
		expected, expectedErr := filepath.EvalSymlinks(m.paths.NginxBin)
		if expectedErr == nil && actual != expected {
			return 0, false
		}
	}
	return pid, true
}

func (m *Manager) TestCurrent() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.testConfigUnlocked(m.paths.NginxMaster)
}

func (m *Manager) Start() (ApplyResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startUnlocked()
}

func (m *Manager) Reload() (ApplyResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reloadUnlocked()
}

func (m *Manager) Stop() (ApplyResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pid, running := m.runningPID()
	if !running {
		_ = os.Remove(m.paths.NginxPID)
		return ApplyResult{Action: "stop", Message: "Nginx 已停止"}, nil
	}
	if err := syscall.Kill(pid, syscall.SIGQUIT); err != nil {
		return ApplyResult{}, fmt.Errorf("发送优雅停止信号失败: %w", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, alive := m.runningPID(); !alive {
			_ = os.Remove(m.paths.NginxPID)
			return ApplyResult{Action: "stop", Message: "Nginx 已优雅停止"}, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	time.Sleep(300 * time.Millisecond)
	if _, alive := m.runningPID(); alive {
		return ApplyResult{}, errors.New("Nginx 未能在超时时间内停止")
	}
	_ = os.Remove(m.paths.NginxPID)
	return ApplyResult{Action: "stop", Message: "Nginx 已停止"}, nil
}

func (m *Manager) ReopenLogs() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	pid, running := m.runningPID()
	if !running {
		return nil
	}
	if err := syscall.Kill(pid, syscall.SIGUSR1); err != nil {
		return fmt.Errorf("通知 Nginx 重新打开日志失败: %w", err)
	}
	return nil
}

func (m *Manager) Prepare(state State) (ApplyResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.installStateUnlocked(state, false)
}

func (m *Manager) Apply(state State) (ApplyResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.installStateUnlocked(state, true)
}

func (m *Manager) installStateUnlocked(state State, activate bool) (ApplyResult, error) {
	if err := domain.ValidateState(state); err != nil {
		return ApplyResult{}, err
	}
	if err := m.paths.Ensure(); err != nil {
		return ApplyResult{}, err
	}
	if err := m.CheckBinary(); err != nil {
		return ApplyResult{}, err
	}

	// First validate a completely isolated candidate tree. This prevents a bad
	// form value from touching the active configuration directory.
	candidateRoot, err := os.MkdirTemp(m.paths.TmpDir, "fnproxy-candidate-")
	if err != nil {
		return ApplyResult{}, err
	}
	defer os.RemoveAll(candidateRoot)
	candidateConfD := filepath.Join(candidateRoot, "conf.d")
	if err := os.MkdirAll(candidateConfD, 0o750); err != nil {
		return ApplyResult{}, err
	}
	candidateMaster, candidateFiles, err := m.render(state, candidateConfD)
	if err != nil {
		return ApplyResult{}, err
	}
	candidateMasterPath := filepath.Join(candidateRoot, "nginx.conf")
	if err := fileutil.WriteFileAtomic(candidateMasterPath, []byte(candidateMaster), 0o640); err != nil {
		return ApplyResult{}, err
	}
	for name, content := range candidateFiles {
		if err := fileutil.WriteFileAtomic(filepath.Join(candidateConfD, name), []byte(content), 0o640); err != nil {
			return ApplyResult{}, err
		}
	}
	if output, err := m.testConfigUnlocked(candidateMasterPath); err != nil {
		return ApplyResult{}, fmt.Errorf("候选配置校验失败: %w\n%s", err, output)
	}

	// Render again with the final include path, then swap the whole conf.d
	// directory. A failed validation or activation restores the previous tree.
	persistentMaster, persistentFiles, err := m.render(state, m.paths.NginxConfD)
	if err != nil {
		return ApplyResult{}, err
	}
	newConfD := filepath.Join(m.paths.NginxConfigDir, ".conf.d-new-"+domain.RandomID())
	oldConfD := filepath.Join(m.paths.NginxConfigDir, ".conf.d-old-"+domain.RandomID())
	if err := os.MkdirAll(newConfD, 0o750); err != nil {
		return ApplyResult{}, err
	}
	defer os.RemoveAll(newConfD)
	for name, content := range persistentFiles {
		if err := fileutil.WriteFileAtomic(filepath.Join(newConfD, name), []byte(content), 0o640); err != nil {
			return ApplyResult{}, err
		}
	}
	newMaster := filepath.Join(m.paths.NginxConfigDir, ".nginx.conf-new-"+domain.RandomID())
	if err := fileutil.WriteFileAtomic(newMaster, []byte(persistentMaster), 0o640); err != nil {
		return ApplyResult{}, err
	}
	defer os.Remove(newMaster)

	oldMaster, oldMasterErr := os.ReadFile(m.paths.NginxMaster)
	hadOldMaster := oldMasterErr == nil
	if oldMasterErr != nil && !errors.Is(oldMasterErr, os.ErrNotExist) {
		return ApplyResult{}, oldMasterErr
	}
	hadOldConfD := false
	if _, statErr := os.Stat(m.paths.NginxConfD); statErr == nil {
		hadOldConfD = true
		if err := os.Rename(m.paths.NginxConfD, oldConfD); err != nil {
			return ApplyResult{}, err
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return ApplyResult{}, statErr
	}
	rollback := func() {
		_ = os.RemoveAll(m.paths.NginxConfD)
		if hadOldConfD {
			_ = os.Rename(oldConfD, m.paths.NginxConfD)
		} else {
			_ = os.MkdirAll(m.paths.NginxConfD, 0o750)
		}
		if hadOldMaster {
			_ = fileutil.WriteFileAtomic(m.paths.NginxMaster, oldMaster, 0o640)
		} else {
			_ = os.Remove(m.paths.NginxMaster)
		}
	}

	if err := os.Rename(newConfD, m.paths.NginxConfD); err != nil {
		rollback()
		return ApplyResult{}, err
	}
	if err := os.Rename(newMaster, m.paths.NginxMaster); err != nil {
		rollback()
		return ApplyResult{}, err
	}
	if output, err := m.testConfigUnlocked(m.paths.NginxMaster); err != nil {
		rollback()
		return ApplyResult{}, fmt.Errorf("持久配置校验失败: %w\n%s", err, output)
	}

	if !activate {
		_ = os.RemoveAll(oldConfD)
		return ApplyResult{Action: "prepare", Message: "Nginx 配置已生成并通过校验"}, nil
	}

	_, wasRunning := m.runningPID()
	var result ApplyResult
	if wasRunning {
		result, err = m.reloadUnlocked()
	} else {
		result, err = m.startUnlocked()
	}
	if err != nil {
		rollback()
		if wasRunning {
			_, _ = m.reloadUnlocked()
		}
		return ApplyResult{}, fmt.Errorf("激活新配置失败，已恢复旧配置: %w", err)
	}
	_ = os.RemoveAll(oldConfD)
	result.Action = "apply"
	if wasRunning {
		result.Message = "配置已校验并平滑重载"
	} else {
		result.Message = "配置已校验，Nginx 已启动"
	}
	return result, nil
}

func (m *Manager) startUnlocked() (ApplyResult, error) {
	if _, running := m.runningPID(); running {
		return ApplyResult{Action: "start", Message: "Nginx 已在运行"}, nil
	}
	if output, err := m.testConfigUnlocked(m.paths.NginxMaster); err != nil {
		return ApplyResult{}, fmt.Errorf("启动前配置校验失败: %w\n%s", err, output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	output, err := m.runCommand(ctx, "-p", ensureTrailingSlash(m.paths.NginxPrefix), "-c", m.paths.NginxMaster)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("Nginx 启动失败: %w: %s", err, strings.TrimSpace(string(output)))
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if pid, running := m.runningPID(); running {
			return ApplyResult{Action: "start", Message: fmt.Sprintf("Nginx 已启动，PID %d", pid), Output: strings.TrimSpace(string(output))}, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	lines, _ := fileutil.TailLines(m.paths.NginxErrorLog, 30)
	return ApplyResult{}, fmt.Errorf("Nginx 启动后未检测到主进程: %s", strings.Join(lines, "\n"))
}

func (m *Manager) reloadUnlocked() (ApplyResult, error) {
	if _, running := m.runningPID(); !running {
		return m.startUnlocked()
	}
	if output, err := m.testConfigUnlocked(m.paths.NginxMaster); err != nil {
		return ApplyResult{}, fmt.Errorf("重载前配置校验失败: %w\n%s", err, output)
	}
	offset := fileutil.FileSize(m.paths.NginxErrorLog)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := m.runCommand(ctx, "-p", ensureTrailingSlash(m.paths.NginxPrefix), "-c", m.paths.NginxMaster, "-s", "reload")
	if err != nil {
		return ApplyResult{}, fmt.Errorf("发送重载信号失败: %w: %s", err, strings.TrimSpace(string(output)))
	}
	time.Sleep(700 * time.Millisecond)
	if _, running := m.runningPID(); !running {
		return ApplyResult{}, errors.New("重载后 Nginx 主进程已退出")
	}
	newLog := fileutil.ReadFileSegment(m.paths.NginxErrorLog, offset)
	lower := strings.ToLower(newLog)
	if strings.Contains(lower, "[emerg]") || strings.Contains(lower, "still could not bind") {
		return ApplyResult{}, fmt.Errorf("Nginx 拒绝了新配置: %s", strings.TrimSpace(newLog))
	}
	return ApplyResult{Action: "reload", Message: "Nginx 已平滑重载", Output: strings.TrimSpace(string(output))}, nil
}

func (m *Manager) testConfigUnlocked(configPath string) (string, error) {
	if _, err := os.Stat(configPath); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	output, err := m.runCommand(ctx, "-p", ensureTrailingSlash(m.paths.NginxPrefix), "-c", configPath, "-t")
	return strings.TrimSpace(string(output)), err
}

func (m *Manager) runCommand(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, m.paths.NginxBin, args...)
	cmd.Dir = m.paths.NginxPrefix
	libraryPath := filepath.Join(m.paths.AppDest, "lib")
	if current := os.Getenv("LD_LIBRARY_PATH"); current != "" {
		libraryPath += ":" + current
	}
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+libraryPath)
	return cmd.CombinedOutput()
}

func (m *Manager) render(state State, confDPath string) (string, map[string]string, error) {
	domain.ApplyStateDefaults(&state)
	if err := domain.ValidateState(state); err != nil {
		return "", nil, err
	}
	certs := make(map[string]CertificateMeta, len(state.Certificates))
	for _, cert := range state.Certificates {
		certs[cert.ID] = cert
	}
	pools := make(map[string]domain.UpstreamPool, len(state.UpstreamPools))
	for _, pool := range state.UpstreamPools {
		pools[pool.ID] = pool
	}
	policies := make(map[string]domain.RateLimitPolicy, len(state.RateLimitPolicies))
	for _, policy := range state.RateLimitPolicies {
		policies[policy.ID] = policy
	}
	for index := range state.Rules {
		policy, exists := policies[state.Rules[index].RateLimitPolicyID]
		if !exists {
			state.Rules[index].RateLimit.Enabled = false
			continue
		}
		state.Rules[index].RateLimit = policy.Settings
		state.Rules[index].RateLimit.Enabled = true
	}

	type group struct {
		port  int
		tls   bool
		rules []ProxyRule
	}
	groupsByPort := make(map[int]*group)
	for _, rule := range state.Rules {
		if !rule.Enabled {
			continue
		}
		item, ok := groupsByPort[rule.ListenPort]
		if !ok {
			item = &group{port: rule.ListenPort, tls: rule.TLS}
			groupsByPort[rule.ListenPort] = item
		}
		item.rules = append(item.rules, rule)
	}
	if len(groupsByPort) == 0 {
		groupsByPort[state.Settings.DefaultHTTPPort] = &group{port: state.Settings.DefaultHTTPPort, tls: false}
	}

	ports := make([]int, 0, len(groupsByPort))
	for port := range groupsByPort {
		ports = append(ports, port)
	}
	sort.Ints(ports)

	files := make(map[string]string)
	if prelude := renderHTTPPrelude(state, m.paths.NginxCacheDir); prelude != "" {
		files["001-runtime-and-upstreams.conf"] = prelude
	}
	for _, port := range ports {
		item := groupsByPort[port]
		sort.Slice(item.rules, func(i, j int) bool {
			if item.rules[i].Name == item.rules[j].Name {
				return item.rules[i].ID < item.rules[j].ID
			}
			return item.rules[i].Name < item.rules[j].Name
		})
		hasCatchAll := false
		for _, rule := range item.rules {
			if len(rule.Domains) == 1 && rule.Domains[0] == "*" {
				hasCatchAll = true
				break
			}
		}
		if !hasCatchAll {
			defaultConf, err := m.renderDefaultServer(*item, certs)
			if err != nil {
				return "", nil, err
			}
			files[fmt.Sprintf("000-default-%05d.conf", port)] = defaultConf
		}
		for index, rule := range item.rules {
			content, err := m.renderRuleServer(rule, certs, pools)
			if err != nil {
				return "", nil, err
			}
			files[fmt.Sprintf("%03d-%05d-%s.conf", index+10, port, rule.ID)] = content
		}
	}
	if streamPrelude := renderStreamPrelude(state); streamPrelude != "" {
		files["001-stream-runtime.stream"] = streamPrelude
	}
	for index, rule := range state.StreamRules {
		if !rule.Enabled {
			continue
		}
		content, err := m.renderStreamRule(rule, certs)
		if err != nil {
			return "", nil, err
		}
		files[fmt.Sprintf("%03d-%05d-%s.stream", index+10, rule.ListenPort, rule.ID)] = content
	}

	workerProcesses := "auto"
	if state.Settings.WorkerProcesses > 0 {
		workerProcesses = strconv.Itoa(state.Settings.WorkerProcesses)
	}
	workerRlimit := ""
	if state.Settings.WorkerRlimitNofile > 0 {
		workerRlimit = fmt.Sprintf("worker_rlimit_nofile %d;\n", state.Settings.WorkerRlimitNofile)
	}
	accessLog := "access_log off;"
	if state.Settings.Logging.AccessEnabled {
		accessLog = fmt.Sprintf("access_log %s fnproxy buffer=%dk flush=%ds;", nginxQuote(m.paths.NginxAccessLog), state.Settings.Logging.AccessBufferKB, state.Settings.Logging.AccessFlushSec)
	}
	logFormat := `log_format fnproxy '$remote_addr - $remote_user [$time_local] "$request" '
                       '$status $body_bytes_sent "$http_referer" '
                       '"$http_user_agent" host="$host" upstream="$upstream_addr" '
                       'request_time=$request_time upstream_time=$upstream_response_time';`
	if state.Settings.Logging.CustomFormat != "" {
		logFormat = "log_format fnproxy " + nginxDirectiveQuote(state.Settings.Logging.CustomFormat) + ";"
	}
	aio := "off"
	if state.Settings.FileAIO {
		aio = "on"
	}
	threadPool := ""
	if state.Settings.ThreadPoolThreads > 0 {
		threadPool = fmt.Sprintf("thread_pool fnproxy threads=%d max_queue=%d;", state.Settings.ThreadPoolThreads, state.Settings.ThreadPoolQueue)
		aio = "threads=fnproxy"
	}
	multiAccept := "off"
	if state.Settings.MultiAccept {
		multiAccept = "on"
	}
	master := fmt.Sprintf(`daemon on;
master_process on;
worker_processes %s;
%s
%s

pid %s;
error_log %s %s;

events {
    worker_connections %d;
    multi_accept %s;
}

http {
    include %s;
    default_type application/octet-stream;

    %s

    %s

    server_tokens off;
    sendfile on;
    aio %s;
    tcp_nopush on;
    keepalive_timeout 65s;
    client_header_timeout 15s;
    client_body_timeout 60s;
    send_timeout 60s;

    client_body_temp_path %s;
    proxy_temp_path %s;
    fastcgi_temp_path %s;
    scgi_temp_path %s;
    uwsgi_temp_path %s;

    ssl_protocols %s;
    %s
    ssl_session_cache shared:FNPROXY_SSL:%dm;
    ssl_session_timeout %dm;
    %s

%s

    gzip %s;
    gzip_comp_level %d;
    gzip_min_length %d;
    gzip_types %s;
    gzip_static %s;
    gunzip %s;

    map $http_upgrade $connection_upgrade {
        default upgrade;
        ''      close;
    }

    include %s;
}
`, workerProcesses, workerRlimit, threadPool, nginxQuote(m.paths.NginxPID), nginxQuote(m.paths.NginxErrorLog), state.Settings.Logging.ErrorLevel, effectiveWorkerConnections(state.Settings), multiAccept, nginxQuote(m.paths.MimeTypes), logFormat, accessLog, aio, nginxQuote(filepath.Join(m.paths.NginxTempDir, "body")), nginxQuote(filepath.Join(m.paths.NginxTempDir, "proxy")), nginxQuote(filepath.Join(m.paths.NginxTempDir, "fastcgi")), nginxQuote(filepath.Join(m.paths.NginxTempDir, "scgi")), nginxQuote(filepath.Join(m.paths.NginxTempDir, "uwsgi")), strings.Join(state.Settings.TLS.Protocols, " "), renderTLSCiphers(state.Settings.TLS), state.Settings.TLS.SessionCacheMB, state.Settings.TLS.SessionTimeoutMinutes, renderTLSAdvanced(state.Settings.TLS), renderRealIP(state.Settings.RealIP), boolDirective(state.Settings.Gzip.Enabled), state.Settings.Gzip.Level, state.Settings.Gzip.MinLength, strings.Join(state.Settings.Gzip.Types, " "), boolDirective(state.Settings.Gzip.Static), boolDirective(state.Settings.Gzip.Gunzip), nginxQuote(filepath.Join(confDPath, "*.conf")))
	if hasStreamConfig(state) {
		master += fmt.Sprintf(`
stream {
    log_format fnproxy_stream '$remote_addr [$time_local] $protocol $status '
                              '$bytes_sent $bytes_received $session_time '
                              '"$upstream_addr" "$ssl_preread_server_name"';
    include %s;
}
`, nginxQuote(filepath.Join(confDPath, "*.stream")))
	}
	return master, files, nil
}

func hasStreamConfig(state State) bool {
	for _, rule := range state.StreamRules {
		if rule.Enabled {
			return true
		}
	}
	for _, pool := range state.UpstreamPools {
		if pool.Protocol == "stream" {
			return true
		}
	}
	return false
}

func streamTarget(poolID, host string, port int) string {
	if poolID != "" {
		return "fnproxy_stream_" + poolID
	}
	if net.ParseIP(host) != nil && strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func renderStreamPrelude(state State) string {
	var builder strings.Builder
	for _, pool := range state.UpstreamPools {
		if pool.Protocol != "stream" {
			continue
		}
		fmt.Fprintf(&builder, "upstream fnproxy_stream_%s {\n", pool.ID)
		switch pool.Strategy {
		case "least_conn":
			builder.WriteString("    least_conn;\n")
		case "hash":
			fmt.Fprintf(&builder, "    hash %s consistent;\n", pool.HashKey)
		case "random":
			builder.WriteString("    random two least_conn;\n")
		}
		for _, server := range pool.Servers {
			host := server.Host
			if net.ParseIP(host) != nil && strings.Contains(host, ":") {
				host = "[" + host + "]"
			}
			fmt.Fprintf(&builder, "    server %s:%d weight=%d max_fails=%d fail_timeout=%ds", host, server.Port, server.Weight, server.MaxFails, server.FailTimeout)
			if server.Backup {
				builder.WriteString(" backup")
			}
			if server.Down {
				builder.WriteString(" down")
			}
			builder.WriteString(";\n")
		}
		builder.WriteString("}\n\n")
	}
	for _, rule := range state.StreamRules {
		if !rule.Enabled {
			continue
		}
		if rule.MaxConnections > 0 {
			fmt.Fprintf(&builder, "limit_conn_zone $binary_remote_addr zone=fnproxy_stream_conn_%s:1m;\n", rule.ID)
		}
		if rule.TLSMode == "passthrough" && len(rule.SNIRoutes) > 0 {
			fmt.Fprintf(&builder, "map $ssl_preread_server_name $fnproxy_stream_backend_%s {\n", rule.ID)
			fmt.Fprintf(&builder, "    default %s;\n", streamTarget(rule.UpstreamPoolID, rule.UpstreamHost, rule.UpstreamPort))
			for _, route := range rule.SNIRoutes {
				target := streamTarget(route.UpstreamPoolID, route.UpstreamHost, route.UpstreamPort)
				for _, name := range route.ServerNames {
					fmt.Fprintf(&builder, "    %s %s;\n", name, target)
				}
			}
			builder.WriteString("}\n")
		}
	}
	return builder.String()
}

func (m *Manager) renderStreamRule(rule domain.StreamRule, certs map[string]CertificateMeta) (string, error) {
	var builder strings.Builder
	builder.WriteString("server {\n")
	address := rule.ListenAddress
	if address == "*" {
		address = ""
	}
	if net.ParseIP(address) != nil && strings.Contains(address, ":") {
		address = "[" + address + "]"
	}
	listen := fmt.Sprintf("%s:%d", address, rule.ListenPort)
	if address == "" {
		listen = strconv.Itoa(rule.ListenPort)
	}
	fmt.Fprintf(&builder, "    listen %s", listen)
	if rule.Protocol == "udp" {
		builder.WriteString(" udp reuseport")
	}
	if rule.TLSMode == "terminate" {
		builder.WriteString(" ssl")
	}
	if rule.AcceptProxyProtocol {
		builder.WriteString(" proxy_protocol")
	}
	builder.WriteString(";\n")
	if rule.TLSMode == "terminate" {
		cert, ok := certs[rule.CertificateID]
		if !ok {
			return "", fmt.Errorf("Stream 规则 %s 引用的证书不存在", rule.Name)
		}
		certFile, keyFile := m.certificateFiles(cert.ID)
		if err := requireFiles(certFile, keyFile); err != nil {
			return "", err
		}
		fmt.Fprintf(&builder, "    ssl_certificate %s;\n    ssl_certificate_key %s;\n", nginxQuote(certFile), nginxQuote(keyFile))
	}
	if rule.TLSMode == "passthrough" {
		builder.WriteString("    ssl_preread on;\n")
	}
	for _, proxy := range rule.TrustedProxies {
		fmt.Fprintf(&builder, "    set_real_ip_from %s;\n", proxy)
	}
	for _, item := range rule.Allow {
		fmt.Fprintf(&builder, "    allow %s;\n", item)
	}
	for _, item := range rule.Deny {
		fmt.Fprintf(&builder, "    deny %s;\n", item)
	}
	target := streamTarget(rule.UpstreamPoolID, rule.UpstreamHost, rule.UpstreamPort)
	if rule.TLSMode == "passthrough" && len(rule.SNIRoutes) > 0 {
		target = "$fnproxy_stream_backend_" + rule.ID
	}
	fmt.Fprintf(&builder, "    proxy_pass %s;\n", target)
	fmt.Fprintf(&builder, "    proxy_connect_timeout %ds;\n    proxy_timeout %ds;\n", rule.ConnectTimeoutSeconds, rule.ProxyTimeoutSeconds)
	if rule.Protocol == "udp" && rule.UDPResponses > 0 {
		fmt.Fprintf(&builder, "    proxy_responses %d;\n", rule.UDPResponses)
	}
	if rule.ProxyProtocol {
		builder.WriteString("    proxy_protocol on;\n")
	}
	if rule.MaxConnections > 0 {
		fmt.Fprintf(&builder, "    limit_conn fnproxy_stream_conn_%s %d;\n", rule.ID, rule.MaxConnections)
	}
	if rule.AccessLog {
		fmt.Fprintf(&builder, "    access_log %s fnproxy_stream;\n", nginxQuote(m.paths.NginxStreamLog))
	} else {
		builder.WriteString("    access_log off;\n")
	}
	builder.WriteString("}\n")
	return builder.String(), nil
}

func effectiveWorkerConnections(settings Settings) int {
	value := settings.WorkerConnections
	if settings.WorkerRlimitNofile > 0 {
		return value
	}
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err == nil && limit.Cur > 0 && uint64(value) > limit.Cur {
		return int(limit.Cur)
	}
	return value
}

func boolDirective(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func renderTLSCiphers(settings domain.TLSSettings) string {
	if strings.TrimSpace(settings.Ciphers) == "" {
		return ""
	}
	return fmt.Sprintf("ssl_ciphers %s;", nginxQuote(settings.Ciphers))
}

func renderTLSAdvanced(settings domain.TLSSettings) string {
	var lines []string
	if settings.OCSPStapling {
		lines = append(lines, "ssl_stapling on;", "ssl_stapling_verify on;")
	}
	if settings.ClientVerify != "off" {
		lines = append(lines,
			"ssl_client_certificate "+nginxQuote(settings.ClientCAFile)+";",
			"ssl_verify_client "+settings.ClientVerify+";",
			fmt.Sprintf("ssl_verify_depth %d;", settings.ClientVerifyDepth),
		)
	}
	return strings.Join(lines, "\n    ")
}

func renderRealIP(settings domain.RealIPSettings) string {
	if !settings.Enabled {
		return ""
	}
	var lines []string
	for _, proxy := range settings.TrustedProxies {
		lines = append(lines, "    set_real_ip_from "+proxy+";")
	}
	lines = append(lines, "    real_ip_header "+settings.Header+";", "    real_ip_recursive "+boolDirective(settings.Recursive)+";")
	return strings.Join(lines, "\n")
}

func renderHTTPPrelude(state State, cacheRoot string) string {
	var builder strings.Builder
	for _, item := range state.Settings.Routing.Maps {
		fmt.Fprintf(&builder, "map %s %s {\n", item.Source, item.Variable)
		if item.Hostnames {
			builder.WriteString("    hostnames;\n")
		}
		fmt.Fprintf(&builder, "    default %s;\n", nginxDirectiveQuote(item.Default))
		for _, entry := range item.Entries {
			fmt.Fprintf(&builder, "    %s %s;\n", nginxDirectiveQuote(entry.Key), nginxDirectiveQuote(entry.Value))
		}
		builder.WriteString("}\n\n")
	}
	for _, item := range state.Settings.Routing.Geos {
		source := ""
		if item.Source != "" {
			source = item.Source + " "
		}
		fmt.Fprintf(&builder, "geo %s%s {\n", source, item.Variable)
		fmt.Fprintf(&builder, "    default %s;\n", nginxDirectiveQuote(item.Default))
		for _, entry := range item.Entries {
			fmt.Fprintf(&builder, "    %s %s;\n", entry.Key, nginxDirectiveQuote(entry.Value))
		}
		builder.WriteString("}\n\n")
	}
	for _, item := range state.Settings.Routing.Splits {
		fmt.Fprintf(&builder, "split_clients %s %s {\n", nginxDirectiveQuote(item.Source), item.Variable)
		for _, entry := range item.Entries {
			fmt.Fprintf(&builder, "    %s %s;\n", entry.Key, nginxDirectiveQuote(entry.Value))
		}
		builder.WriteString("}\n\n")
	}
	for _, pool := range state.UpstreamPools {
		if pool.Protocol != "http" {
			continue
		}
		fmt.Fprintf(&builder, "upstream fnproxy_up_%s {\n", pool.ID)
		switch pool.Strategy {
		case "least_conn":
			builder.WriteString("    least_conn;\n")
		case "ip_hash":
			builder.WriteString("    ip_hash;\n")
		case "hash":
			fmt.Fprintf(&builder, "    hash %s consistent;\n", pool.HashKey)
		case "random":
			builder.WriteString("    random two least_conn;\n")
		}
		for _, server := range pool.Servers {
			host := server.Host
			if net.ParseIP(host) != nil && strings.Contains(host, ":") {
				host = "[" + host + "]"
			}
			fmt.Fprintf(&builder, "    server %s:%d weight=%d max_fails=%d fail_timeout=%ds", host, server.Port, server.Weight, server.MaxFails, server.FailTimeout)
			if server.Backup {
				builder.WriteString(" backup")
			}
			if server.Down {
				builder.WriteString(" down")
			}
			builder.WriteString(";\n")
		}
		if pool.Keepalive > 0 {
			fmt.Fprintf(&builder, "    keepalive %d;\n    keepalive_requests %d;\n    keepalive_time %ds;\n    keepalive_timeout %ds;\n", pool.Keepalive, pool.KeepaliveRequests, pool.KeepaliveTime, pool.KeepaliveTimeout)
		}
		builder.WriteString("}\n\n")
	}
	for _, rule := range state.Rules {
		if !rule.Enabled {
			continue
		}
		if rule.RateLimit.Enabled {
			fmt.Fprintf(&builder, "limit_req_zone $binary_remote_addr zone=fnproxy_req_%s:1m rate=%dr/s;\n", rule.ID, rule.RateLimit.RequestsPerSecond)
			if rule.RateLimit.Connections > 0 {
				fmt.Fprintf(&builder, "limit_conn_zone $binary_remote_addr zone=fnproxy_conn_%s:1m;\n", rule.ID)
			}
		}
		cacheItems := []struct {
			id    string
			cache domain.CacheSettings
		}{{rule.ID, rule.RootLocation.Cache}}
		for _, location := range rule.Locations {
			if !location.Enabled {
				continue
			}
			cacheItems = append(cacheItems, struct {
				id    string
				cache domain.CacheSettings
			}{rule.ID + "_" + location.ID, location.Settings.Cache})
		}
		for _, item := range cacheItems {
			if item.cache.Enabled {
				fmt.Fprintf(&builder, "proxy_cache_path %s levels=1:2 keys_zone=fnproxy_cache_%s:%dm max_size=%dm inactive=%dm use_temp_path=off;\n", nginxQuote(filepath.Join(cacheRoot, item.id)), item.id, item.cache.KeysZoneMB, item.cache.MaxSizeMB, item.cache.InactiveMinutes)
			}
		}
	}
	return builder.String()
}

func (m *Manager) renderDefaultServer(item struct {
	port  int
	tls   bool
	rules []ProxyRule
}, certs map[string]CertificateMeta) (string, error) {
	var builder strings.Builder
	builder.WriteString("server {\n")
	if item.tls {
		if len(item.rules) == 0 {
			return "", errors.New("HTTPS 监听端口缺少可用证书")
		}
		cert, ok := certs[item.rules[0].CertificateID]
		if !ok {
			return "", errors.New("默认 HTTPS 站点找不到证书")
		}
		certFile, keyFile := m.certificateFiles(cert.ID)
		if err := requireFiles(certFile, keyFile); err != nil {
			return "", err
		}
		fmt.Fprintf(&builder, "    listen %d ssl default_server;\n", item.port)
		builder.WriteString("    http2 on;\n")
		fmt.Fprintf(&builder, "    ssl_certificate %s;\n", nginxQuote(certFile))
		fmt.Fprintf(&builder, "    ssl_certificate_key %s;\n", nginxQuote(keyFile))
	} else {
		fmt.Fprintf(&builder, "    listen %d default_server;\n", item.port)
	}
	builder.WriteString("    server_name _;\n")
	builder.WriteString("    return 404;\n")
	builder.WriteString("}\n")
	return builder.String(), nil
}

func (m *Manager) renderRuleServer(rule ProxyRule, certs map[string]CertificateMeta, pools map[string]domain.UpstreamPool) (string, error) {
	var builder strings.Builder
	catchAll := len(rule.Domains) == 1 && rule.Domains[0] == "*"
	builder.WriteString("server {\n")
	listenSuffix := ""
	if rule.TLS {
		listenSuffix += " ssl"
	}
	if catchAll {
		listenSuffix += " default_server"
	}
	fmt.Fprintf(&builder, "    listen %d%s;\n", rule.ListenPort, listenSuffix)
	if rule.TLS && rule.HTTP2 {
		builder.WriteString("    http2 on;\n")
	}
	serverNames := rule.Domains
	if catchAll {
		serverNames = []string{"_"}
	}
	fmt.Fprintf(&builder, "    server_name %s;\n", strings.Join(serverNames, " "))

	if rule.TLS {
		cert, ok := certs[rule.CertificateID]
		if !ok {
			return "", fmt.Errorf("规则 %s 引用的证书不存在", rule.Name)
		}
		certFile, keyFile := m.certificateFiles(cert.ID)
		if err := requireFiles(certFile, keyFile); err != nil {
			return "", fmt.Errorf("规则 %s: %w", rule.Name, err)
		}
		fmt.Fprintf(&builder, "    ssl_certificate %s;\n", nginxQuote(certFile))
		fmt.Fprintf(&builder, "    ssl_certificate_key %s;\n", nginxQuote(keyFile))
	}

	if rule.ClientMaxBodyMB == 0 {
		builder.WriteString("    client_max_body_size 0;\n")
	} else {
		fmt.Fprintf(&builder, "    client_max_body_size %dm;\n", rule.ClientMaxBodyMB)
	}

	if rule.RootLocation.RedirectToHTTPS && !rule.TLS {
		builder.WriteString("    return 308 https://$host$request_uri;\n")
		builder.WriteString("}\n")
		return builder.String(), nil
	}

	if err := m.renderHTTPLocation(&builder, rule, "/", "prefix", rule.RootLocation, rule.ID); err != nil {
		return "", err
	}
	for _, location := range rule.Locations {
		if !location.Enabled {
			continue
		}
		if err := m.renderHTTPLocation(&builder, rule, location.Path, location.Match, location.Settings, rule.ID+"_"+location.ID); err != nil {
			return "", err
		}
	}
	builder.WriteString("}\n")
	return builder.String(), nil
}

func (m *Manager) renderHTTPLocation(builder *strings.Builder, rule ProxyRule, path, match string, settings domain.LocationSettings, cacheID string) error {
	prefix := ""
	switch match {
	case "exact":
		prefix = "= "
	case "regex":
		prefix = "~ "
	}
	fmt.Fprintf(builder, "\n    location %s%s {\n", prefix, nginxDirectiveQuote(path))
	for _, rewrite := range settings.Rewrites {
		fmt.Fprintf(builder, "        rewrite %s %s %s;\n", nginxDirectiveQuote(rewrite.Pattern), nginxDirectiveQuote(rewrite.Replacement), rewrite.Flag)
	}
	for _, address := range settings.Allow {
		fmt.Fprintf(builder, "        allow %s;\n", address)
	}
	for _, address := range settings.Deny {
		fmt.Fprintf(builder, "        deny %s;\n", address)
	}
	if settings.BasicAuth {
		fmt.Fprintf(builder, "        auth_basic %s;\n", nginxQuote(settings.BasicAuthRealm))
		fmt.Fprintf(builder, "        auth_basic_user_file %s;\n", nginxQuote(settings.BasicAuthFile))
	}
	if settings.AuthRequest != "" {
		fmt.Fprintf(builder, "        auth_request %s;\n", nginxDirectiveQuote(settings.AuthRequest))
	}
	if settings.SecureLink.Enabled {
		fmt.Fprintf(builder, "        secure_link $arg_%s,$arg_expires;\n", settings.SecureLink.Argument)
		fmt.Fprintf(builder, "        secure_link_md5 \"$secure_link_expires$uri %s\";\n", escapeNginxQuoted(settings.SecureLink.Secret))
		builder.WriteString("        if ($secure_link = \"\") { return 403; }\n")
		builder.WriteString("        if ($secure_link = \"0\") { return 410; }\n")
	}
	if settings.DAV.Enabled {
		fmt.Fprintf(builder, "        dav_methods %s;\n", strings.Join(settings.DAV.Methods, " "))
		if settings.DAV.CreateFullPutPath {
			builder.WriteString("        create_full_put_path on;\n")
		}
		fmt.Fprintf(builder, "        min_delete_depth %d;\n", settings.DAV.MinDeleteDepth)
	}
	for _, filter := range settings.SubFilters {
		fmt.Fprintf(builder, "        sub_filter %s %s;\n", nginxQuote(filter.Search), nginxQuote(filter.Replacement))
	}
	if len(settings.SubFilters) > 0 {
		fmt.Fprintf(builder, "        sub_filter_once %s;\n", onOff(settings.SubFilterOnce))
		extraTypes := make([]string, 0, len(settings.SubFilterTypes))
		for _, mimeType := range settings.SubFilterTypes {
			if mimeType != "text/html" {
				extraTypes = append(extraTypes, mimeType)
			}
		}
		if len(extraTypes) > 0 {
			fmt.Fprintf(builder, "        sub_filter_types %s;\n", strings.Join(extraTypes, " "))
		}
	}
	if settings.AdditionBefore != "" {
		fmt.Fprintf(builder, "        add_before_body %s;\n", nginxDirectiveQuote(settings.AdditionBefore))
	}
	if settings.AdditionAfter != "" {
		fmt.Fprintf(builder, "        add_after_body %s;\n", nginxDirectiveQuote(settings.AdditionAfter))
	}
	if settings.Mirror != "" {
		fmt.Fprintf(builder, "        mirror %s;\n", nginxDirectiveQuote(settings.Mirror))
		fmt.Fprintf(builder, "        mirror_request_body %s;\n", onOff(settings.MirrorRequestBody))
	}
	if settings.SSI {
		builder.WriteString("        ssi on;\n")
	}
	if len(settings.ValidReferers) > 0 {
		fmt.Fprintf(builder, "        valid_referers %s;\n", strings.Join(settings.ValidReferers, " "))
		if settings.DenyInvalidReferer {
			builder.WriteString("        if ($invalid_referer) { return 403; }\n")
		}
	}
	for _, header := range settings.ResponseHeaders {
		always := ""
		if header.Always {
			always = " always"
		}
		fmt.Fprintf(builder, "        add_header %s %s%s;\n", header.Name, nginxDirectiveQuote(header.Value), always)
	}

	switch settings.BackendType {
	case "static":
		directive := "root"
		if settings.StaticAlias {
			directive = "alias"
		}
		fmt.Fprintf(builder, "        %s %s;\n", directive, nginxQuote(settings.StaticPath))
		if len(settings.IndexFiles) > 0 {
			fmt.Fprintf(builder, "        index %s;\n", strings.Join(settings.IndexFiles, " "))
		}
		if settings.AutoIndex {
			builder.WriteString("        autoindex on;\n")
		}
		if settings.Expires != "" {
			fmt.Fprintf(builder, "        expires %s;\n", settings.Expires)
		}
		if len(settings.TryFiles) > 0 {
			quoted := make([]string, 0, len(settings.TryFiles))
			for _, value := range settings.TryFiles {
				quoted = append(quoted, nginxDirectiveQuote(value))
			}
			fmt.Fprintf(builder, "        try_files %s;\n", strings.Join(quoted, " "))
		}
	case "return":
		if settings.ReturnTarget == "" {
			fmt.Fprintf(builder, "        return %d;\n", settings.ReturnCode)
		} else {
			fmt.Fprintf(builder, "        return %d %s;\n", settings.ReturnCode, nginxDirectiveQuote(settings.ReturnTarget))
		}
	case "grpc":
		grpcScheme := "grpc"
		if settings.UpstreamScheme == "https" {
			grpcScheme = "grpcs"
		}
		fmt.Fprintf(builder, "        grpc_pass %s;\n", renderHTTPBackend(settings, grpcScheme))
	case "fastcgi":
		fmt.Fprintf(builder, "        fastcgi_pass %s;\n", renderHTTPBackend(settings, ""))
		builder.WriteString("        fastcgi_param QUERY_STRING $query_string;\n")
		builder.WriteString("        fastcgi_param REQUEST_METHOD $request_method;\n")
		builder.WriteString("        fastcgi_param CONTENT_TYPE $content_type;\n")
		builder.WriteString("        fastcgi_param CONTENT_LENGTH $content_length;\n")
		builder.WriteString("        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;\n")
		builder.WriteString("        fastcgi_param REQUEST_URI $request_uri;\n")
		builder.WriteString("        fastcgi_param SERVER_PROTOCOL $server_protocol;\n")
		builder.WriteString("        fastcgi_param REMOTE_ADDR $remote_addr;\n")
		builder.WriteString("        fastcgi_param SERVER_NAME $server_name;\n")
	case "uwsgi":
		fmt.Fprintf(builder, "        uwsgi_pass %s;\n", renderHTTPBackend(settings, ""))
		builder.WriteString("        uwsgi_param QUERY_STRING $query_string;\n")
		builder.WriteString("        uwsgi_param REQUEST_METHOD $request_method;\n")
		builder.WriteString("        uwsgi_param CONTENT_TYPE $content_type;\n")
		builder.WriteString("        uwsgi_param CONTENT_LENGTH $content_length;\n")
		builder.WriteString("        uwsgi_param REQUEST_URI $request_uri;\n")
		builder.WriteString("        uwsgi_param SERVER_PROTOCOL $server_protocol;\n")
		builder.WriteString("        uwsgi_param REMOTE_ADDR $remote_addr;\n")
		builder.WriteString("        uwsgi_param SERVER_NAME $server_name;\n")
	case "scgi":
		fmt.Fprintf(builder, "        scgi_pass %s;\n", renderHTTPBackend(settings, ""))
		builder.WriteString("        scgi_param CONTENT_LENGTH $content_length;\n")
		builder.WriteString("        scgi_param SCGI 1;\n")
		builder.WriteString("        scgi_param REQUEST_METHOD $request_method;\n")
		builder.WriteString("        scgi_param REQUEST_URI $request_uri;\n")
		builder.WriteString("        scgi_param QUERY_STRING $query_string;\n")
		builder.WriteString("        scgi_param SERVER_PROTOCOL $server_protocol;\n")
		builder.WriteString("        scgi_param REMOTE_ADDR $remote_addr;\n")
	case "memcached":
		builder.WriteString("        memcached_key $uri;\n")
		fmt.Fprintf(builder, "        memcached_pass %s;\n", renderHTTPBackend(settings, ""))
	case "status":
		builder.WriteString("        stub_status;\n")
	default:
		fmt.Fprintf(builder, "        proxy_pass %s;\n", renderHTTPBackend(settings, settings.UpstreamScheme))
		m.renderProxySettings(builder, rule, settings, cacheID)
	}
	builder.WriteString("    }\n")
	return nil
}

func renderHTTPBackend(settings domain.LocationSettings, scheme string) string {
	if settings.UpstreamPoolID != "" {
		backend := "fnproxy_up_" + settings.UpstreamPoolID
		if scheme != "" {
			return scheme + "://" + backend
		}
		return backend
	}
	host := settings.UpstreamHost
	if net.ParseIP(host) != nil && strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	backend := fmt.Sprintf("%s:%d", host, settings.UpstreamPort)
	if scheme != "" {
		return scheme + "://" + backend
	}
	return backend
}

func (m *Manager) renderProxySettings(builder *strings.Builder, rule ProxyRule, settings domain.LocationSettings, cacheID string) {
	builder.WriteString("        proxy_http_version 1.1;\n")
	if rule.PreserveHost {
		builder.WriteString("        proxy_set_header Host $host;\n")
	} else {
		builder.WriteString("        proxy_set_header Host $proxy_host;\n")
	}
	builder.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
	builder.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
	builder.WriteString("        proxy_set_header X-Forwarded-Proto $scheme;\n")
	builder.WriteString("        proxy_set_header X-Forwarded-Host $host;\n")
	builder.WriteString("        proxy_set_header X-Forwarded-Port $server_port;\n")
	if rule.WebSocket {
		builder.WriteString("        proxy_set_header Upgrade $http_upgrade;\n")
		builder.WriteString("        proxy_set_header Connection $connection_upgrade;\n")
	} else {
		builder.WriteString("        proxy_set_header Connection \"\";\n")
	}
	for _, header := range settings.RequestHeaders {
		fmt.Fprintf(builder, "        proxy_set_header %s %s;\n", header.Name, nginxDirectiveQuote(header.Value))
	}
	fmt.Fprintf(builder, "        proxy_connect_timeout %ds;\n", rule.ConnectTimeoutSeconds)
	fmt.Fprintf(builder, "        proxy_read_timeout %ds;\n", rule.ReadTimeoutSeconds)
	fmt.Fprintf(builder, "        proxy_send_timeout %ds;\n", rule.SendTimeoutSeconds)
	if rule.Streaming || rule.WebSocket {
		builder.WriteString("        proxy_buffering off;\n")
	}
	if rule.Streaming {
		builder.WriteString("        proxy_request_buffering off;\n")
	}
	if rule.RateLimit.Enabled {
		fmt.Fprintf(builder, "        limit_req zone=fnproxy_req_%s burst=%d%s;\n", rule.ID, rule.RateLimit.Burst, map[bool]string{true: " nodelay", false: ""}[rule.RateLimit.NoDelay])
		if rule.RateLimit.Connections > 0 {
			fmt.Fprintf(builder, "        limit_conn fnproxy_conn_%s %d;\n", rule.ID, rule.RateLimit.Connections)
		}
		if rule.RateLimit.DownloadKBps > 0 {
			fmt.Fprintf(builder, "        limit_rate %dk;\n", rule.RateLimit.DownloadKBps)
		}
	}
	if settings.Cache.Enabled {
		fmt.Fprintf(builder, "        proxy_cache fnproxy_cache_%s;\n", cacheID)
		fmt.Fprintf(builder, "        proxy_cache_valid %ds;\n", settings.Cache.ValidSeconds)
		if settings.Cache.SliceKB == 0 {
			fmt.Fprintf(builder, "        proxy_cache_key %s;\n", nginxDirectiveQuote(settings.Cache.Key))
		}
		if len(settings.Cache.Bypass) > 0 {
			fmt.Fprintf(builder, "        proxy_cache_bypass %s;\n", strings.Join(settings.Cache.Bypass, " "))
			fmt.Fprintf(builder, "        proxy_no_cache %s;\n", strings.Join(settings.Cache.Bypass, " "))
		}
		if settings.Cache.UseStale {
			builder.WriteString("        proxy_cache_use_stale error timeout updating http_500 http_502 http_503 http_504;\n")
		}
		if settings.Cache.SliceKB > 0 {
			fmt.Fprintf(builder, "        slice %dk;\n", settings.Cache.SliceKB)
			builder.WriteString("        proxy_set_header Range $slice_range;\n")
			builder.WriteString("        proxy_cache_key \"$scheme$request_method$host$request_uri$slice_range\";\n")
		}
	}
	if settings.UpstreamScheme == "https" {
		builder.WriteString("        proxy_ssl_server_name on;\n")
		sslName := settings.UpstreamHost
		if settings.UpstreamPoolID != "" {
			sslName = "$host"
		}
		fmt.Fprintf(builder, "        proxy_ssl_name %s;\n", sslName)
		if rule.VerifyUpstreamTLS {
			caBundle := "/etc/ssl/certs/ca-certificates.crt"
			fmt.Fprintf(builder, "        proxy_ssl_trusted_certificate %s;\n", nginxQuote(caBundle))
			builder.WriteString("        proxy_ssl_verify on;\n")
			builder.WriteString("        proxy_ssl_verify_depth 5;\n")
		} else {
			builder.WriteString("        proxy_ssl_verify off;\n")
		}
	}
}

func (m *Manager) certificateFiles(id string) (string, string) {
	base := filepath.Join(m.paths.CertificateDir, id)
	return filepath.Join(base, "fullchain.pem"), filepath.Join(base, "privkey.pem")
}

func requireFiles(paths ...string) error {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("缺少文件 %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s 不是普通文件", path)
		}
	}
	return nil
}

func nginxQuote(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`)
	return `"` + replacer.Replace(value) + `"`
}

func nginxDirectiveQuote(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + replacer.Replace(value) + `"`
}

func escapeNginxQuoted(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`).Replace(value)
}

func onOff(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func ensureTrailingSlash(path string) string {
	if strings.HasSuffix(path, string(os.PathSeparator)) {
		return path
	}
	return path + string(os.PathSeparator)
}
