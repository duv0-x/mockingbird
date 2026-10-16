package jsonfile

import (
	"context"
	"fmt"
	"os"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// soapRepo implements storage.SoapRepository. Resource content is kept in
// memory too (reads never hit disk) and mirrored to
// projects/soap/<id>/resources/<rid>.wsdl|.xsd.
type soapRepo struct {
	s *store
}

func (r soapRepo) List(ctx context.Context) ([]domain.SoapProject, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	out := make([]domain.SoapProject, 0, len(r.s.soapOrder))
	for _, id := range r.s.soapOrder {
		out = append(out, cloneSoapProject(r.s.soap[id]))
	}
	return out, nil
}

func (r soapRepo) Get(ctx context.Context, projectID string) (domain.SoapProject, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p, ok := r.s.soap[projectID]
	if !ok {
		return domain.SoapProject{}, fmt.Errorf("soap project %s: %w", projectID, storage.ErrNotFound)
	}
	return cloneSoapProject(p), nil
}

func (r soapRepo) FindByName(ctx context.Context, name string) (domain.SoapProject, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, id := range r.s.soapOrder {
		if equalFold(r.s.soap[id].Name, name) {
			return cloneSoapProject(r.s.soap[id]), nil
		}
	}
	return domain.SoapProject{}, fmt.Errorf("soap project %q: %w", name, storage.ErrNotFound)
}

func (r soapRepo) Create(ctx context.Context, p domain.SoapProject) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if p.ID == "" {
		p.ID = domain.NewID()
	}
	if _, exists := r.s.soap[p.ID]; exists {
		return fmt.Errorf("%w: soap project %s", storage.ErrConflict, p.ID)
	}
	_, err := r.s.persistSoapLocked(p)
	return err
}

func (r soapRepo) Update(ctx context.Context, projectID string, fn func(*domain.SoapProject) error) (domain.SoapProject, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	cur, ok := r.s.soap[projectID]
	if !ok {
		return domain.SoapProject{}, fmt.Errorf("soap project %s: %w", projectID, storage.ErrNotFound)
	}
	updated := cloneSoapProject(cur)
	if err := fn(&updated); err != nil {
		return domain.SoapProject{}, fmt.Errorf("update soap project %s: %w", projectID, err)
	}
	if updated.ID != projectID {
		return domain.SoapProject{}, fmt.Errorf("update soap project %s: changing the id: %w", projectID, storage.ErrConflict)
	}
	stored, err := r.s.persistSoapLocked(updated)
	if err != nil {
		return domain.SoapProject{}, err
	}
	return cloneSoapProject(stored), nil
}

func (r soapRepo) Put(ctx context.Context, p domain.SoapProject) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if p.ID == "" {
		p.ID = domain.NewID()
	}
	_, err := r.s.persistSoapLocked(p)
	return err
}

// persistSoapLocked mirrors persistRestLocked; content files of resources
// that left the tree are deleted.
func (s *store) persistSoapLocked(p domain.SoapProject) (domain.SoapProject, error) {
	if err := s.soapChildConflicts(p); err != nil {
		return domain.SoapProject{}, err
	}
	f := soapProjectFromDomain(p)
	if err := writeFile(s.soapProjectFile(p.ID), f); err != nil {
		return domain.SoapProject{}, fmt.Errorf("persist soap project %s: %w", p.ID, err)
	}
	stored := soapProjectToDomain(f)
	old, existed := s.soap[p.ID]
	if !existed {
		s.soapOrder = append(s.soapOrder, p.ID)
	}
	s.dropSoapContents(p.ID, old, stored)
	s.soap[p.ID] = stored
	s.dropSoapIndex(p.ID)
	s.putSoapIndex(stored)
	return stored, nil
}

func (r soapRepo) Delete(ctx context.Context, projectID string) (domain.SoapProject, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p, ok := r.s.soap[projectID]
	if !ok {
		return domain.SoapProject{}, fmt.Errorf("soap project %s: %w", projectID, storage.ErrNotFound)
	}
	if err := removeFile(r.s.soapProjectFile(projectID)); err != nil {
		return domain.SoapProject{}, fmt.Errorf("delete soap project %s: %w", projectID, err)
	}
	if err := os.RemoveAll(r.s.soapResourceDir(projectID)); err != nil {
		return domain.SoapProject{}, fmt.Errorf("delete soap resources of %s: %w", projectID, err)
	}
	delete(r.s.soap, projectID)
	delete(r.s.contents, projectID)
	r.s.soapOrder = removeID(r.s.soapOrder, projectID)
	r.s.dropSoapIndex(projectID)
	return cloneSoapProject(p), nil
}

func (r soapRepo) ProjectOf(ctx context.Context, entityID string) (string, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	pid, ok := r.s.soapIndex[entityID]
	if !ok {
		return "", fmt.Errorf("soap entity %s: %w", entityID, storage.ErrNotFound)
	}
	return pid, nil
}

func (r soapRepo) ResourceContent(ctx context.Context, projectID, resourceID string) (string, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	content, ok := r.s.contents[projectID][resourceID]
	if !ok {
		return "", fmt.Errorf("soap resource %s/%s: %w", projectID, resourceID, storage.ErrNotFound)
	}
	return content, nil
}

func (r soapRepo) PutResourceContent(ctx context.Context, projectID, resourceID, content string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p, ok := r.s.soap[projectID]
	if !ok {
		return fmt.Errorf("soap project %s: %w", projectID, storage.ErrNotFound)
	}
	res := p.Resource(resourceID)
	if res == nil {
		return fmt.Errorf("soap resource %s/%s: %w", projectID, resourceID, storage.ErrNotFound)
	}
	if err := os.MkdirAll(r.s.soapResourceDir(projectID), dirPerm); err != nil {
		return fmt.Errorf("persist soap resource %s/%s: %w", projectID, resourceID, err)
	}
	if err := writeAtomic(r.s.soapContentFile(projectID, resourceID, string(res.Type)), []byte(content), filePerm); err != nil {
		return fmt.Errorf("persist soap resource %s/%s: %w", projectID, resourceID, err)
	}
	if r.s.contents[projectID] == nil {
		r.s.contents[projectID] = map[string]string{}
	}
	r.s.contents[projectID][resourceID] = content
	return nil
}

// dropSoapContents deletes the content files of resources that were removed
// from the tree (SPEC.md §3.2.3). The store must be locked.
func (s *store) dropSoapContents(projectID string, old, updated domain.SoapProject) {
	removed := map[string]*domain.SoapResource{}
	for i := range old.Resources {
		removed[old.Resources[i].ID] = &old.Resources[i]
	}
	for i := range updated.Resources {
		delete(removed, updated.Resources[i].ID)
	}
	for id, res := range removed {
		removeFile(s.soapContentFile(projectID, id, string(res.Type)))
		if s.contents[projectID] != nil {
			delete(s.contents[projectID], id)
		}
	}
}
