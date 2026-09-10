package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	acmemanager "github.com/chenpingonline/nginx-web-fnos/internal/acme"
	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/metrics"
	appservice "github.com/chenpingonline/nginx-web-fnos/internal/service"
)

const gatewayPrefix = "/app/nginx-web"

type API struct {
	service *appservice.AppService
	web     fs.FS
	devMode bool
}

func New(service *appservice.AppService, web fs.FS) *API {
	return &API{
		service: service,
		web:     web,
		devMode: os.Getenv("FNPROXY_DEV_ALLOW") == "1",
	}
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.setSecurityHeaders(w)
	if r.URL.Path == gatewayPrefix {
		http.Redirect(w, r, gatewayPrefix+"/", http.StatusTemporaryRedirect)
		return
	}
	cleanPath := r.URL.Path
	if strings.HasPrefix(cleanPath, gatewayPrefix+"/") {
		cleanPath = strings.TrimPrefix(cleanPath, gatewayPrefix)
	}
	if cleanPath == "" {
		cleanPath = "/"
	}
	if cleanPath == "/healthz" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": domain.AppVersion})
		return
	}
	if strings.HasPrefix(cleanPath, "/api/") || cleanPath == "/api" {
		a.handleAPI(w, r, cleanPath)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeAPIError(w, http.StatusMethodNotAllowed, "仅支持 GET 或 HEAD")
		return
	}
	a.serveWeb(w, r, cleanPath)
}

func (a *API) setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'self'")
}

func (a *API) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if a.devMode {
		return true
	}
	value := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Trim-Isadmin")))
	if value == "true" || value == "1" || value == "yes" {
		return true
	}
	writeAPIError(w, http.StatusForbidden, "仅飞牛管理员可以管理反向代理")
	return false
}

func (a *API) requireMutation(w http.ResponseWriter, r *http.Request) bool {
	if !a.requireAdmin(w, r) {
		return false
	}
	if a.devMode {
		return true
	}
	if r.Header.Get("X-FnProxy-Request") != "1" {
		writeAPIError(w, http.StatusForbidden, "缺少管理请求标识")
		return false
	}
	return true
}

