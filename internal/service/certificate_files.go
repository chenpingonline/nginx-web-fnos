package service

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func readCertificateFile(path string, limit int64) (string, error) {
	path = strings.TrimSpace(path)
	if !filepath.IsAbs(path) {
		return "", errors.New("请输入服务器上的绝对文件路径")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", errors.New("文件不存在或没有读取权限")
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("路径必须指向普通文件")
	}
	if info.Size() > limit {
		return "", errors.New("文件过大")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", errors.New("无法读取文件，请检查权限")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return "", errors.New("读取文件失败")
	}
	if int64(len(data)) > limit {
		return "", errors.New("文件过大")
	}
	return string(data), nil
}
