package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogWriterFollowsRotatedPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backend.log")
	writer := AppendLog{Path: path}
	if _, err := writer.Write([]byte("before\n")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("after\n")); err != nil {
		t.Fatal(err)
	}
	active, err := os.ReadFile(path)
	if err != nil || string(active) != "after\n" {
		t.Fatalf("active log: %q, %v", active, err)
	}
	rotated, err := os.ReadFile(path + ".1")
	if err != nil || string(rotated) != "before\n" {
		t.Fatalf("rotated log: %q, %v", rotated, err)
	}
}
