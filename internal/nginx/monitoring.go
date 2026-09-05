package nginx

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type BasicStatus struct {
	PID         int
	Requests    uint64
	Connections int
}

func (m *Manager) BasicStatus(ctx context.Context) (BasicStatus, error) {
	pid, running := m.runningPID()
	if !running {
		return BasicStatus{}, fmt.Errorf("Nginx 已停止")
	}
	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", m.paths.StatusSocket())
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/status", nil)
	if err != nil {
		return BasicStatus{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return BasicStatus{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return BasicStatus{}, fmt.Errorf("状态接口 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return BasicStatus{}, err
	}
	status, err := parseBasicStatus(string(data))
	status.PID = pid
	return status, err
}

func parseBasicStatus(body string) (BasicStatus, error) {
	var status BasicStatus
	var accepts, handled uint64
	var reading, writing, waiting int
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 4 {
		return status, fmt.Errorf("无法识别 Nginx 状态数据")
	}
	if n, err := fmt.Sscanf(lines[0], "Active connections: %d", &status.Connections); err != nil || n != 1 {
		return status, fmt.Errorf("连接数格式不正确")
	}
	if n, err := fmt.Sscanf(lines[2], "%d %d %d", &accepts, &handled, &status.Requests); err != nil || n != 3 {
		return status, fmt.Errorf("请求计数格式不正确")
	}
	if n, err := fmt.Sscanf(lines[3], "Reading: %d Writing: %d Waiting: %d", &reading, &writing, &waiting); err != nil || n != 3 {
		return status, fmt.Errorf("连接状态格式不正确")
	}
	if status.Connections < 1 || reading < 0 || writing < 0 || waiting < 0 {
		return status, fmt.Errorf("连接状态无效")
	}
	// The private status request itself is one active client connection.
	status.Connections--
	return status, nil
}
