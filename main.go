package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

//go:embed web/*
var webAssets embed.FS

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	paths, err := loadPaths()
	fatalIf(err)
	service, err := newAppService(paths)
	fatalIf(err)

	switch command {
	case "serve":
		fatalIf(serve(paths, service))
	case "init":
		result, err := service.Prepare()
		fatalIf(err)
		printJSON(result)
	case "nginx-start":
		result, err := service.NginxStart()
		fatalIf(err)
		printJSON(result)
	case "nginx-stop":
		result, err := service.NginxStop()
		fatalIf(err)
		printJSON(result)
	case "nginx-reload":
		result, err := service.NginxReload()
		fatalIf(err)
		printJSON(result)
	case "nginx-test":
		result, err := service.NginxTest()
		fatalIf(err)
		printJSON(result)
	case "doctor":
		fatalIf(runDoctor(paths, service))
	case "version", "--version", "-v":
		fmt.Printf("nginx-web %s (Nginx %s)\n", AppVersion, NginxVersion)
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", command)
		fmt.Fprintln(os.Stderr, "可用命令: serve, init, nginx-start, nginx-stop, nginx-reload, nginx-test, doctor, version")
		os.Exit(2)
	}
}

func serve(paths Paths, service *AppService) error {
	if _, err := os.Stat(paths.NginxMaster); errors.Is(err, os.ErrNotExist) {
		if _, prepareErr := service.Prepare(); prepareErr != nil {
			log.Printf("初始 Nginx 配置生成失败，管理页面仍将启动: %v", prepareErr)
		}
	}
	if err := os.MkdirAll(filepath.Dir(paths.SocketPath), 0o750); err != nil {
		return err
	}
	if err := removeStaleSocket(paths.SocketPath); err != nil {
		return err
	}
	listener, err := net.Listen("unix", paths.SocketPath)
	if err != nil {
		return fmt.Errorf("监听 Unix Socket 失败: %w", err)
	}
	defer listener.Close()
	defer os.Remove(paths.SocketPath)
	_ = os.Chmod(paths.SocketPath, 0o660)

	api, err := newAPI(service, webAssets)
	if err != nil {
		return err
	}
	server := &http.Server{
		Handler:           api,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("nginx-web %s 管理服务已监听 %s", AppVersion, paths.SocketPath)
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

func runDoctor(paths Paths, service *AppService) error {
	checks := map[string]any{
		"app_name":      AppName,
		"app_version":   AppVersion,
		"nginx_version": NginxVersion,
		"paths":         paths,
		"overview":      service.Overview(),
	}
	if err := service.nginx.CheckBinary(); err != nil {
		checks["binary_error"] = err.Error()
	} else {
		checks["binary_ok"] = true
	}
	if _, err := os.Stat(paths.NginxMaster); err == nil {
		result, testErr := service.NginxTest()
		if testErr != nil {
			checks["config_error"] = testErr.Error()
		} else {
			checks["config_test"] = result
		}
	}
	printJSON(checks)
	return nil
}

func printJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func fatalIf(err error) {
	if err == nil {
		return
	}
	log.Print(err)
	os.Exit(1)
}
