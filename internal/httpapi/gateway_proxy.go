package httpapi

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"golang.org/x/net/html"
)

const gatewayProxyPath = gatewayPrefix + "/proxy/"
const maxGatewayHTMLBytes = 16 << 20

func (a *API) handleGatewayProxy(w http.ResponseWriter, r *http.Request, cleanPath string) {
	if !a.requireAdmin(w, r) {
		return
	}
	if a.service == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "反向代理服务尚未初始化")
		return
	}

	ruleID, upstreamPath, ok := splitGatewayProxyPath(cleanPath)
	if !ok || !domain.ValidID(ruleID) {
		writeAPIError(w, http.StatusBadRequest, "FN Connect 代理规则 ID 不合法")
		return
	}

	rule, found := findGatewayProxyRule(a.service.State().Rules, ruleID)
	if !found {
		writeAPIError(w, http.StatusNotFound, "找不到已启用的 HTTP 反向代理规则")
		return
	}
	if rule.UpstreamPoolID != "" {
		writeAPIError(w, http.StatusBadRequest, "验证版暂不支持上游池规则")
		return
	}

	target, err := gatewayProxyTarget(rule)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}

	originalHost := r.Host
	routePrefix := gatewayProxyPath + rule.ID
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = gatewayProxyTransport(rule)
	originalDirector := proxy.Director
	proxy.Director = func(request *http.Request) {
		originalDirector(request)
		request.URL.Path = upstreamPath
		request.URL.RawPath = ""
		request.Host = target.Host
		if rule.PreserveHost {
			request.Host = originalHost
		}
		request.Header.Set("X-Forwarded-Host", originalHost)
		request.Header.Set("X-Forwarded-Prefix", routePrefix)
		// Let net/http transparently decode compressed HTML so absolute resource
		// paths can be rewritten for the fnOS gateway subpath.
		request.Header.Del("Accept-Encoding")
		if request.Header.Get("X-Forwarded-Proto") == "" {
			if r.TLS != nil {
				request.Header.Set("X-Forwarded-Proto", "https")
			} else {
				request.Header.Set("X-Forwarded-Proto", "http")
			}
		}
		// fnOS identity headers are only for nginx-web authorization and must not
		// be disclosed to an arbitrary upstream application.
		request.Header.Del("X-Trim-Userid")
		request.Header.Del("X-Trim-Isadmin")
		request.Header.Del("X-Trim-Username")
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		rewriteGatewayLocation(response.Header, target, routePrefix)
		rewriteGatewayCookiePaths(response.Header, routePrefix)
		return rewriteGatewayHTMLResponse(response, target, routePrefix)
	}
	proxy.ErrorHandler = func(response http.ResponseWriter, _ *http.Request, proxyErr error) {
		writeAPIError(response, http.StatusBadGateway, "连接上游失败: "+proxyErr.Error())
	}
	proxy.ServeHTTP(w, r)
}

func rewriteGatewayHTMLResponse(response *http.Response, target *url.URL, routePrefix string) error {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]))
	if mediaType != "text/html" || response.Body == nil {
		return nil
	}
	if response.ContentLength > maxGatewayHTMLBytes {
		return nil
	}

	originalBody := response.Body
	data, err := io.ReadAll(io.LimitReader(originalBody, maxGatewayHTMLBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxGatewayHTMLBytes {
		response.Body = io.NopCloser(io.MultiReader(bytes.NewReader(data), originalBody))
		return nil
	}
	if err := originalBody.Close(); err != nil {
		return err
	}

	document, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		setGatewayResponseBody(response, data)
		return nil
	}
	rewriteGatewayHTMLNode(document, target, routePrefix)
	var output bytes.Buffer
	if err := html.Render(&output, document); err != nil {
		setGatewayResponseBody(response, data)
		return nil
	}
	setGatewayResponseBody(response, output.Bytes())
	return nil
}

func setGatewayResponseBody(response *http.Response, data []byte) {
	response.Body = io.NopCloser(bytes.NewReader(data))
	response.ContentLength = int64(len(data))
	response.Header.Set("Content-Length", strconv.Itoa(len(data)))
	response.Header.Del("Content-Encoding")
	response.Header.Del("ETag")
}

func rewriteGatewayHTMLNode(node *html.Node, target *url.URL, routePrefix string) {
	if node.Type == html.ElementNode {
		for index := range node.Attr {
			attribute := &node.Attr[index]
			switch strings.ToLower(attribute.Key) {
			case "href", "src", "action", "formaction", "poster", "data":
				attribute.Val = rewriteGatewayResourceURL(attribute.Val, target, routePrefix)
			case "srcset":
				attribute.Val = rewriteGatewaySrcset(attribute.Val, target, routePrefix)
			case "style":
				attribute.Val = rewriteGatewayCSSURLs(attribute.Val, routePrefix)
			case "content":
				if strings.EqualFold(node.Data, "meta") && gatewayHTMLAttribute(node, "http-equiv") == "refresh" {
					attribute.Val = rewriteGatewayMetaRefresh(attribute.Val, target, routePrefix)
				}
			}
		}
	}
	if node.Type == html.TextNode && node.Parent != nil && strings.EqualFold(node.Parent.Data, "style") {
		node.Data = rewriteGatewayCSSURLs(node.Data, routePrefix)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		rewriteGatewayHTMLNode(child, target, routePrefix)
	}
}

func gatewayHTMLAttribute(node *html.Node, key string) string {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, key) {
			return strings.ToLower(strings.TrimSpace(attribute.Val))
		}
	}
	return ""
}

