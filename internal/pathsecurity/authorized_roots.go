package pathsecurity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const authorizationSocket = "/var/run/trim_open_gateway_apiscope.socket"

var authorizationClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", authorizationSocket)
	}},
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
}

// Query on each validation operation, never cache an administrator's grants.
// Legacy hosts without the Open API retain their startup-environment behavior.
func currentAuthorizedRoots() ([]string, error) {
	token := strings.TrimSpace(os.Getenv("TRIM_API_TOKEN"))
	if token == "" {
		if _, err := os.Stat(authorizationSocket); errors.Is(err, os.ErrNotExist) {
			return AuthorizedRoots(), nil
		}
		return nil, errors.New("无法查询飞牛最新授权目录：缺少应用 API token，请检查应用安装及接口授权")
	}
	roots, err := queryAuthorizedRoots(authorizationClient, token)
	if err != nil {
		return nil, err
	}
	// Package-owned shares are independent of revocable administrator grants.
	// Do not merge TRIM_DATA_ACCESSIBLE_PATHS: it may contain revoked paths.
	for _, path := range strings.Split(os.Getenv("TRIM_DATA_SHARE_PATHS"), string(os.PathListSeparator)) {
		if strings.TrimSpace(path) != "" {
			roots = append(roots, path)
		}
	}
	return roots, nil
}

func queryAuthorizedRoots(client *http.Client, token string) ([]string, error) {
	body := []byte(`{"req":"trim.file.getSharedAccessibleFolders","appName":"nginx-web","data":{}}`)
	request, err := http.NewRequest(http.MethodPost, "http://localhost/api/v1/trimapp", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		return nil, errors.New("无法查询飞牛最新授权目录，请检查飞牛授权接口后重试")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("查询飞牛最新授权目录失败（HTTP %d），请检查应用接口授权", response.StatusCode)
	}
	var result struct {
		Code *int `json:"code"`
		Data *struct {
			Paths *[]string `json:"paths"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&result); err != nil || result.Code == nil {
		return nil, errors.New("飞牛授权接口返回了无效响应")
	}
	if *result.Code != 0 {
		return nil, fmt.Errorf("查询飞牛最新授权目录失败（错误码 %d），请检查应用接口授权", *result.Code)
	}
	if result.Data == nil || result.Data.Paths == nil {
		return nil, errors.New("飞牛授权接口未返回目录列表")
	}
	return *result.Data.Paths, nil
}

type pathValidator struct {
	roots  []string
	loaded bool
}

func (v *pathValidator) authorizedRoots() ([]string, error) {
	if !v.loaded {
		roots, err := currentAuthorizedRoots()
		if err != nil {
			return nil, err
		}
		v.roots, v.loaded = roots, true
	}
	return v.roots, nil
}
