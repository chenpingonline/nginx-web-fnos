// Package adminauth authenticates the independent management HTTP listener.
// fnOS gateway headers and development bypasses are never trusted here.
package adminauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/fileutil"
	"golang.org/x/crypto/bcrypt"
)

const cookieName = "nginx_web_session"
const sessionLifetime = 12 * time.Hour

type credentials struct {
	Username string `json:"username"`
	Hash     string `json:"password_hash"`
}

type session struct {
	CSRF    string
	Expires time.Time
}

type attempts struct {
	Count int
	Until time.Time
}

type Manager struct {
	credentials credentials
	secure      bool
	mu          sync.Mutex
	sessions    map[[32]byte]session
	attempts    map[string]attempts
	loginSlots  chan struct{}
	now         func() time.Time
}

type authenticatedKey struct{}

func Authenticated(ctx context.Context) bool {
	value, _ := ctx.Value(authenticatedKey{}).(bool)
	return value
}

// Passwords are only read at startup. Persisted hashes allow the bootstrap
// secret to be removed; supplying a new secret resets the administrator.
func Load(varDir string) (*Manager, error) {
	path := filepath.Join(varDir, "admin.json")
	var stored credentials
	data, err := os.ReadFile(path)
	if err == nil {
		if json.Unmarshal(data, &stored) != nil || domain.ValidateAuthUsername(stored.Username) != nil {
			return nil, errors.New("管理员账户文件无效")
		}
		if _, err := bcrypt.Cost([]byte(stored.Hash)); err != nil {
			return nil, errors.New("管理员密码摘要无效")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("读取管理员账户失败: %w", err)
	}
	password := os.Getenv("FNPROXY_ADMIN_PASSWORD")
	if secretFile := os.Getenv("FNPROXY_ADMIN_PASSWORD_FILE"); secretFile != "" {
		if password != "" {
			return nil, errors.New("管理员密码和密码文件不能同时设置")
		}
		secret, err := os.Open(secretFile)
		if err != nil {
			return nil, fmt.Errorf("读取管理员密码文件失败: %w", err)
		}
		data, err := io.ReadAll(io.LimitReader(secret, 75))
		secret.Close()
		if err != nil {
			return nil, fmt.Errorf("读取管理员密码文件失败: %w", err)
		}
		password = strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
		if password == "" {
			return nil, errors.New("管理员密码文件不能为空")
		}
	}
	username := strings.TrimSpace(os.Getenv("FNPROXY_ADMIN_USER"))
	if username == "" {
		username = stored.Username
		if username == "" {
			username = "admin"
		}
	}
	if err := domain.ValidateAuthUsername(username); err != nil {
		return nil, err
	}
	if password != "" {
		if len(password) < 8 || len(password) > 72 {
			return nil, errors.New("管理员密码长度必须为 8–72 字节")
		}
		if stored.Username != username || bcrypt.CompareHashAndPassword([]byte(stored.Hash), []byte(password)) != nil {
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return nil, err
			}
			stored = credentials{Username: username, Hash: string(hash)}
			data, _ := json.MarshalIndent(stored, "", "  ")
			if err := fileutil.WriteFileAtomic(path, append(data, '\n'), 0o600); err != nil {
				return nil, fmt.Errorf("保存管理员账户失败: %w", err)
			}
		}
	} else if stored.Hash == "" {
		return nil, errors.New("首次启动需要 FNPROXY_ADMIN_PASSWORD 或 FNPROXY_ADMIN_PASSWORD_FILE；没有默认密码")
	} else if username != stored.Username {
		return nil, errors.New("修改管理员名称时必须同时设置管理员密码")
	}
	secure := os.Getenv("FNPROXY_COOKIE_SECURE")
	if secure != "" && secure != "0" && secure != "1" {
		return nil, errors.New("FNPROXY_COOKIE_SECURE 必须是 0 或 1")
	}
	return &Manager{credentials: stored, secure: secure == "1", sessions: make(map[[32]byte]session),
		attempts: make(map[string]attempts), loginSlots: make(chan struct{}, 2), now: time.Now}, nil
}

func (m *Manager) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "same-origin")
		path := r.URL.Path
		if strings.HasPrefix(path, "/app/nginx-web/") {
			path = strings.TrimPrefix(path, "/app/nginx-web")
		}
		// These callbacks are for the private Unix socket only.
		if strings.HasPrefix(path, "/internal/") {
			respond(w, http.StatusNotFound, map[string]string{"error": "页面不存在"})
			return
		}
		current, valid := m.session(r)
		switch path {
		case "/api/auth/session":
			if r.Method != http.MethodGet {
				respond(w, http.StatusMethodNotAllowed, nil)
				return
			}
			m.respondSession(w, current, valid)
			return
		case "/api/auth/login":
			m.login(w, r)
			return
		}
		if strings.HasPrefix(path, "/api/") || path == "/api" {
			if !valid {
				respond(w, http.StatusUnauthorized, map[string]string{"error": "请登录管理员账户"})
				return
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				if !m.sameOrigin(r) || r.Header.Get("X-FnProxy-Request") != "1" ||
					subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(current.CSRF)) != 1 {
					respond(w, http.StatusForbidden, map[string]string{"error": "请求校验失败，请刷新页面重试"})
					return
				}
			}
			if path == "/api/auth/logout" {
				if r.Method != http.MethodPost {
					respond(w, http.StatusMethodNotAllowed, nil)
					return
				}
				m.revoke(r)
				m.setCookie(w, r, "", -1)
				respond(w, http.StatusOK, map[string]bool{"ok": true})
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), authenticatedKey{}, true))
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Manager) sameOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		scheme := "http"
		if r.TLS != nil || m.secure {
			scheme = "https"
		}
		return err == nil && u.Host == r.Host && u.Scheme == scheme && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == ""
	}
	return true
}