func rewriteGatewayResourceURL(raw string, target *url.URL, routePrefix string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
		return raw
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return raw
	}
	if parsed.IsAbs() {
		if !strings.EqualFold(parsed.Scheme, target.Scheme) || !strings.EqualFold(parsed.Host, target.Host) {
			return raw
		}
		parsed.Scheme = ""
		parsed.Host = ""
	}
	if !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, routePrefix+"/") || parsed.Path == routePrefix {
		return raw
	}
	parsed.Path = routePrefix + parsed.Path
	return parsed.String()
}

func rewriteGatewaySrcset(raw string, target *url.URL, routePrefix string) string {
	candidates := strings.Split(raw, ",")
	for index, candidate := range candidates {
		fields := strings.Fields(strings.TrimSpace(candidate))
		if len(fields) == 0 {
			continue
		}
		fields[0] = rewriteGatewayResourceURL(fields[0], target, routePrefix)
		candidates[index] = strings.Join(fields, " ")
	}
	return strings.Join(candidates, ", ")
}

func rewriteGatewayCSSURLs(raw, routePrefix string) string {
	replacements := []struct{ old, new string }{
		{"url(/", "url(" + routePrefix + "/"},
		{"url('/", "url('" + routePrefix + "/"},
		{"url(\"/", "url(\"" + routePrefix + "/"},
	}
	for _, replacement := range replacements {
		raw = strings.ReplaceAll(raw, replacement.old, replacement.new)
	}
	return raw
}

func rewriteGatewayMetaRefresh(raw string, target *url.URL, routePrefix string) string {
	parts := strings.SplitN(raw, ";", 2)
	if len(parts) != 2 {
		return raw
	}
	assignment := strings.SplitN(parts[1], "=", 2)
	if len(assignment) != 2 || !strings.EqualFold(strings.TrimSpace(assignment[0]), "url") {
		return raw
	}
	return parts[0] + ";url=" + rewriteGatewayResourceURL(strings.TrimSpace(assignment[1]), target, routePrefix)
}

func splitGatewayProxyPath(cleanPath string) (string, string, bool) {
	rest := strings.TrimPrefix(cleanPath, "/proxy/")
	if rest == cleanPath || rest == "" {
		return "", "", false
	}
	parts := strings.SplitN(rest, "/", 2)
	upstreamPath := "/"
	if len(parts) == 2 && parts[1] != "" {
		upstreamPath += parts[1]
	}
	return parts[0], upstreamPath, true
}

func findGatewayProxyRule(rules []domain.ProxyRule, id string) (domain.ProxyRule, bool) {
	for _, rule := range rules {
		if rule.ID == id && rule.Enabled {
			return rule, true
		}
	}
	return domain.ProxyRule{}, false
}

func gatewayProxyTarget(rule domain.ProxyRule) (*url.URL, error) {
	scheme := strings.ToLower(strings.TrimSpace(rule.UpstreamScheme))
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("仅支持 HTTP 或 HTTPS 上游")
	}
	if net.ParseIP(rule.UpstreamHost) == nil && strings.ContainsAny(rule.UpstreamHost, "/?#@") {
		return nil, fmt.Errorf("上游地址不合法")
	}
	if strings.TrimSpace(rule.UpstreamHost) == "" || rule.UpstreamPort < 1 || rule.UpstreamPort > 65535 {
		return nil, fmt.Errorf("上游地址或端口不合法")
	}
	return &url.URL{Scheme: scheme, Host: net.JoinHostPort(rule.UpstreamHost, strconv.Itoa(rule.UpstreamPort))}, nil
}

func gatewayProxyTransport(rule domain.ProxyRule) *http.Transport {
	connectTimeout := time.Duration(rule.ConnectTimeoutSeconds) * time.Second
	if connectTimeout <= 0 {
		connectTimeout = 10 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}).DialContext
	transport.ResponseHeaderTimeout = time.Duration(rule.ReadTimeoutSeconds) * time.Second
	if transport.ResponseHeaderTimeout <= 0 {
		transport.ResponseHeaderTimeout = 60 * time.Second
	}
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: !rule.VerifyUpstreamTLS} //nolint:gosec -- rule explicitly controls upstream verification
	return transport
}

func rewriteGatewayLocation(header http.Header, target *url.URL, routePrefix string) {
	raw := header.Get("Location")
	if raw == "" {
		return
	}
	location, err := url.Parse(raw)
	if err != nil {
		return
	}
	if location.IsAbs() {
		if !strings.EqualFold(location.Scheme, target.Scheme) || !strings.EqualFold(location.Host, target.Host) {
			return
		}
		location.Scheme = ""
		location.Host = ""
	}
	if strings.HasPrefix(location.Path, "/") {
		location.Path = routePrefix + location.Path
		header.Set("Location", location.String())
	}
}

func rewriteGatewayCookiePaths(header http.Header, routePrefix string) {
	cookies := header.Values("Set-Cookie")
	if len(cookies) == 0 {
		return
	}
	header.Del("Set-Cookie")
	for _, raw := range cookies {
		parts := strings.Split(raw, ";")
		pathFound := false
		for index := 1; index < len(parts); index++ {
			attribute := strings.TrimSpace(parts[index])
			if strings.HasPrefix(strings.ToLower(attribute), "path=") {
				cookiePath := strings.TrimSpace(attribute[len("path="):])
				if !strings.HasPrefix(cookiePath, "/") {
					cookiePath = "/" + cookiePath
				}
				parts[index] = " Path=" + routePrefix + cookiePath
				pathFound = true
			}
		}
		if !pathFound {
			parts = append(parts, " Path="+routePrefix+"/")
		}
		header.Add("Set-Cookie", strings.Join(parts, ";"))
	}
}
