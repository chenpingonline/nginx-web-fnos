package metrics

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type cursor struct {
	ID     string `json:"id"`
	Offset int64  `json:"offset"`
}

func fileID(info os.FileInfo) string {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino)
	}
	return info.Name() + ":" + info.ModTime().String()
}

func (c *Collector) readLogs(now time.Time) error {
	entries, err := os.ReadDir(filepath.Dir(c.logPath))
	if err != nil {
		return errors.New("统计日志尚不可用")
	}
	type logFile struct {
		path     string
		rotation int
		info     os.FileInfo
	}
	var files []logFile
	base := filepath.Base(c.logPath)
	for _, entry := range entries {
		rotation := 0
		if entry.Name() != base {
			if !strings.HasPrefix(entry.Name(), base+".") {
				continue
			}
			rotation, err = strconv.Atoi(strings.TrimPrefix(entry.Name(), base+"."))
			if err != nil || rotation < 1 || rotation > 100 {
				continue
			}
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, logFile{filepath.Join(filepath.Dir(c.logPath), entry.Name()), rotation, info})
	}
	if len(files) == 0 {
		return errors.New("统计日志尚不可用，请先应用配置")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rotation > files[j].rotation })
	start := 0
	if c.data.Cursor.ID != "" {
		start = -1
		for i, file := range files {
			if fileID(file.info) == c.data.Cursor.ID {
				start = i
				break
			}
		}
		if start == -1 {
			latest := files[len(files)-1]
			c.data.Cursor = cursor{fileID(latest.info), latest.info.Size()}
			return errors.New("部分统计日志已被清理，已从当前记录继续采集")
		}
	}
	budget := int64(4 * 1024 * 1024)
	for _, file := range files[start:] {
		id := fileID(file.info)
		if c.data.Cursor.ID != id {
			c.data.Cursor = cursor{ID: id}
		}
		if c.data.Cursor.Offset > file.info.Size() {
			c.data.Cursor.Offset = 0
		}
		f, err := os.Open(file.path)
		if err != nil {
			return errors.New("统计日志读取失败")
		}
		if _, err := f.Seek(c.data.Cursor.Offset, io.SeekStart); err != nil {
			f.Close()
			return err
		}
		reader := bufio.NewReader(io.LimitReader(f, budget))
		for {
			line, readErr := reader.ReadBytes('\n')
			if readErr != nil {
				break
			} // keep a partial final record for the next poll
			c.data.Cursor.Offset += int64(len(line))
			budget -= int64(len(line))
			if len(line) < 16*1024 {
				c.ingest(line, now)
			}
		}
		f.Close()
		if c.data.Cursor.Offset < file.info.Size() {
			return errors.New("统计日志正在追赶，当前统计可能不完整")
		}
	}
	return nil
}
