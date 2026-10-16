// Package servicetest provides an in-memory storage.Store for tests of the
// service, api, auth and web packages. It follows the storage contract
// (SPEC.md §3.3) closely enough for behavior tests: deep copies on read and
// write, child-id conflicts on Create, ProjectOf by scanning, case-insensitive
// name lookups. It is not used in production.
package servicetest

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// MemStore implements storage.Store in memory.
type MemStore struct {
	mu       sync.Mutex
	rest     []domain.RestProject
	soap     []domain.SoapProject
	contents map[string]string // "projectID/resourceID" -> content
	users    []domain.User
	restEv   []domain.RestEvent
	soapEv   []domain.SoapEvent
	state    map[string]int
}

func NewMemStore() *MemStore {
	return &MemStore{contents: map[string]string{}, state: map[string]int{}}
}

func (m *MemStore) Rest() storage.RestRepository    { return memRest{m} }
func (m *MemStore) Soap() storage.SoapRepository    { return memSoap{m} }
func (m *MemStore) Events() storage.EventRepository { return memEvents{m} }
func (m *MemStore) Users() storage.UserRepository   { return memUsers{m} }
func (m *MemStore) State() storage.StateRepository  { return memState{m} }
func (m *MemStore) Close() error                    { return nil }
func (m *MemStore) SequenceIndexes() map[string]int { // test helper
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]int, len(m.state))
	for k, v := range m.state {
		out[k] = v
	}
	return out
}

// clone deep-copies through JSON, like a store that persists and reloads.
func clone[T any](v T) T {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}

func restIDs(p domain.RestProject) []string {
	ids := []string{p.ID}
	for _, a := range p.Applications {
		ids = append(ids, a.ID)
		for _, r := range a.Resources {
			ids = append(ids, r.ID)
			for _, me := range r.Methods {
				ids = append(ids, me.ID)
				for _, mr := range me.MockResponses {
					ids = append(ids, mr.ID)
				}
			}
		}
	}
	return ids
}

func soapIDs(p domain.SoapProject) []string {
	ids := []string{p.ID}
	for _, r := range p.Resources {
		ids = append(ids, r.ID)
	}
	for _, po := range p.Ports {
		ids = append(ids, po.ID)
		for _, o := range po.Operations {
			ids = append(ids, o.ID)
			for _, mr := range o.MockResponses {
				ids = append(ids, mr.ID)
			}
		}
	}
	return ids
}

// conflict reports an id of ids already used outside the project being
// written (skipRest/skipSoap) or repeated within ids.
func (m *MemStore) conflict(ids []string, skipRest, skipSoap string) error {
	existing := m.allIDs(skipRest, skipSoap)
	seen := map[string]bool{}
	for _, id := range ids {
		if existing[id] || seen[id] {
			return fmt.Errorf("id %s: %w", id, storage.ErrConflict)
		}
		seen[id] = true
	}
	return nil
}

func (m *MemStore) allIDs(skipRest, skipSoap string) map[string]bool {
	ids := map[string]bool{}
	for _, p := range m.rest {
		if p.ID == skipRest {
			continue
		}
		for _, id := range restIDs(p) {
			ids[id] = true
		}
	}
	for _, p := range m.soap {
		if p.ID == skipSoap {
			continue
		}
		for _, id := range soapIDs(p) {
			ids[id] = true
		}
	}
	return ids
}

// setRestParents fills parent ids implied by nesting, as the real store does on load.
func setRestParents(p *domain.RestProject) {
	for i := range p.Applications {
		a := &p.Applications[i]
		a.ProjectID = p.ID
		for j := range a.Resources {
			r := &a.Resources[j]
			r.ApplicationID = a.ID
			for k := range r.Methods {
				me := &r.Methods[k]
				me.ResourceID = r.ID
				for l := range me.MockResponses {
					me.MockResponses[l].MethodID = me.ID
				}
			}
		}
	}
}

func setSoapParents(p *domain.SoapProject) {
	for i := range p.Resources {
		p.Resources[i].ProjectID = p.ID
		p.Resources[i].Content = ""
	}
	for i := range p.Ports {
		po := &p.Ports[i]
		po.ProjectID = p.ID
		for j := range po.Operations {
			o := &po.Operations[j]
			o.PortID = po.ID
			for k := range o.MockResponses {
				o.MockResponses[k].OperationID = o.ID
			}
		}
	}
}

type memRest struct{ m *MemStore }

func (r memRest) List(ctx context.Context) ([]domain.RestProject, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	return clone(r.m.rest), nil
}

func (r memRest) index(id string) int {
	return slices.IndexFunc(r.m.rest, func(p domain.RestProject) bool { return p.ID == id })
}

func (r memRest) Get(ctx context.Context, id string) (domain.RestProject, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	i := r.index(id)
	if i < 0 {
		return domain.RestProject{}, fmt.Errorf("rest project %s: %w", id, storage.ErrNotFound)
	}
	return clone(r.m.rest[i]), nil
}

