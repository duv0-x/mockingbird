package jsonfile

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// usersRepo implements storage.UserRepository. The whole users.json file is
// rewritten on every change (it holds a handful of rows).
type usersRepo struct {
	s *store
}

func (u usersRepo) List(ctx context.Context) ([]domain.User, error) {
	u.s.mu.Lock()
	defer u.s.mu.Unlock()
	out := make([]domain.User, 0, len(u.s.userOrder))
	for _, id := range u.s.userOrder {
		out = append(out, u.s.users[id])
	}
	return out, nil
}

func (u usersRepo) Get(ctx context.Context, id string) (domain.User, error) {
	u.s.mu.Lock()
	defer u.s.mu.Unlock()
	user, ok := u.s.users[id]
	if !ok {
		return domain.User{}, fmt.Errorf("user %s: %w", id, storage.ErrNotFound)
	}
	return user, nil
}

// GetByUsername matches case-insensitively (AbstractUserService.findByUsername).
func (u usersRepo) GetByUsername(ctx context.Context, username string) (domain.User, error) {
	u.s.mu.Lock()
	defer u.s.mu.Unlock()
	id, ok := u.s.usersByName[lower(username)]
	if !ok {
		return domain.User{}, fmt.Errorf("user %q: %w", username, storage.ErrNotFound)
	}
	return u.s.users[id], nil
}

// Save upserts by id. Username uniqueness is a service-layer rule (T4);
// the store only keeps the case-insensitive lookup index coherent.
func (u usersRepo) Save(ctx context.Context, user domain.User) error {
	u.s.mu.Lock()
	defer u.s.mu.Unlock()
	return u.s.saveUserLocked(user)
}

func (u usersRepo) Delete(ctx context.Context, id string) (domain.User, error) {
	u.s.mu.Lock()
	defer u.s.mu.Unlock()
	user, ok := u.s.users[id]
	if !ok {
		return domain.User{}, fmt.Errorf("user %s: %w", id, storage.ErrNotFound)
	}
	delete(u.s.users, id)
	u.s.userOrder = removeID(u.s.userOrder, id)
	// Drop any username entry pointing at the removed user.
	for name, uid := range u.s.usersByName {
		if uid == id {
			delete(u.s.usersByName, name)
		}
	}
	if err := u.s.persistUsersLocked(); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *store) saveUserLocked(user domain.User) error {
	if user.ID == "" {
		user.ID = domain.NewID()
	}
	if _, ok := s.users[user.ID]; !ok {
		s.userOrder = append(s.userOrder, user.ID)
	}
	// Drop the stale lookup entry if the username changed.
	for name, uid := range s.usersByName {
		if uid == user.ID && !equalFold(name, user.Username) {
			delete(s.usersByName, name)
		}
	}
	s.users[user.ID] = user
	s.usersByName[lower(user.Username)] = user.ID
	return s.persistUsersLocked()
}

func (s *store) persistUsersLocked() error {
	f := usersFile{SchemaVersion: schemaVersion, Users: make([]userFile, 0, len(s.userOrder))}
	for _, id := range s.userOrder {
		f.Users = append(f.Users, userFromDomain(s.users[id]))
	}
	path := filepath.Join(s.dir, "users.json")
	if err := writeFile(path, f); err != nil {
		return fmt.Errorf("persist users: %w", err)
	}
	return nil
}

var _ storage.UserRepository = usersRepo{}
