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
	if err := domain.ValidateState(state); err != nil {
		return "", nil, err
	}
	certs := make(map[string]CertificateMeta, len(state.Certificates))
	for _, cert := range state.Certificates {
		certs[cert.ID] = cert
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
			content, err := m.renderRuleServer(rule, certs)
			if err != nil {
				return "", nil, err
			}
			files[fmt.Sprintf("%03d-%05d-%s.conf", index+10, port, rule.ID)] = content
		}
	}

	master := fmt.Sprintf(`daemon on;
master_process on;
worker_processes auto;

pid %s;
error_log %s notice;

events {
    worker_connections 4096;
    multi_accept on;
}

http {
    include %s;
    default_type application/octet-stream;

    log_format fnproxy '$remote_addr - $remote_user [$time_local] "$request" '
                       '$status $body_bytes_sent "$http_referer" '
                       '"$http_user_agent" host="$host" upstream="$upstream_addr" '
                       'request_time=$request_time upstream_time=$upstream_response_time';

    access_log %s fnproxy;

    server_tokens off;
    sendfile on;
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

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_session_cache shared:FNPROXY_SSL:10m;
    ssl_session_timeout 10m;

    map $http_upgrade $connection_upgrade {
        default upgrade;
        ''      close;
    }

    include %s;
}
`, nginxQuote(m.paths.NginxPID), nginxQuote(m.paths.NginxErrorLog), nginxQuote(m.paths.MimeTypes), nginxQuote(m.paths.NginxAccessLog), nginxQuote(filepath.Join(m.paths.NginxTempDir, "body")), nginxQuote(filepath.Join(m.paths.NginxTempDir, "proxy")), nginxQuote(filepath.Join(m.paths.NginxTempDir, "fastcgi")), nginxQuote(filepath.Join(m.paths.NginxTempDir, "scgi")), nginxQuote(filepath.Join(m.paths.NginxTempDir, "uwsgi")), nginxQuote(filepath.Join(confDPath, "*.conf")))
	return master, files, nil
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

func (m *Manager) renderRuleServer(rule ProxyRule, certs map[string]CertificateMeta) (string, error) {
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

	upstreamHost := rule.UpstreamHost
	if net.ParseIP(upstreamHost) != nil && strings.Contains(upstreamHost, ":") {
		upstreamHost = "[" + upstreamHost + "]"
	}
	fmt.Fprintf(&builder, "\n    location / {\n")
	fmt.Fprintf(&builder, "        proxy_pass %s://%s:%d;\n", rule.UpstreamScheme, upstreamHost, rule.UpstreamPort)
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
	fmt.Fprintf(&builder, "        proxy_connect_timeout %ds;\n", rule.ConnectTimeoutSeconds)
	fmt.Fprintf(&builder, "        proxy_read_timeout %ds;\n", rule.ReadTimeoutSeconds)
	fmt.Fprintf(&builder, "        proxy_send_timeout %ds;\n", rule.SendTimeoutSeconds)
	if rule.Streaming || rule.WebSocket {
		builder.WriteString("        proxy_buffering off;\n")
	}
	if rule.Streaming {
		builder.WriteString("        proxy_request_buffering off;\n")
	}
	if rule.UpstreamScheme == "https" {
		builder.WriteString("        proxy_ssl_server_name on;\n")
		fmt.Fprintf(&builder, "        proxy_ssl_name %s;\n", rule.UpstreamHost)
		if rule.VerifyUpstreamTLS {
			caBundle := "/etc/ssl/certs/ca-certificates.crt"
			if _, err := os.Stat(caBundle); err != nil {
				return "", errors.New("启用上游 TLS 校验时未找到系统 CA 证书包")
			}
			fmt.Fprintf(&builder, "        proxy_ssl_trusted_certificate %s;\n", nginxQuote(caBundle))
			builder.WriteString("        proxy_ssl_verify on;\n")
			builder.WriteString("        proxy_ssl_verify_depth 5;\n")
		} else {
			builder.WriteString("        proxy_ssl_verify off;\n")
		}
	}
	builder.WriteString("    }\n")
	builder.WriteString("}\n")
	return builder.String(), nil
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

func ensureTrailingSlash(path string) string {
	if strings.HasSuffix(path, string(os.PathSeparator)) {
		return path
	}
	return path + string(os.PathSeparator)
}
