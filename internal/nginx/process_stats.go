package nginx

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Process information is optional: an unsupported platform or restricted /proc
// must be shown as unavailable, never as a fabricated uptime or worker count.
func processStats(pid int) (*int64, *int) {
	if runtime.GOOS != "linux" {
		return nil, nil
	}
	return readProcessStats("/proc", pid)
}

func readProcessStats(root string, pid int) (*int64, *int) {
	if pid <= 1 {
		return nil, nil
	}
	process := filepath.Join(root, strconv.Itoa(pid))
	return processUptime(root, process), processWorkers(process, root, pid)
}

func processUptime(root, process string) *int64 {
	stat, err := os.ReadFile(filepath.Join(process, "stat"))
	if err != nil {
		return nil
	}
	fields := processStatFields(string(stat))
	if len(fields) < 20 {
		return nil
	}
	// The fields after '(comm)' start with field 3, so index 19 is starttime.
	started, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(root, "uptime"))
	if err != nil || len(strings.Fields(string(data))) < 1 {
		return nil
	}
	uptime, err := strconv.ParseFloat(strings.Fields(string(data))[0], 64)
	if err != nil || math.IsNaN(uptime) || math.IsInf(uptime, 0) {
		return nil
	}
	auxv, err := os.ReadFile(filepath.Join(root, "self", "auxv"))
	if err != nil {
		return nil
	}
	ticks := clockTicks(auxv)
	if ticks == 0 {
		return nil
	}
	seconds := uptime - float64(started)/float64(ticks)
	if seconds < 0 || seconds >= float64(math.MaxInt64) {
		return nil
	}
	v := int64(seconds)
	return &v
}

func processStatFields(stat string) []string {
	// A process name may contain spaces or parentheses; the final ')' closes comm.
	end := strings.LastIndexByte(stat, ')')
	if end < 0 {
		return nil
	}
	return strings.Fields(stat[end+1:])
}

func clockTicks(auxv []byte) uint64 {
	// AT_CLKTCK is the kernel-provided USER_HZ used by /proc/<pid>/stat.
	// Read it instead of assuming every target reports 100 ticks per second.
	const atClockTicks = 17
	word := strconv.IntSize / 8
	for offset := 0; offset+2*word <= len(auxv); offset += 2 * word {
		var key, value uint64
		if word == 8 {
			key = binary.NativeEndian.Uint64(auxv[offset:])
			value = binary.NativeEndian.Uint64(auxv[offset+word:])
		} else {
			key = uint64(binary.NativeEndian.Uint32(auxv[offset:]))
			value = uint64(binary.NativeEndian.Uint32(auxv[offset+word:]))
		}
		if key == 0 {
			break
		}
		if key == atClockTicks {
			return value
		}
	}
	return 0
}

func processWorkers(process, root string, pid int) *int {
	data, err := os.ReadFile(filepath.Join(process, "task", strconv.Itoa(pid), "children"))
	if err != nil {
		return nil
	}
	count := 0
	for _, child := range strings.Fields(string(data)) {
		childPID, err := strconv.Atoi(child)
		if err != nil || childPID <= 1 {
			return nil
		}
		childDir := filepath.Join(root, strconv.Itoa(childPID))
		stat, err := os.ReadFile(filepath.Join(childDir, "stat"))
		if os.IsNotExist(err) {
			continue // A worker can exit while the directory is being sampled.
		}
		if err != nil {
			return nil
		}
		fields := processStatFields(string(stat))
		if len(fields) < 2 {
			return nil
		}
		if fields[1] != strconv.Itoa(pid) {
			continue // Exited/reused PID; this is no longer our master's child.
		}
		command, err := os.ReadFile(filepath.Join(childDir, "cmdline"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil
		}
		title := strings.TrimSpace(strings.ReplaceAll(string(command), "\x00", " "))
		if title == "nginx: worker process" || strings.HasPrefix(title, "nginx: worker process ") {
			count++
		}
	}
	return &count
}
