package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// fakeStore is a package-local in-memory storage.Store with just what the
// engine uses.
type fakeStore struct {
	mu       sync.Mutex
	projects map[string]domain.RestProject
	events   []domain.RestEvent
	sequence map[string]int
}

func newFakeStore(projects ...domain.RestProject) *fakeStore {
	s := &fakeStore{projects: map[string]domain.RestProject{}, sequence: map[string]int{}}
	for _, p := range projects {
		s.projects[p.ID] = clone(p)
	}
	return s
}

func clone(p domain.RestProject) domain.RestProject {
	b, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}
	var out domain.RestProject
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}

func (s *fakeStore) Rest() storage.RestRepository    { return fakeRest{s} }
func (s *fakeStore) Soap() storage.SoapRepository    { return nil }
func (s *fakeStore) Events() storage.EventRepository { return fakeEvents{s} }
func (s *fakeStore) Users() storage.UserRepository   { return nil }
func (s *fakeStore) State() storage.StateRepository  { return fakeState{s} }
func (s *fakeStore) Close() error                    { return nil }

func (s *fakeStore) project(id string) domain.RestProject {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clone(s.projects[id])
}

func (s *fakeStore) restEvents() []domain.RestEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.RestEvent{}, s.events...)
}

type fakeRest struct{ s *fakeStore }

func (r fakeRest) List(context.Context) ([]domain.RestProject, error) { return nil, nil }
func (r fakeRest) Get(_ context.Context, id string) (domain.RestProject, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p, ok := r.s.projects[id]
	if !ok {
		return domain.RestProject{}, storage.ErrNotFound
	}
	return clone(p), nil
}
func (r fakeRest) FindByName(context.Context, string) (domain.RestProject, error) {
	return domain.RestProject{}, storage.ErrNotFound
}
func (r fakeRest) Create(_ context.Context, p domain.RestProject) error {
	return r.Put(context.Background(), p)
}
func (r fakeRest) Update(_ context.Context, id string, fn func(*domain.RestProject) error) (domain.RestProject, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p, ok := r.s.projects[id]
	if !ok {
		return domain.RestProject{}, storage.ErrNotFound
	}
	p = clone(p)
	if err := fn(&p); err != nil {
		return domain.RestProject{}, err
	}
	r.s.projects[id] = p
	return clone(p), nil
}
func (r fakeRest) Put(_ context.Context, p domain.RestProject) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.projects[p.ID] = clone(p)
	return nil
}
func (r fakeRest) Delete(context.Context, string) (domain.RestProject, error) {
	return domain.RestProject{}, storage.ErrNotFound
}
func (r fakeRest) ProjectOf(_ context.Context, entityID string) (string, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for id, p := range r.s.projects {
		for _, a := range p.Applications {
			if a.ID == entityID {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("entity %s: %w", entityID, storage.ErrNotFound)
}

type fakeEvents struct{ s *fakeStore }

func (e fakeEvents) AddRest(_ context.Context, ev domain.RestEvent) error {
	e.s.mu.Lock()
	defer e.s.mu.Unlock()
	e.s.events = append(e.s.events, ev)
	return nil
}
func (e fakeEvents) GetRest(context.Context, string) (domain.RestEvent, error) {
	return domain.RestEvent{}, storage.ErrNotFound
}
func (e fakeEvents) ListRest(context.Context) ([]domain.RestEvent, error) { return nil, nil }
func (e fakeEvents) ClearRest(context.Context) error                      { return nil }
func (e fakeEvents) AddSoap(context.Context, domain.SoapEvent) error      { return nil }
func (e fakeEvents) GetSoap(context.Context, string) (domain.SoapEvent, error) {
	return domain.SoapEvent{}, storage.ErrNotFound
}
func (e fakeEvents) ListSoap(context.Context) ([]domain.SoapEvent, error) { return nil, nil }
func (e fakeEvents) ClearSoap(context.Context) error                      { return nil }

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
