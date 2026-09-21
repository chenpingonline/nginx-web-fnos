package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// HealthCheck verifies that the Unix socket can serve HTTP, not merely that
// its filesystem entry already exists.
func HealthCheck(socketPath string) error {
	dialer := net.Dialer{Timeout: time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	defer transport.CloseIdleConnections()

	client := http.Client{Transport: transport, Timeout: 2 * time.Second}
	response, err := client.Get("http://nginx-web/healthz")
	if err != nil {
		return fmt.Errorf("管理服务尚未就绪: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("管理服务健康检查返回 HTTP %d", response.StatusCode)
	}
	return nil
}
