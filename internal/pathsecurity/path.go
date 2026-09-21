package pathsecurity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"golang.org/x/sys/unix"
)

// AuthorizedRoots returns the directories that fnOS has explicitly made
// available to the package user. Both variables use the platform path-list
// separator (":" on fnOS/Linux).
func AuthorizedRoots() []string {
	var roots []string
	for _, name := range []string{"TRIM_DATA_ACCESSIBLE_PATHS", "TRIM_DATA_SHARE_PATHS"} {
		for _, root := range filepath.SplitList(os.Getenv(name)) {
			if strings.TrimSpace(root) != "" {
				roots = append(roots, root)
			}
		}
	}
	return roots
}

// ValidateFile verifies that a user-supplied file resolves inside an fnOS
// authorized directory and is a readable regular file.
func ValidateFile(path string) error {
	resolved, err := validateAuthorizedPath(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return errors.New("文件不存在或没有读取权限")
	}
	if !info.Mode().IsRegular() {
		return errors.New("路径必须指向普通文件")
	}
	if err := unix.Access(resolved, unix.R_OK); err != nil {
		return errors.New("应用用户没有文件读取权限，请在飞牛应用设置中授权所在目录")
	}
	return nil
}

// ValidateDirectory verifies that a user-supplied directory resolves inside
// an fnOS authorized directory. DAV roots additionally require write access.
func ValidateDirectory(path string, writable bool) error {
	resolved, err := validateAuthorizedPath(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return errors.New("目录不存在或没有访问权限")
	}
	if !info.IsDir() {
		return errors.New("路径必须指向目录")
	}
	mode := uint32(unix.R_OK | unix.X_OK)
	if writable {
		mode |= unix.W_OK
	}
	if err := unix.Access(resolved, mode); err != nil {
		operation := "读取"
		if writable {
			operation = "读写"
		}
		return fmt.Errorf("应用用户没有目录%s权限，请在飞牛应用设置中授权该目录", operation)
	}
	return nil
}

// ValidateState covers every external filesystem path emitted into generated
// Nginx configuration. Paths owned internally by the application are not
// user-controlled and are validated by platform.Paths instead.
func ValidateState(state domain.State) error {
	if state.Settings.TLS.ClientVerify != "" && state.Settings.TLS.ClientVerify != "off" {
		if err := ValidateFile(state.Settings.TLS.ClientCAFile); err != nil {
			return fmt.Errorf("客户端 CA 文件: %w", err)
		}
	}
	for _, rule := range state.Rules {
		if err := validateLocation(rule.RootLocation); err != nil {
			return fmt.Errorf("规则 %q 的根 Location: %w", rule.Name, err)
		}
		for _, location := range rule.Locations {
			if err := validateLocation(location.Settings); err != nil {
				return fmt.Errorf("规则 %q 的 Location %q: %w", rule.Name, location.Name, err)
			}
		}
	}
	return nil
}

func validateLocation(settings domain.LocationSettings) error {
	if settings.BackendType == "static" {
		if err := ValidateDirectory(settings.StaticPath, settings.DAV.Enabled); err != nil {
			return fmt.Errorf("静态文件目录: %w", err)
		}
	}
	if settings.BasicAuth {
		if err := ValidateFile(settings.BasicAuthFile); err != nil {
			return fmt.Errorf("Basic Auth 密码文件: %w", err)
		}
	}
	return nil
}

func validateAuthorizedPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.ContainsAny(path, "\x00\r\n") || !filepath.IsAbs(path) {
		return "", errors.New("必须填写安全的绝对路径")
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." {
			return "", errors.New("路径不能包含上级目录跳转")
		}
	}
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return "", errors.New("路径不存在或无法解析符号链接")
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", errors.New("无法解析绝对路径")
	}
	for _, root := range AuthorizedRoots() {
		if !filepath.IsAbs(root) {
			continue
		}
		rootResolved, resolveErr := filepath.EvalSymlinks(filepath.Clean(root))
		if resolveErr != nil {
			continue
		}
		relative, relErr := filepath.Rel(rootResolved, resolved)
		if relErr == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return resolved, nil
		}
	}
	return "", errors.New("路径不在飞牛已授权目录内，请先在应用设置中添加授权目录")
}