func (a *API) handleAPI(w http.ResponseWriter, r *http.Request, apiPath string) {
	if r.Method == http.MethodGet {
		if !a.requireAdmin(w, r) {
			return
		}
	} else if !a.requireMutation(w, r) {
		return
	}

	switch {
	case apiPath == "/api/dashboard" && r.Method == http.MethodGet:
		minutes := 60
		rawMinutes := r.URL.Query().Get("minutes")
		if rawMinutes != "" {
			parsed, err := strconv.Atoi(rawMinutes)
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "时间范围必须为 15 分钟、1 小时、5 小时、1 天、7 天或 1 月（30 天）")
				return
			}
			minutes = parsed
		}
		if !metrics.ValidRange(minutes) {
			writeAPIError(w, http.StatusBadRequest, "时间范围必须为 15 分钟、1 小时、5 小时、1 天、7 天或 1 月（30 天）")
			return
		}
		writeJSON(w, http.StatusOK, a.service.Dashboard(minutes, r.URL.Query().Get("rule")))
	case apiPath == "/api/overview" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, a.service.Overview())
	case apiPath == "/api/state" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, a.service.State())
	case apiPath == "/api/rule-groups" && r.Method == http.MethodPost:
		var input domain.RuleGroup
		if !decodeJSON(w, r, &input) {
			return
		}
		group, err := a.service.SaveRuleGroup("", input)
		writeResult(w, http.StatusCreated, group, err)
	case strings.HasPrefix(apiPath, "/api/rule-groups/"):
		id := strings.TrimPrefix(apiPath, "/api/rule-groups/")
		if r.Method == http.MethodPut {
			var input domain.RuleGroup
			if !decodeJSON(w, r, &input) {
				return
			}
			group, err := a.service.SaveRuleGroup(id, input)
			writeResult(w, http.StatusOK, group, err)
		} else if r.Method == http.MethodDelete {
			err := a.service.DeleteRuleGroup(id)
			writeResult(w, http.StatusOK, map[string]bool{"ok": err == nil}, err)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	case apiPath == "/api/rules" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, a.service.State().Rules)
	case apiPath == "/api/upstreams" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, a.service.State().UpstreamPools)
	case apiPath == "/api/upstreams" && r.Method == http.MethodPost:
		var input domain.UpstreamPool
		if !decodeJSON(w, r, &input) {
			return
		}
		pool, err := a.service.CreateUpstreamPool(input)
		writeResult(w, http.StatusCreated, pool, err)
	case strings.HasPrefix(apiPath, "/api/upstreams/"):
		a.handleUpstreamPool(w, r, strings.TrimPrefix(apiPath, "/api/upstreams/"))
	case apiPath == "/api/rate-limit-policies" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, a.service.State().RateLimitPolicies)
	case apiPath == "/api/rate-limit-policies" && r.Method == http.MethodPost:
		var input domain.RateLimitPolicy
		if !decodeJSON(w, r, &input) {
			return
		}
		policy, err := a.service.CreateRateLimitPolicy(input)
		writeResult(w, http.StatusCreated, policy, err)
	case strings.HasPrefix(apiPath, "/api/rate-limit-policies/"):
		a.handleRateLimitPolicy(w, r, strings.TrimPrefix(apiPath, "/api/rate-limit-policies/"))
	case apiPath == "/api/streams" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, a.service.State().StreamRules)
	case apiPath == "/api/streams" && r.Method == http.MethodPost:
		var input domain.StreamRule
		if !decodeJSON(w, r, &input) {
			return
		}
		rule, err := a.service.CreateStreamRule(input)
		writeResult(w, http.StatusCreated, rule, err)
	case strings.HasPrefix(apiPath, "/api/streams/"):
		a.handleStreamRule(w, r, strings.TrimPrefix(apiPath, "/api/streams/"))
	case apiPath == "/api/rules" && r.Method == http.MethodPost:
		var input domain.ProxyRule
		if !decodeJSON(w, r, &input) {
			return
		}
		rule, err := a.service.CreateRule(input)
		writeResult(w, http.StatusCreated, rule, err)
	case strings.HasPrefix(apiPath, "/api/rules/"):
		a.handleRule(w, r, strings.TrimPrefix(apiPath, "/api/rules/"))
	case apiPath == "/api/acme/providers" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, acmemanager.Providers())
	case apiPath == "/api/acme" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, a.service.ACME().List())
	case apiPath == "/api/acme" && r.Method == http.MethodPost:
		var input acmemanager.Input
		if !decodeJSON(w, r, &input) {
			return
		}
		job, err := a.service.ACME().Create(input)
		writeResult(w, http.StatusAccepted, job, err)
	case strings.HasPrefix(apiPath, "/api/acme/") && (r.Method == http.MethodPost || r.Method == http.MethodDelete):
		parts := strings.Split(strings.TrimPrefix(apiPath, "/api/acme/"), "/")
		action := "delete"
		if r.Method == http.MethodPost {
			if len(parts) != 2 {
				writeAPIError(w, http.StatusBadRequest, "任务操作无效")
				return
			}
			action = parts[1]
		} else if len(parts) != 1 {
			writeAPIError(w, http.StatusBadRequest, "任务操作无效")
			return
		}
		if r.Method == http.MethodPost && action == "reissue" {
			var input struct {
				Domains []string `json:"domains"`
			}
			if !decodeJSON(w, r, &input) {
				return
			}
			job, err := a.service.ACME().Reissue(parts[0], input.Domains)
			writeResult(w, http.StatusAccepted, job, err)
			return
		}
		err := a.service.ACME().Action(parts[0], action)
		writeResult(w, http.StatusOK, map[string]bool{"ok": err == nil}, err)
	case apiPath == "/api/certificates" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, a.service.State().Certificates)
	case apiPath == "/api/certificates" && r.Method == http.MethodPost:
		var input appservice.CertificateInput
		if !decodeJSON(w, r, &input) {
			return
		}
		certificate, err := a.service.ImportCertificate(input)
		writeResult(w, http.StatusCreated, certificate, err)
	case strings.HasPrefix(apiPath, "/api/certificates/") && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(apiPath, "/api/certificates/")
		err := a.service.DeleteCertificate(id)
		writeResult(w, http.StatusOK, map[string]any{"ok": err == nil}, err)
	case apiPath == "/api/backup" && r.Method == http.MethodGet:
		backup, err := a.service.ExportBackup()
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"nginx-web-backup-%s.json\"", time.Now().UTC().Format("20060102-150405")))
		writeJSON(w, http.StatusOK, backup)
	case apiPath == "/api/backup/restore" && r.Method == http.MethodPost:
		var backup appservice.Backup
		if !decodeJSONLimit(w, r, &backup, appservice.MaxBackupBytes) {
			return
		}
		state, err := a.service.RestoreBackup(backup)
		writeResult(w, http.StatusOK, state, err)
	case apiPath == "/api/settings" && r.Method == http.MethodPut:
		var settings domain.Settings
		if !decodeJSON(w, r, &settings) {
			return
		}
		err := a.service.UpdateSettings(settings)
		writeResult(w, http.StatusOK, a.service.State().Settings, err)
	case apiPath == "/api/settings/test" && r.Method == http.MethodPost:
		var settings domain.Settings
		if !decodeJSON(w, r, &settings) {
			return
		}
		result, err := a.service.TestSettings(settings)
		writeResult(w, http.StatusOK, result, err)
	case apiPath == "/api/cache" && r.Method == http.MethodDelete:
		err := a.service.ClearCache()
		writeResult(w, http.StatusOK, map[string]any{"ok": err == nil}, err)
	case apiPath == "/api/apply" && r.Method == http.MethodPost:
		var input struct {
			Summary string `json:"summary"`
		}
		if r.ContentLength != 0 && !decodeJSON(w, r, &input) {
			return
		}
		result, err := a.service.Apply(input.Summary)
		writeResult(w, http.StatusOK, result, err)
	case apiPath == "/api/nginx/start" && r.Method == http.MethodPost:
		result, err := a.service.NginxStart()
		writeResult(w, http.StatusOK, result, err)
	case apiPath == "/api/nginx/stop" && r.Method == http.MethodPost:
		result, err := a.service.NginxStop()
		writeResult(w, http.StatusOK, result, err)
	case apiPath == "/api/nginx/reload" && r.Method == http.MethodPost:
		result, err := a.service.NginxReload()
		writeResult(w, http.StatusOK, result, err)
	case apiPath == "/api/nginx/test" && r.Method == http.MethodPost:
		result, err := a.service.NginxTest()
		writeResult(w, http.StatusOK, result, err)
	case apiPath == "/api/logs" && r.Method == http.MethodGet:
		kind := r.URL.Query().Get("type")
		if kind == "" {
			kind = "error"
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("lines"))
		if limit == 0 {
			limit = 300
		}
		lines, err := a.service.Logs(kind, limit)
		writeResult(w, http.StatusOK, map[string]any{"type": kind, "lines": lines}, err)
	case apiPath == "/api/logs/rotate" && r.Method == http.MethodPost:
		rotated, err := a.service.RotateLogs(true)
		writeResult(w, http.StatusOK, map[string]any{"rotated": rotated}, err)
	case apiPath == "/api/revisions" && r.Method == http.MethodGet:
		revisions, err := a.service.ListRevisions()
		writeResult(w, http.StatusOK, revisions, err)
	case strings.HasPrefix(apiPath, "/api/revisions/"):
		a.handleRevision(w, r, strings.TrimPrefix(apiPath, "/api/revisions/"))
	case apiPath == "/api/config" && r.Method == http.MethodGet:
		config, err := a.service.Config()
		writeResult(w, http.StatusOK, config, err)
	default:
		writeAPIError(w, http.StatusNotFound, "接口不存在")
	}
}

