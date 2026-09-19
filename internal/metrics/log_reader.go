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
	return readMetricLogs(c.logPath, &c.data.Cursor, func(line []byte) { c.ingest(line, now) })
}

func readMetricLogs(logPath string, position *cursor, ingest func([]byte)) error {
	entries, err := os.ReadDir(filepath.Dir(logPath))
	if err != nil {
		return errors.New("统计日志尚不可用")
	}
	type logFile struct {
		path     string
		rotation int
		info     os.FileInfo
	}
	var files []logFile
	base := filepath.Base(logPath)
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
		files = append(files, logFile{filepath.Join(filepath.Dir(logPath), entry.Name()), rotation, info})
	}
	if len(files) == 0 {
		return errors.New("统计日志尚不可用，请先应用配置")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rotation > files[j].rotation })
	start := 0
	if position.ID != "" {
		start = -1
		for i, file := range files {
			if fileID(file.info) == position.ID {
				start = i
				break
			}
		}
		if start == -1 {
			latest := files[len(files)-1]
			*position = cursor{fileID(latest.info), latest.info.Size()}
			return errors.New("部分统计日志已被清理，已从当前记录继续采集")
		}
	}
	budget := int64(4 * 1024 * 1024)
	for _, file := range files[start:] {
		id := fileID(file.info)
		if position.ID != id {
			*position = cursor{ID: id}
		}
		if position.Offset > file.info.Size() {
			position.Offset = 0
		}
		f, err := os.Open(file.path)
		if err != nil {
			return errors.New("统计日志读取失败")
		}
		if _, err := f.Seek(position.Offset, io.SeekStart); err != nil {
			f.Close()
			return err
		}
		reader := bufio.NewReader(io.LimitReader(f, budget))
		for {
			line, readErr := reader.ReadBytes('\n')
			if readErr != nil {
				break
			} // keep a partial final record for the next poll
			position.Offset += int64(len(line))
			budget -= int64(len(line))
			if len(line) < 16*1024 {
				ingest(line)
			}
		}
		f.Close()
		if position.Offset < file.info.Size() {
			return errors.New("统计日志正在追赶，当前统计可能不完整")
		}
	}
	return nil
}
