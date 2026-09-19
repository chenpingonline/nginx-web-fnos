package metrics

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStreamAggregationPersistenceAndRotation(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	dir := t.TempDir()
	log := filepath.Join(dir, "stream.log")
	history := filepath.Join(dir, "history.json")
	c := NewStream(log, history, now.Add(-time.Hour))
	row := func(rule, protocol string, status int, limit string) string {
		return fmt.Sprintf(`{"time":%d,"rule":%q,"protocol":%q,"status":%d,"sent":20,"received":10,"duration":2,"client":"127.0.0.1","upstream":"127.0.0.1:80","limit":%q}`+"\n", now.Unix(), rule, protocol, status, limit)
	}
	if err := os.WriteFile(log, []byte(row("a", "TCP", 200, "")+row("b", "UDP", 502, "")+row("a", "TCP", 503, "REJECTED")+row("bad", "HTTP", 200, "")), 0600); err != nil {
		t.Fatal(err)
	}
	c.Collect(now)
	r := c.Snapshot(now, 60, "")
	if r.Sessions != 3 || r.Errors != 2 || r.Rejected != 1 || r.Sent != 60 || r.Received != 30 || r.Duration != 6 || len(r.Recent) != 3 {
		t.Fatalf("bad aggregate: %+v", r)
	}
	a := c.Snapshot(now, 15, "a")
	if a.Sessions != 2 || a.Errors != 1 || len(a.Rules) != 1 || len(a.Recent) != 2 {
		t.Fatalf("bad scope: %+v", a)
	}
	c.Flush(now)
	c = NewStream(log, history, now)
	c.Collect(now)
	if c.Snapshot(now, 60, "").Sessions != 3 {
		t.Fatal("restart duplicated or lost records")
	}
	if err := os.Rename(log, log+".1"); err != nil {
		t.Fatal(err)
	}
	partial := row("b", "UDP", 200, "")
	os.WriteFile(log, []byte(partial[:len(partial)-1]), 0600)
	c.Collect(now)
	if c.Snapshot(now, 60, "").Sessions != 3 {
		t.Fatal("read partial record")
	}
	f, _ := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0600)
	f.WriteString("\n")
	f.Close()
	c.Collect(now)
	if c.Snapshot(now, 60, "").Sessions != 4 {
		t.Fatal("rotation/partial resume failed")
	}
	for i := 0; i < 250; i++ {
		c.ingest([]byte(row("a", "TCP", 200, "")), now)
	}
	if len(c.data.Recent) != 200 || len(c.Snapshot(now, 60, "").Recent) != 50 {
		t.Fatal("unbounded samples")
	}
	c.prune(now.Add(Retention + time.Hour))
	if len(c.data.Buckets) != 0 || len(c.data.Recent) != 0 {
		t.Fatal("history not pruned")
	}
}
func TestStreamFirstInstallDoesNotBackfill(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "stream.log")
	now := time.Now()
	os.WriteFile(log, []byte("old log\n"), 0600)
	c := NewStream(log, filepath.Join(dir, "history.json"), now)
	c.Collect(now)
	if c.data.Cursor.Offset != 8 || c.Snapshot(now, 60, "").Sessions != 0 {
		t.Fatal("unexpected backfill")
	}
}

func TestStreamSamplePreservesMilliseconds(t *testing.T) {
	now := time.UnixMilli(1_758_292_800_500)
	c := NewStream(filepath.Join(t.TempDir(), "missing.log"), filepath.Join(t.TempDir(), "history.json"), now.Add(-time.Hour))
	c.ingest([]byte(`{"time":1758292800.123,"rule":"mysql8","protocol":"TCP","status":200,"sent":20,"received":10,"duration":2,"client":"127.0.0.1","upstream":"127.0.0.1:3308","limit":""}`), now)

	recent := c.Snapshot(now, 60, "").Recent
	if len(recent) != 1 || recent[0].Time.UnixMilli() != 1_758_292_800_123 {
		t.Fatalf("stream sample time = %v, want millisecond precision", recent)
	}
}