func (a *API) handleRateLimitPolicy(w http.ResponseWriter, r *http.Request, id string) {
	if strings.Contains(id, "/") || id == "" {
		writeAPIError(w, http.StatusNotFound, "限流策略不存在")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var input domain.RateLimitPolicy
		if !decodeJSON(w, r, &input) {
			return
		}
		policy, err := a.service.UpdateRateLimitPolicy(id, input)
		writeResult(w, http.StatusOK, policy, err)
	case http.MethodDelete:
		err := a.service.DeleteRateLimitPolicy(id)
		writeResult(w, http.StatusOK, map[string]any{"ok": err == nil}, err)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "限流策略接口不支持该请求方法")
	}
}

func (a *API) handleUpstreamPool(w http.ResponseWriter, r *http.Request, id string) {
	if strings.Contains(id, "/") || id == "" {
		writeAPIError(w, http.StatusNotFound, "后端服务组不存在")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var input domain.UpstreamPool
		if !decodeJSON(w, r, &input) {
			return
		}
		pool, err := a.service.UpdateUpstreamPool(id, input)
		writeResult(w, http.StatusOK, pool, err)
	case http.MethodDelete:
		err := a.service.DeleteUpstreamPool(id)
		writeResult(w, http.StatusOK, map[string]any{"ok": err == nil}, err)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "后端服务组接口不支持该请求方法")
	}
}

