package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
)

func TestStorePersistsValidatedUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(state *domain.State) error {
		state.Settings.DefaultHTTPPort = 19080
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Snapshot().Settings.DefaultHTTPPort; got != 19080 {
		t.Fatalf("expected persisted port 19080, got %d", got)
	}
}

func TestStoreRejectsCorruptState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(path); err == nil {
		t.Fatal("expected corrupt state to be rejected")
	}
}
