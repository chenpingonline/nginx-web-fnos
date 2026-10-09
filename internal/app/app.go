package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/adminauth"
	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/httpapi"
	"github.com/chenpingonline/nginx-web-fnos/internal/platform"
	"github.com/chenpingonline/nginx-web-fnos/internal/service"
)

type App struct {
	paths   platform.Paths
	service *service.AppService
	web     fs.FS
}

func New(paths platform.Paths, service *service.AppService, web fs.FS) *App {
	return &App{paths: paths, service: service, web: web}
}

func (a *App) Serve() error {
	runtime, err := platform.LoadRuntime()
	if err != nil {
		return err
	}
	var authentication *adminauth.Manager
	var tcpListener net.Listener
	if runtime.Standalone {
		authentication, err = adminauth.Load(a.paths.VarDir)
		if err != nil {
			return err
		}
		tcpListener, err = net.Listen("tcp", runtime.Listen)
		if err != nil {
			return fmt.Errorf("监听管理 HTTP 端口失败: %w", err)
		}
		defer tcpListener.Close()
	}
	if err := os.MkdirAll(filepath.Dir(a.paths.SocketPath), 0o750); err != nil {
		return err
	}
	if err := removeStaleSocket(a.paths.SocketPath); err != nil {
		return err
	}
	listener, err := net.Listen("unix", a.paths.SocketPath)
	if err != nil {
		return fmt.Errorf("监听 Unix Socket 失败: %w", err)
	}
	defer listener.Close()
	defer os.Remove(a.paths.SocketPath)
	_ = os.Chmod(a.paths.SocketPath, 0o660)
	if runtime.Standalone {
		defer func() {
			if _, err := a.service.NginxStop(); err != nil {
				log.Printf("停止 Nginx 失败: %v", err)
			}
		}()
	}
	if err := a.service.Initialize(); err != nil {
		log.Printf("初始 Nginx 配置初始化失败，管理页面仍将启动: %v", err)
	}
	if runtime.Standalone {
		// Restart the last applied configuration without publishing saved drafts.
		if _, err := a.service.NginxStart(); err != nil {
			log.Printf("Nginx 启动失败，请在管理页面检查并应用配置: %v", err)
		}
	}
	maintenanceContext, stopMaintenance := context.WithCancel(context.Background())
	go a.service.MaintainLogs(maintenanceContext)
	metricsDone := make(chan struct{})
	go func() {
		defer close(metricsDone)
		a.service.MaintainMetrics(maintenanceContext)
	}()
	defer func() {
		stopMaintenance()
		<-metricsDone // Flush the last minute before exiting.
	}()
	// Start certificate jobs only after owning the server socket; a second serve must not issue duplicate orders.
	go a.service.MaintainACME(maintenanceContext)

	handler := httpapi.New(a.service, a.web)
	if runtime.Standalone {
		handler = httpapi.NewStandalone(a.service, a.web)
	}
	server := newServer(handler)
	servers := []*http.Server{server}
	listeners := []net.Listener{listener}
	if tcpListener != nil {
		servers = append(servers, newServer(authentication.Handler(handler)))
		listeners = append(listeners, tcpListener)
		log.Printf("独立管理入口已监听 %s", tcpListener.Addr())
	}
	serverErrors := make(chan error, len(servers))
	for index, server := range servers {
		go func(server *http.Server, listener net.Listener) {
			if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErrors <- err
			}
		}(server, listeners[index])
	}
	log.Printf("%s %s 管理服务已监听 %s", domain.AppName, domain.AppVersion, a.paths.SocketPath)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	var serveErr error
	select {
	case sig := <-signals:
		log.Printf("收到 %s，正在停止管理服务", sig)
	case serveErr = <-serverErrors:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	for _, server := range servers {
		if err := server.Shutdown(ctx); err != nil {
			server.Close()
			serveErr = errors.Join(serveErr, err)
		}
	}
	return serveErr
}

func newServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

func removeStaleSocket(socketPath string) error {
	info, err := os.Lstat(socketPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("Socket 路径已被普通文件占用: %s", socketPath)
	}
	connection, err := net.DialTimeout("unix", socketPath, 300*time.Millisecond)
	if err == nil {
		connection.Close()
		return fmt.Errorf("管理 Socket 正在使用，请勿同时启动多个实例: %s", socketPath)
	}
	if !errors.Is(err, syscall.ECONNREFUSED) && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("无法确认管理 Socket 是否失效: %w", err)
	}
	return os.Remove(socketPath)
}