func (r memRest) FindByName(ctx context.Context, name string) (domain.RestProject, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	for _, p := range r.m.rest {
		if strings.EqualFold(p.Name, name) {
			return clone(p), nil
		}
	}
	return domain.RestProject{}, storage.ErrNotFound
}

func (r memRest) Create(ctx context.Context, p domain.RestProject) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	if err := r.m.conflict(restIDs(p), "", ""); err != nil {
		return err
	}
	p = clone(p)
	setRestParents(&p)
	r.m.rest = append(r.m.rest, p)
	return nil
}

func (r memRest) Update(ctx context.Context, id string, fn func(*domain.RestProject) error) (domain.RestProject, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	i := r.index(id)
	if i < 0 {
		return domain.RestProject{}, fmt.Errorf("rest project %s: %w", id, storage.ErrNotFound)
	}
	p := clone(r.m.rest[i])
	if err := fn(&p); err != nil {
		return domain.RestProject{}, err
	}
	p.ID = id
	if err := r.m.conflict(restIDs(p), id, ""); err != nil {
		return domain.RestProject{}, err
	}
	setRestParents(&p)
	r.m.rest[i] = clone(p)
	return clone(p), nil
}

func (r memRest) Put(ctx context.Context, p domain.RestProject) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	if err := r.m.conflict(restIDs(p), p.ID, ""); err != nil {
		return err
	}
	p = clone(p)
	setRestParents(&p)
	if i := r.index(p.ID); i >= 0 {
		r.m.rest[i] = p
	} else {
		r.m.rest = append(r.m.rest, p)
	}
	return nil
}

func (r memRest) Delete(ctx context.Context, id string) (domain.RestProject, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	i := r.index(id)
	if i < 0 {
		return domain.RestProject{}, fmt.Errorf("rest project %s: %w", id, storage.ErrNotFound)
	}
	p := r.m.rest[i]
	r.m.rest = slices.Delete(r.m.rest, i, i+1)
	return p, nil
}

