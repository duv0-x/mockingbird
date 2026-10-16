package jsonfile

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// eventsStore keeps REST and SOAP event logs with their own lock: mock
// traffic appends an event per request and must not contend with project
// reads. Eviction mirrors CreateRestEventService: when the count reaches the
// maximum, the oldest event (by StartDate) is deleted before the new one is
// saved.
type eventsStore struct {
	dir     string
	restMax int
	soapMax int

	mu   sync.Mutex
	rest []domain.RestEvent
	soap []domain.SoapEvent
}

func newEventsStore(dir string, restMax, soapMax int) eventsStore {
	return eventsStore{dir: dir, restMax: restMax, soapMax: soapMax}
}

func (e *eventsStore) load() error {
	if err := loadJSONDir(filepath.Join(e.dir, "events", "rest"), ".json", func(base string, data []byte) error {
		f, err := decodeRestEventFile(data)
		if err != nil {
			slog.Error("jsonfile: skipping unparsable file", "path", e.restPath(base), "err", err)
			return nil
		}
		if f.Event.ID == "" || f.Event.ID != base {
			slog.Error("jsonfile: skipping event file with mismatched id", "path", e.restPath(base), "id", f.Event.ID)
			return nil
		}
		e.rest = append(e.rest, cloneRestEvent(f.Event))
		return nil
	}); err != nil {
		return fmt.Errorf("jsonfile: load rest events: %w", err)
	}
	return loadJSONDir(filepath.Join(e.dir, "events", "soap"), ".json", func(base string, data []byte) error {
		f, err := decodeSoapEventFile(data)
		if err != nil {
			slog.Error("jsonfile: skipping unparsable file", "path", e.soapPath(base), "err", err)
			return nil
		}
		if f.Event.ID == "" || f.Event.ID != base {
			slog.Error("jsonfile: skipping event file with mismatched id", "path", e.soapPath(base), "id", f.Event.ID)
			return nil
		}
		e.soap = append(e.soap, cloneSoapEvent(f.Event))
		return nil
	})
}

func (e *eventsStore) restPath(id string) string {
	return filepath.Join(e.dir, "events", "rest", id+".json")
}

func (e *eventsStore) soapPath(id string) string {
	return filepath.Join(e.dir, "events", "soap", id+".json")
}

func (e *eventsStore) AddRest(ctx context.Context, ev domain.RestEvent) error {
	if ev.ID == "" {
		ev.ID = domain.NewID()
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.evictRest()
	if err := writeFile(e.restPath(ev.ID), restEventFile{SchemaVersion: schemaVersion, Kind: "RestEvent", Event: ev}); err != nil {
		return fmt.Errorf("persist rest event %s: %w", ev.ID, err)
	}
	e.rest = append(e.rest, cloneRestEvent(ev))
	return nil
}

func (e *eventsStore) evictRest() {
	if len(e.rest) < e.restMax {
		return
	}
	oldest := oldestRestIndex(e.rest)
	id := e.rest[oldest].ID
	removeFile(e.restPath(id))
	e.rest = append(e.rest[:oldest], e.rest[oldest+1:]...)
}

func oldestRestIndex(events []domain.RestEvent) int {
	best := 0
	for i := 1; i < len(events); i++ {
		if events[i].StartDate.Before(events[best].StartDate.Time) {
			best = i
		}
	}
	return best
}

func (e *eventsStore) GetRest(ctx context.Context, id string) (domain.RestEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.rest {
		if e.rest[i].ID == id {
			return cloneRestEvent(e.rest[i]), nil
		}
	}
	return domain.RestEvent{}, fmt.Errorf("rest event %s: %w", id, storage.ErrNotFound)
}

func (e *eventsStore) ListRest(ctx context.Context) ([]domain.RestEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]domain.RestEvent, 0, len(e.rest))
	for i := range e.rest {
		out = append(out, cloneRestEvent(e.rest[i]))
	}
	return out, nil
}

func (e *eventsStore) ClearRest(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.rest {
		removeFile(e.restPath(e.rest[i].ID))
	}
	e.rest = nil
	return nil
}

func (e *eventsStore) AddSoap(ctx context.Context, ev domain.SoapEvent) error {
	if ev.ID == "" {
		ev.ID = domain.NewID()
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.evictSoap()
	if err := writeFile(e.soapPath(ev.ID), soapEventFile{SchemaVersion: schemaVersion, Kind: "SoapEvent", Event: ev}); err != nil {
		return fmt.Errorf("persist soap event %s: %w", ev.ID, err)
	}
	e.soap = append(e.soap, cloneSoapEvent(ev))
	return nil
}

func (e *eventsStore) evictSoap() {
	if len(e.soap) < e.soapMax {
		return
	}
	oldest := oldestSoapIndex(e.soap)
	id := e.soap[oldest].ID
	removeFile(e.soapPath(id))
	e.soap = append(e.soap[:oldest], e.soap[oldest+1:]...)
}

func oldestSoapIndex(events []domain.SoapEvent) int {
	best := 0
	for i := 1; i < len(events); i++ {
		if events[i].StartDate.Before(events[best].StartDate.Time) {
			best = i
		}
	}
	return best
}

func (e *eventsStore) GetSoap(ctx context.Context, id string) (domain.SoapEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.soap {
		if e.soap[i].ID == id {
			return cloneSoapEvent(e.soap[i]), nil
		}
	}
	return domain.SoapEvent{}, fmt.Errorf("soap event %s: %w", id, storage.ErrNotFound)
}

func (e *eventsStore) ListSoap(ctx context.Context) ([]domain.SoapEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]domain.SoapEvent, 0, len(e.soap))
	for i := range e.soap {
		out = append(out, cloneSoapEvent(e.soap[i]))
	}
	return out, nil
}

func (e *eventsStore) ClearSoap(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.soap {
		removeFile(e.soapPath(e.soap[i].ID))
	}
	e.soap = nil
	return nil
}
