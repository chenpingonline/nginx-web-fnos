package nginx

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeProcFixture(t *testing.T, root, relative, value string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func procStat(pid, parent int, name string, start uint64) string {
	fields := make([]string, 20)
	for i := range fields {
		fields[i] = "0"
	}
	fields[0], fields[1], fields[19] = "S", strconv.Itoa(parent), strconv.FormatUint(start, 10)
	return fmt.Sprintf("%d (%s) %s", pid, name, strings.Join(fields, " "))
}

func ticksAuxv(ticks uint64) []byte {
	word := strconv.IntSize / 8
	data := make([]byte, 4*word)
	if word == 8 {
		binary.NativeEndian.PutUint64(data, 17)
		binary.NativeEndian.PutUint64(data[word:], ticks)
	} else {
		binary.NativeEndian.PutUint32(data, 17)
		binary.NativeEndian.PutUint32(data[word:], uint32(ticks))
	}
	return data
}

func TestProcessStatsMeasureMasterAgeAndActualWorkers(t *testing.T) {
	root := t.TempDir()
	writeProcFixture(t, root, "uptime", "1000.50 1000.00\n")
	writeProcFixture(t, root, "self/auxv", string(ticksAuxv(250)))
	// comm may itself contain spaces and parentheses.
	writeProcFixture(t, root, "100/stat", procStat(100, 1, "nginx (master)", 225000))
	writeProcFixture(t, root, "100/task/100/children", "101 102 103 104 105\n")
	for child, title := range map[int]string{
		101: "nginx: worker process\x00\x00",
		102: "nginx: worker process is shutting down\x00",
		103: "nginx: cache manager process\x00",
		104: "nginx: cache loader process\x00",
		105: "nginx: worker process\x00",
	} {
		parent := 100
		if child == 105 {
			parent = 200 // Child PID was recycled after children was read.
		}
		writeProcFixture(t, root, fmt.Sprintf("%d/stat", child), procStat(child, parent, "nginx", 225000))
		writeProcFixture(t, root, fmt.Sprintf("%d/cmdline", child), title)
	}
	uptime, workers := readProcessStats(root, 100)
	if uptime == nil || *uptime != 100 || workers == nil || *workers != 2 {
		t.Fatalf("actual process data incorrect: uptime=%v workers=%v", uptime, workers)
	}
	writeProcFixture(t, root, "100/task/100/children", "")
	_, workers = readProcessStats(root, 100)
	if workers == nil || *workers != 0 {
		t.Fatal("observed master without workers must be zero")
	}
}

func TestProcessStatsUnavailableAndMalformedDataRemainUnknown(t *testing.T) {
	root := t.TempDir()
	if uptime, workers := readProcessStats(root, 100); uptime != nil || workers != nil {
		t.Fatal("missing proc files must not fabricate zero process data")
	}
	writeProcFixture(t, root, "uptime", "10.00 0.00")
	writeProcFixture(t, root, "self/auxv", string(ticksAuxv(100)))
	writeProcFixture(t, root, "100/stat", procStat(100, 1, "nginx", 2000))
	if uptime, _ := readProcessStats(root, 100); uptime != nil {
		t.Fatal("a start time after system uptime must be rejected")
	}
	writeProcFixture(t, root, "100/stat", "invalid")
	writeProcFixture(t, root, "100/task/100/children", "invalid")
	if uptime, workers := readProcessStats(root, 100); uptime != nil || workers != nil {
		t.Fatal("malformed proc fields must remain unknown")
	}
	for _, data := range [][]byte{nil, {17, 1}, ticksAuxv(0), make([]byte, 32)} {
		if clockTicks(data) != 0 {
			t.Fatal("invalid aux vector must not supply an assumed clock frequency")
		}
	}
}