func (a *API) handleStreamRule(w http.ResponseWriter, r *http.Request, id string) {
	if strings.Contains(id, "/") || id == "" {
		writeAPIError(w, http.StatusNotFound, "Stream 规则不存在")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var input domain.StreamRule
		if !decodeJSON(w, r, &input) {
			return
		}
		rule, err := a.service.UpdateStreamRule(id, input)
		writeResult(w, http.StatusOK, rule, err)
	case http.MethodDelete:
		err := a.service.DeleteStreamRule(id)
		writeResult(w, http.StatusOK, map[string]any{"ok": err == nil}, err)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "Stream 规则接口不支持该请求方法")
	}
}

func (a *API) handleRule(w http.ResponseWriter, r *http.Request, id string) {
	if strings.Contains(id, "/") || id == "" {
		writeAPIError(w, http.StatusNotFound, "规则不存在")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var input domain.ProxyRule
		if !decodeJSON(w, r, &input) {
			return
		}
		rule, err := a.service.UpdateRule(id, input)
		writeResult(w, http.StatusOK, rule, err)
	case http.MethodDelete:
		err := a.service.DeleteRule(id)
		writeResult(w, http.StatusOK, map[string]any{"ok": err == nil}, err)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "规则接口不支持该请求方法")
	}
}

func (a *API) handleRevision(w http.ResponseWriter, r *http.Request, suffix string) {
	parts := strings.Split(strings.Trim(suffix, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeAPIError(w, http.StatusNotFound, "配置历史不存在")
		return
	}
	id := parts[0]
	if len(parts) == 2 && parts[1] == "restore" && r.Method == http.MethodPost {
		state, err := a.service.RestoreRevision(id)
		writeResult(w, http.StatusOK, state, err)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		err := a.service.DeleteRevision(id)
		writeResult(w, http.StatusOK, map[string]any{"ok": err == nil}, err)
		return
	}
	writeAPIError(w, http.StatusNotFound, "配置历史操作不存在")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	return decodeJSONLimit(w, r, target, 3*1024*1024)
}
func decodeJSONLimit(w http.ResponseWriter, r *http.Request, target any, limit int64) bool {
	if r.Body == nil {
		writeAPIError(w, http.StatusBadRequest, "请求体不能为空")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeAPIError(w, http.StatusBadRequest, "请求内容不正确: "+err.Error())
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeAPIError(w, http.StatusBadRequest, "请求体只能包含一个 JSON 对象")
		return false
	}
	return true
}

func writeResult(w http.ResponseWriter, status int, value any, err error) {
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, status, value)
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (a *API) serveWeb(w http.ResponseWriter, r *http.Request, requestPath string) {
	name := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	if name == "" || name == "." {
		name = "index.html"
	}
	data, err := fs.ReadFile(a.web, name)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "读取页面失败", http.StatusInternalServerError)
			return
		}
		name = "index.html"
		data, err = fs.ReadFile(a.web, name)
		if err != nil {
			http.Error(w, "页面不存在", http.StatusNotFound)
			return
		}
	}
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	w.Header().Set("Content-Type", contentType)
	if name == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	http.ServeContent(w, r, name, time.Time{}, strings.NewReader(string(data)))
}

func apiBaseURL() string {
	return gatewayPrefix + "/api"
}

func formatHTTPError(resp *http.Response) error {
	defer resp.Body.Close()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&body); err == nil && body.Error != "" {
		return errors.New(body.Error)
	}
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}
