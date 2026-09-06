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

	"github.com/chenpingonline/fn-nginx-web/internal/domain"
	"github.com/chenpingonline/fn-nginx-web/internal/httpapi"
	"github.com/chenpingonline/fn-nginx-web/internal/platform"
	"github.com/chenpingonline/fn-nginx-web/internal/service"
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
	if err := a.service.Initialize(); err != nil {
		log.Printf("初始 Nginx 配置初始化失败，管理页面仍将启动: %v", err)
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
	// Start certificate jobs only after owning the server socket; a second serve must not issue duplicate orders.
	go a.service.MaintainACME(maintenanceContext)
	defer os.Remove(a.paths.SocketPath)
	_ = os.Chmod(a.paths.SocketPath, 0o660)

	server := &http.Server{
		Handler:           httpapi.New(a.service, a.web),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("%s %s 管理服务已监听 %s", domain.AppName, domain.AppVersion, a.paths.SocketPath)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
		close(serverErrors)
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	select {
	case sig := <-signals:
		log.Printf("收到 %s，正在停止管理服务", sig)
	case err := <-serverErrors:
		if err != nil {
			return err
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
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
	return os.Remove(socketPath)
}
