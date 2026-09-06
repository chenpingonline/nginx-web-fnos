package store

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/fileutil"
)

type Store struct {
	mu    sync.RWMutex
	path  string
	state domain.State
}

func New(path string) (*Store, error) {
	store := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		store.state = domain.DefaultState()
		if err := store.persistLocked(store.state); err != nil {
			return nil, err
		}
		return store, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &store.state); err != nil {
		return nil, err
	}
	if store.state.SchemaVersion == 0 {
		store.state.SchemaVersion = domain.SchemaVersion
	}
	domain.ApplyStateDefaults(&store.state)
	if err := domain.ValidateState(store.state); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Snapshot() domain.State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return domain.CloneState(s.state)
}

func (s *Store) Update(fn func(*domain.State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := domain.CloneState(s.state)
	if err := fn(&next); err != nil {
		return err
	}
	next.SchemaVersion = domain.SchemaVersion
	domain.ApplyStateDefaults(&next)
	next.UpdatedAt = time.Now().UTC()
	if err := domain.ValidateState(next); err != nil {
		return err
	}
	if err := s.persistLocked(next); err != nil {
		return err
	}
	s.state = next
	return nil
}

func (s *Store) Replace(next domain.State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next.SchemaVersion = domain.SchemaVersion
	domain.ApplyStateDefaults(&next)
	next.UpdatedAt = time.Now().UTC()
	if err := domain.ValidateState(next); err != nil {
		return err
	}
	if err := s.persistLocked(next); err != nil {
		return err
	}
	s.state = domain.CloneState(next)
	return nil
}

func (s *Store) persistLocked(state domain.State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(s.path, append(data, '\n'), 0o600)
}
