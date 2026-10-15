package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// All user results are redacted (no password hash).

func (s *Service) Users(ctx context.Context) ([]domain.User, error) {
	users, err := s.deps.Store.Users().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	out := make([]domain.User, 0, len(users))
	for _, u := range users {
		out = append(out, u.Redacted())
	}
	return out, nil
}

func (s *Service) User(ctx context.Context, id string) (domain.User, error) {
	u, err := s.deps.Store.Users().Get(ctx, id)
	if err != nil {
		return domain.User{}, fmt.Errorf("load user %s: %w", id, err)
	}
	return u.Redacted(), nil
}

// checkUsernameFree fails with ErrConflict when another user (other than
// selfID) has the username, compared case-insensitively.
func (s *Service) checkUsernameFree(ctx context.Context, username, selfID string) error {
	other, err := s.deps.Store.Users().GetByUsername(ctx, username)
	switch {
	case err == nil && other.ID != selfID:
		return fmt.Errorf("username %q is already used: %w", username, storage.ErrConflict)
	case err != nil && !errors.Is(err, storage.ErrNotFound):
		return fmt.Errorf("find user by username: %w", err)
	}
	return nil
}

func validateUserInput(in UserInput) error {
	if in.Username == "" {
		return invalid("username", "username cannot be empty")
	}
	if !validEnum(in.Status, userStatuses...) {
		return invalid("status", "unknown user status %q", in.Status)
	}
	if !validEnum(in.Role, roles...) {
		return invalid("role", "unknown role %q", in.Role)
	}
	return nil
}

func (s *Service) CreateUser(ctx context.Context, in UserInput) (domain.User, error) {
	if err := validateUserInput(in); err != nil {
		return domain.User{}, err
	}
	if in.Password == "" {
		return domain.User{}, invalid("password", "password cannot be empty")
	}
	if err := s.checkUsernameFree(ctx, in.Username, ""); err != nil {
		return domain.User{}, err
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return domain.User{}, err
	}
	id, err := s.newUserID(ctx)
	if err != nil {
		return domain.User{}, err
	}
	now := domain.Now()
	u := domain.User{
		ID:       id,
		Username: in.Username,
		Password: hash,
		Email:    in.Email,
		FullName: in.FullName,
		Created:  now,
		Updated:  now,
		Status:   in.Status,
		Role:     in.Role,
	}
	if err := s.deps.Store.Users().Save(ctx, u); err != nil {
		return domain.User{}, fmt.Errorf("save user: %w", err)
	}
	return u.Redacted(), nil
}

func (s *Service) newUserID(ctx context.Context) (string, error) {
	for {
		id := domain.NewID()
		_, err := s.deps.Store.Users().Get(ctx, id)
		if errors.Is(err, storage.ErrNotFound) {
			return id, nil
		}
		if err != nil {
			return "", fmt.Errorf("check user id: %w", err)
		}
	}
}

// isLastAdmin reports whether u is the only user with role ADMIN. Like
// AbstractUserService, the count includes administrators of any status.
func (s *Service) isLastAdmin(ctx context.Context, u domain.User) (bool, error) {
	if u.Role != domain.RoleAdmin {
		return false, nil
	}
	users, err := s.deps.Store.Users().List(ctx)
	if err != nil {
		return false, fmt.Errorf("list users: %w", err)
	}
	admins := 0
	for _, x := range users {
		if x.Role == domain.RoleAdmin {
			admins++
		}
	}
	return admins == 1, nil
}

// UpdateUser keeps the hash when in.Password is empty; the last ADMIN cannot be demoted or deactivated.
func (s *Service) UpdateUser(ctx context.Context, id string, in UserInput) (domain.User, error) {
	u, err := s.deps.Store.Users().Get(ctx, id)
	if err != nil {
		return domain.User{}, fmt.Errorf("load user %s: %w", id, err)
	}
	if err := validateUserInput(in); err != nil {
		return domain.User{}, err
	}
	if err := s.checkUsernameFree(ctx, in.Username, id); err != nil {
		return domain.User{}, err
	}
	last, err := s.isLastAdmin(ctx, u)
	if err != nil {
		return domain.User{}, err
	}
	if last && in.Role != domain.RoleAdmin {
		return domain.User{}, invalid("role", "the last administrator cannot lose the ADMIN role")
	}
	if last && in.Status != domain.UserActive {
		return domain.User{}, invalid("status", "the last administrator cannot be inactivated or locked")
	}
	u.Username = in.Username
	u.Email = in.Email
	u.FullName = in.FullName
	u.Status = in.Status
	u.Role = in.Role
	u.Updated = domain.Now()
	// CastleMock re-hashed the stored hash when no password was sent (D-2).
	if in.Password != "" {
		if u.Password, err = auth.HashPassword(in.Password); err != nil {
			return domain.User{}, err
		}
	}
	if err := s.deps.Store.Users().Save(ctx, u); err != nil {
		return domain.User{}, fmt.Errorf("save user %s: %w", id, err)
	}
	return u.Redacted(), nil
}

func (s *Service) DeleteUser(ctx context.Context, id string) error {
	u, err := s.deps.Store.Users().Get(ctx, id)
	if err != nil {
		return fmt.Errorf("load user %s: %w", id, err)
	}
	last, err := s.isLastAdmin(ctx, u)
	if err != nil {
		return err
	}
	if last {
		return invalid("", "unable to delete the last administrator")
	}
	if _, err := s.deps.Store.Users().Delete(ctx, id); err != nil {
		return fmt.Errorf("delete user %s: %w", id, err)
	}
	return nil
}

func (s *Service) Profile(ctx context.Context, userID string) (domain.User, error) {
	return s.User(ctx, userID)
}

// UpdateProfile changes the principal's username, email, full name and,
// when non-empty, password (UpdateCurrentUserService, D-2). Role and status
// are not editable here.
func (s *Service) UpdateProfile(ctx context.Context, userID string, in ProfileInput) (domain.User, error) {
	u, err := s.deps.Store.Users().Get(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("load user %s: %w", userID, err)
	}
	if in.Username == "" {
		return domain.User{}, invalid("username", "username cannot be empty")
	}
	if err := s.checkUsernameFree(ctx, in.Username, userID); err != nil {
		return domain.User{}, err
	}
	u.Username = in.Username
	u.Email = in.Email
	u.FullName = in.FullName
	u.Updated = domain.Now()
	if in.Password != "" {
		if u.Password, err = auth.HashPassword(in.Password); err != nil {
			return domain.User{}, err
		}
	}
	if err := s.deps.Store.Users().Save(ctx, u); err != nil {
		return domain.User{}, fmt.Errorf("save user %s: %w", userID, err)
	}
	return u.Redacted(), nil
}