func (r memRest) ProjectOf(ctx context.Context, entityID string) (string, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	for _, p := range r.m.rest {
		if slices.Contains(restIDs(p)[1:], entityID) {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("rest entity %s: %w", entityID, storage.ErrNotFound)
}

type memSoap struct{ m *MemStore }

func (s memSoap) index(id string) int {
	return slices.IndexFunc(s.m.soap, func(p domain.SoapProject) bool { return p.ID == id })
}

func (s memSoap) List(ctx context.Context) ([]domain.SoapProject, error) {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	return clone(s.m.soap), nil
}

func (s memSoap) Get(ctx context.Context, id string) (domain.SoapProject, error) {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	i := s.index(id)
	if i < 0 {
		return domain.SoapProject{}, fmt.Errorf("soap project %s: %w", id, storage.ErrNotFound)
	}
	return clone(s.m.soap[i]), nil
}

func (s memSoap) FindByName(ctx context.Context, name string) (domain.SoapProject, error) {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	for _, p := range s.m.soap {
		if strings.EqualFold(p.Name, name) {
			return clone(p), nil
		}
	}
	return domain.SoapProject{}, storage.ErrNotFound
}

func (s memSoap) Create(ctx context.Context, p domain.SoapProject) error {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	if err := s.m.conflict(soapIDs(p), "", ""); err != nil {
		return err
	}
	p = clone(p)
	setSoapParents(&p)
	s.m.soap = append(s.m.soap, p)
	return nil
}

func (s memSoap) dropRemovedContents(before, after domain.SoapProject) {
	for _, r := range before.Resources {
		if after.ID != before.ID || after.Resource(r.ID) == nil {
			delete(s.m.contents, before.ID+"/"+r.ID)
		}
	}
}

func (s memSoap) Update(ctx context.Context, id string, fn func(*domain.SoapProject) error) (domain.SoapProject, error) {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	i := s.index(id)
	if i < 0 {
		return domain.SoapProject{}, fmt.Errorf("soap project %s: %w", id, storage.ErrNotFound)
	}
	p := clone(s.m.soap[i])
	if err := fn(&p); err != nil {
		return domain.SoapProject{}, err
	}
	p.ID = id
	if err := s.m.conflict(soapIDs(p), "", id); err != nil {
		return domain.SoapProject{}, err
	}
	setSoapParents(&p)
	s.dropRemovedContents(s.m.soap[i], p)
	s.m.soap[i] = clone(p)
	return clone(p), nil
}

func (s memSoap) Put(ctx context.Context, p domain.SoapProject) error {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	if err := s.m.conflict(soapIDs(p), "", p.ID); err != nil {
		return err
	}
	p = clone(p)
	setSoapParents(&p)
	if i := s.index(p.ID); i >= 0 {
		s.dropRemovedContents(s.m.soap[i], p)
		s.m.soap[i] = p
	} else {
		s.m.soap = append(s.m.soap, p)
	}
	return nil
}

func (s memSoap) Delete(ctx context.Context, id string) (domain.SoapProject, error) {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	i := s.index(id)
	if i < 0 {
		return domain.SoapProject{}, fmt.Errorf("soap project %s: %w", id, storage.ErrNotFound)
	}
	p := s.m.soap[i]
	s.dropRemovedContents(p, domain.SoapProject{})
	s.m.soap = slices.Delete(s.m.soap, i, i+1)
	return p, nil
}

func (s memSoap) ProjectOf(ctx context.Context, entityID string) (string, error) {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	for _, p := range s.m.soap {
		if slices.Contains(soapIDs(p)[1:], entityID) {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("soap entity %s: %w", entityID, storage.ErrNotFound)
}

func (s memSoap) ResourceContent(ctx context.Context, projectID, resourceID string) (string, error) {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	c, ok := s.m.contents[projectID+"/"+resourceID]
	if !ok {
		return "", fmt.Errorf("resource %s: %w", resourceID, storage.ErrNotFound)
	}
	return c, nil
}

func (s memSoap) PutResourceContent(ctx context.Context, projectID, resourceID, content string) error {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	i := s.index(projectID)
	if i < 0 || s.m.soap[i].Resource(resourceID) == nil {
		return fmt.Errorf("resource %s: %w", resourceID, storage.ErrNotFound)
	}
	s.m.contents[projectID+"/"+resourceID] = content
	return nil
}

type memEvents struct{ m *MemStore }

func (e memEvents) AddRest(ctx context.Context, ev domain.RestEvent) error {
	e.m.mu.Lock()
	defer e.m.mu.Unlock()
	e.m.restEv = append(e.m.restEv, clone(ev))
	return nil
}

func (e memEvents) GetRest(ctx context.Context, id string) (domain.RestEvent, error) {
	e.m.mu.Lock()
	defer e.m.mu.Unlock()
	for _, ev := range e.m.restEv {
		if ev.ID == id {
			return clone(ev), nil
		}
	}
	return domain.RestEvent{}, storage.ErrNotFound
}

func (e memEvents) ListRest(ctx context.Context) ([]domain.RestEvent, error) {
	e.m.mu.Lock()
	defer e.m.mu.Unlock()
	return clone(e.m.restEv), nil
}

func (e memEvents) ClearRest(ctx context.Context) error {
	e.m.mu.Lock()
	defer e.m.mu.Unlock()
	e.m.restEv = nil
	return nil
}

func (e memEvents) AddSoap(ctx context.Context, ev domain.SoapEvent) error {
	e.m.mu.Lock()
	defer e.m.mu.Unlock()
	e.m.soapEv = append(e.m.soapEv, clone(ev))
	return nil
}

func (e memEvents) GetSoap(ctx context.Context, id string) (domain.SoapEvent, error) {
	e.m.mu.Lock()
	defer e.m.mu.Unlock()
	for _, ev := range e.m.soapEv {
		if ev.ID == id {
			return clone(ev), nil
		}
	}
	return domain.SoapEvent{}, storage.ErrNotFound
}

func (e memEvents) ListSoap(ctx context.Context) ([]domain.SoapEvent, error) {
	e.m.mu.Lock()
	defer e.m.mu.Unlock()
	return clone(e.m.soapEv), nil
}

func (e memEvents) ClearSoap(ctx context.Context) error {
	e.m.mu.Lock()
	defer e.m.mu.Unlock()
	e.m.soapEv = nil
	return nil
}

type memUsers struct{ m *MemStore }

func (u memUsers) List(ctx context.Context) ([]domain.User, error) {
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	return clone(u.m.users), nil
}

func (u memUsers) Get(ctx context.Context, id string) (domain.User, error) {
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	for _, x := range u.m.users {
		if x.ID == id {
			return x, nil
		}
	}
	return domain.User{}, fmt.Errorf("user %s: %w", id, storage.ErrNotFound)
}

func (u memUsers) GetByUsername(ctx context.Context, username string) (domain.User, error) {
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	for _, x := range u.m.users {
		if strings.EqualFold(x.Username, username) {
			return x, nil
		}
	}
	return domain.User{}, fmt.Errorf("user %s: %w", username, storage.ErrNotFound)
}

func (u memUsers) Save(ctx context.Context, user domain.User) error {
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	for i, x := range u.m.users {
		if x.ID == user.ID {
			u.m.users[i] = user
			return nil
		}
	}
	u.m.users = append(u.m.users, user)
	return nil
}

func (u memUsers) Delete(ctx context.Context, id string) (domain.User, error) {
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	for i, x := range u.m.users {
		if x.ID == id {
			u.m.users = slices.Delete(u.m.users, i, i+1)
			return x, nil
		}
	}
	return domain.User{}, fmt.Errorf("user %s: %w", id, storage.ErrNotFound)
}

type memState struct{ m *MemStore }

func (s memState) SequenceIndex(ctx context.Context, key string) (int, error) {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	return s.m.state[key], nil
}

func (s memState) SetSequenceIndex(ctx context.Context, key string, index int) error {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	s.m.state[key] = index
	return nil
}

func (s memState) DeleteSequenceIndex(ctx context.Context, key string) error {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	delete(s.m.state, key)
	return nil
}
