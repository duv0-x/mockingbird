package soap

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// fakeStore is a package-local in-memory storage.Store with what the engine uses.
type fakeStore struct {
	mu       sync.Mutex
	projects map[string]domain.SoapProject
	contents map[string]string
	events   []domain.SoapEvent
	sequence map[string]int
}

func newFakeStore(projects ...domain.SoapProject) *fakeStore {
	s := &fakeStore{projects: map[string]domain.SoapProject{}, contents: map[string]string{}, sequence: map[string]int{}}
	for _, p := range projects {
		s.projects[p.ID] = clone(p)
	}
	return s
}

func clone(p domain.SoapProject) domain.SoapProject {
	b, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}
	var out domain.SoapProject
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}

func (s *fakeStore) Rest() storage.RestRepository    { return nil }
func (s *fakeStore) Soap() storage.SoapRepository    { return fakeSoap{s: s} }
func (s *fakeStore) Events() storage.EventRepository { return fakeEvents{s: s} }
func (s *fakeStore) Users() storage.UserRepository   { return nil }
func (s *fakeStore) State() storage.StateRepository  { return fakeState{s} }
func (s *fakeStore) Close() error                    { return nil }

func (s *fakeStore) project(id string) domain.SoapProject {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clone(s.projects[id])
}

func (s *fakeStore) soapEvents() []domain.SoapEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.SoapEvent{}, s.events...)
}

type fakeSoap struct {
	storage.SoapRepository
	s *fakeStore
}

func (r fakeSoap) Get(_ context.Context, id string) (domain.SoapProject, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p, ok := r.s.projects[id]
	if !ok {
		return domain.SoapProject{}, storage.ErrNotFound
	}
	return clone(p), nil
}

func (r fakeSoap) Update(_ context.Context, id string, fn func(*domain.SoapProject) error) (domain.SoapProject, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p, ok := r.s.projects[id]
	if !ok {
		return domain.SoapProject{}, storage.ErrNotFound
	}
	p = clone(p)
	if err := fn(&p); err != nil {
		return domain.SoapProject{}, err
	}
	r.s.projects[id] = p
	return clone(p), nil
}

func (r fakeSoap) ResourceContent(_ context.Context, _, resourceID string) (string, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	c, ok := r.s.contents[resourceID]
	if !ok {
		return "", storage.ErrNotFound
	}
	return c, nil
}

type fakeEvents struct {
	storage.EventRepository
	s *fakeStore
}

func (e fakeEvents) AddSoap(_ context.Context, ev domain.SoapEvent) error {
	e.s.mu.Lock()
	defer e.s.mu.Unlock()
	e.s.events = append(e.s.events, ev)
	return nil
}

type fakeState struct{ s *fakeStore }

func (st fakeState) SequenceIndex(_ context.Context, key string) (int, error) {
	st.s.mu.Lock()
	defer st.s.mu.Unlock()
	return st.s.sequence[key], nil
}

func (st fakeState) SetSequenceIndex(_ context.Context, key string, index int) error {
	st.s.mu.Lock()
	defer st.s.mu.Unlock()
	st.s.sequence[key] = index
	return nil
}

func (st fakeState) DeleteSequenceIndex(_ context.Context, key string) error {
	st.s.mu.Lock()
	defer st.s.mu.Unlock()
	delete(st.s.sequence, key)
	return nil
}
