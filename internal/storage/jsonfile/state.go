package jsonfile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/duv0-x/mockingbird/internal/storage"
)

// stateStore keeps the SEQUENCE cursors of REST methods and SOAP operations
// out of the project files (D-13). Writes are coalesced: the first change
// arms a timer (<= flushDelay) and Close flushes whatever is left, so
// project files only change on real edits.
type stateStore struct {
	mu         sync.Mutex
	path       string
	data       map[string]int
	dirty      bool
	timer      *time.Timer
	flushDelay time.Duration
}

func newStateStore(path string, flushDelay time.Duration) stateStore {
	return stateStore{path: path, data: map[string]int{}, flushDelay: flushDelay}
}

func (s *stateStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("jsonfile: read %s: %w", s.path, err)
	}
	f, err := decodeStateFile(data)
	if err != nil {
		if errors.Is(err, ErrUnsupportedSchema) {
			return fmt.Errorf("jsonfile: %w", err)
		}
		// A corrupt state file is runtime state only; it is rebuilt from
		// scratch on the next flush.
		slog.Error("jsonfile: skipping unparsable file", "path", s.path, "err", err)
		return nil
	}
	for k, v := range f.SequenceIndex {
		s.data[k] = v
	}
	return nil
}

func (s *stateStore) SequenceIndex(ctx context.Context, key string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[key], nil
}

func (s *stateStore) SetSequenceIndex(ctx context.Context, key string, index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = index
	s.markDirtyLocked()
	return nil
}

func (s *stateStore) DeleteSequenceIndex(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; ok {
		delete(s.data, key)
		s.markDirtyLocked()
	}
	return nil
}

func (s *stateStore) markDirtyLocked() {
	s.dirty = true
	if s.timer == nil {
		s.timer = time.AfterFunc(s.flushDelay, s.flush)
	}
}

func (s *stateStore) flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushLocked()
}

func (s *stateStore) flushLocked() {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if !s.dirty {
		return
	}
	if err := writeFile(s.path, stateFile{SchemaVersion: schemaVersion, SequenceIndex: s.data}); err != nil {
		// Keep dirty so the next change (or Close) retries.
		slog.Error("jsonfile: flush sequence state", "path", s.path, "err", err)
		return
	}
	s.dirty = false
}

func (s *stateStore) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushLocked()
	if s.dirty {
		return fmt.Errorf("jsonfile: flush sequence state %s: still dirty", s.path)
	}
	return nil
}

var _ storage.StateRepository = (*stateStore)(nil)