func (m *Manager) session(r *http.Request) (session, bool) {
	cookie, err := r.Cookie(cookieName)
	if err != nil || len(cookie.Value) != 43 {
		return session{}, false
	}
	key := sha256.Sum256([]byte(cookie.Value))
	m.mu.Lock()
	defer m.mu.Unlock()
	current, found := m.sessions[key]
	if found && !m.now().Before(current.Expires) {
		delete(m.sessions, key)
		return session{}, false
	}
	return current, found
}

func (m *Manager) revoke(r *http.Request) {
	if cookie, err := r.Cookie(cookieName); err == nil {
		m.mu.Lock()
		delete(m.sessions, sha256.Sum256([]byte(cookie.Value)))
		m.mu.Unlock()
	}
}

func (m *Manager) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respond(w, http.StatusMethodNotAllowed, nil)
		return
	}
	if r.Header.Get("X-FnProxy-Request") != "1" || !m.sameOrigin(r) {
		respond(w, http.StatusForbidden, map[string]string{"error": "请求校验失败，请刷新页面重试"})
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !m.allowAttempt(ip) {
		w.Header().Set("Retry-After", "60")
		respond(w, http.StatusTooManyRequests, map[string]string{"error": "登录尝试过于频繁，请稍后重试"})
		return
	}
	select {
	case m.loginSlots <- struct{}{}:
		defer func() { <-m.loginSlots }()
	default:
		respond(w, http.StatusTooManyRequests, map[string]string{"error": "登录请求繁忙，请稍后重试"})
		return
	}
	var input struct{ Username, Password string }
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF || len(input.Password) > 72 {
		respond(w, http.StatusBadRequest, map[string]string{"error": "登录信息格式不正确"})
		return
	}
	passwordOK := bcrypt.CompareHashAndPassword([]byte(m.credentials.Hash), []byte(input.Password)) == nil
	usernameOK := subtle.ConstantTimeCompare([]byte(input.Username), []byte(m.credentials.Username)) == 1
	if !passwordOK || !usernameOK {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "用户名或密码错误"})
		return
	}
	token, err := randomToken()
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "无法创建会话"})
		return
	}
	csrf, err := randomToken()
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "无法创建会话"})
		return
	}
	m.revoke(r)
	current := session{CSRF: csrf, Expires: m.now().Add(sessionLifetime)}
	m.mu.Lock()
	for key, value := range m.sessions {
		if !m.now().Before(value.Expires) {
			delete(m.sessions, key)
		}
	}
	if len(m.sessions) >= 128 {
		var oldestKey [32]byte
		oldest := current.Expires
		for key, value := range m.sessions {
			if !value.Expires.After(oldest) {
				oldestKey, oldest = key, value.Expires
			}
		}
		delete(m.sessions, oldestKey)
	}
	m.sessions[sha256.Sum256([]byte(token))] = current
	m.mu.Unlock()
	m.setCookie(w, r, token, int(sessionLifetime.Seconds()))
	m.respondSession(w, current, true)
}

func (m *Manager) allowAttempt(ip string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	for key, value := range m.attempts {
		if !now.Before(value.Until) {
			delete(m.attempts, key)
		}
	}
	value := m.attempts[ip]
	if value.Count >= 10 || (value.Count == 0 && len(m.attempts) >= 1024) {
		return false
	}
	if value.Count == 0 {
		value.Until = now.Add(time.Minute)
	}
	value.Count++
	m.attempts[ip] = value
	return true
}

func (m *Manager) setCookie(w http.ResponseWriter, r *http.Request, token string, maxAge int) {
	cookie := &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true,
		Secure: m.secure || r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: maxAge}
	if maxAge > 0 {
		cookie.Expires = m.now().Add(sessionLifetime)
	} else {
		cookie.Expires = time.Unix(1, 0)
	}
	http.SetCookie(w, cookie)
}

func (m *Manager) respondSession(w http.ResponseWriter, current session, valid bool) {
	result := map[string]any{"mode": "standalone", "authenticated": valid}
	if valid {
		result["username"] = m.credentials.Username
		result["csrf_token"] = current.CSRF
	}
	respond(w, http.StatusOK, result)
}

func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func randomToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}
