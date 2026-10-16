package jsonfile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// restRepo implements storage.RestRepository over the in-memory image.
type restRepo struct {
	s *store
}

func (r restRepo) List(ctx context.Context) ([]domain.RestProject, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	out := make([]domain.RestProject, 0, len(r.s.restOrder))
	for _, id := range r.s.restOrder {
		out = append(out, cloneRestProject(r.s.rest[id]))
	}
	return out, nil
}

func (r restRepo) Get(ctx context.Context, projectID string) (domain.RestProject, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p, ok := r.s.rest[projectID]
	if !ok {
		return domain.RestProject{}, fmt.Errorf("rest project %s: %w", projectID, storage.ErrNotFound)
	}
	return cloneRestProject(p), nil
}

func (r restRepo) FindByName(ctx context.Context, name string) (domain.RestProject, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, id := range r.s.restOrder {
		if equalFold(r.s.rest[id].Name, name) {
			return cloneRestProject(r.s.rest[id]), nil
		}
	}
	return domain.RestProject{}, fmt.Errorf("rest project %q: %w", name, storage.ErrNotFound)
}

func (r restRepo) Create(ctx context.Context, p domain.RestProject) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if p.ID == "" {
		p.ID = domain.NewID()
	}
	if _, exists := r.s.rest[p.ID]; exists {
		return fmt.Errorf("%w: rest project %s", storage.ErrConflict, p.ID)
	}
	_, err := r.s.persistRestLocked(p)
	return err
}

func (r restRepo) Update(ctx context.Context, projectID string, fn func(*domain.RestProject) error) (domain.RestProject, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	cur, ok := r.s.rest[projectID]
	if !ok {
		return domain.RestProject{}, fmt.Errorf("rest project %s: %w", projectID, storage.ErrNotFound)
	}
	updated := cloneRestProject(cur)
	if err := fn(&updated); err != nil {
		return domain.RestProject{}, fmt.Errorf("update rest project %s: %w", projectID, err)
	}
	if updated.ID != projectID {
		// Renaming the project id would orphan its file; callers move data
		// via Put/Delete instead.
		return domain.RestProject{}, fmt.Errorf("update rest project %s: changing the id: %w", projectID, storage.ErrConflict)
	}
	stored, err := r.s.persistRestLocked(updated)
	if err != nil {
		return domain.RestProject{}, err
	}
	return cloneRestProject(stored), nil
}

func (r restRepo) Put(ctx context.Context, p domain.RestProject) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if p.ID == "" {
		p.ID = domain.NewID()
	}
	_, err := r.s.persistRestLocked(p)
	return err
}

// persistRestLocked checks ids, writes the project file and replaces the
// in-memory tree. The stored tree is the file round trip of p: a deep copy
// without computed fields and with parent ids set from nesting, i.e.
// exactly what a restart would load.
func (s *store) persistRestLocked(p domain.RestProject) (domain.RestProject, error) {
	if err := s.restChildConflicts(p); err != nil {
		return domain.RestProject{}, err
	}
	f := restProjectFromDomain(p)
	if err := writeFile(s.restProjectFile(p.ID), f); err != nil {
		return domain.RestProject{}, fmt.Errorf("persist rest project %s: %w", p.ID, err)
	}
	stored := restProjectToDomain(f)
	if _, existed := s.rest[p.ID]; !existed {
		s.restOrder = append(s.restOrder, p.ID)
	}
	s.rest[p.ID] = stored
	s.dropRestIndex(p.ID)
	s.putRestIndex(stored)
	return stored, nil
}

func (r restRepo) Delete(ctx context.Context, projectID string) (domain.RestProject, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p, ok := r.s.rest[projectID]
	if !ok {
		return domain.RestProject{}, fmt.Errorf("rest project %s: %w", projectID, storage.ErrNotFound)
	}
	if err := removeFile(r.s.restProjectFile(projectID)); err != nil {
		return domain.RestProject{}, fmt.Errorf("delete rest project %s: %w", projectID, err)
	}
	delete(r.s.rest, projectID)
	r.s.restOrder = removeID(r.s.restOrder, projectID)
	r.s.dropRestIndex(projectID)
	return cloneRestProject(p), nil
}

func (r restRepo) ProjectOf(ctx context.Context, entityID string) (string, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	pid, ok := r.s.restIndex[entityID]
	if !ok {
		return "", fmt.Errorf("rest entity %s: %w", entityID, storage.ErrNotFound)
	}
	return pid, nil
}

// equalFold matches Java's equalsIgnoreCase used by CastleMock name lookups.
func equalFold(a, b string) bool { return strings.EqualFold(a, b) }

func lower(s string) string { return strings.ToLower(s) }

func removeID(ids []string, id string) []string {
	for i, v := range ids {
		if v == id {
			return append(ids[:i], ids[i+1:]...)
		}
	}
	return ids
}

func removeFile(path string) error {
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
