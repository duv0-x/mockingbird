package service

import (
	"context"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// All user results are redacted (no password hash).

func (s *Service) Users(ctx context.Context) ([]domain.User, error) {
	return nil, errNotImplemented
}

func (s *Service) User(ctx context.Context, id string) (domain.User, error) {
	return domain.User{}, errNotImplemented
}

func (s *Service) CreateUser(ctx context.Context, in UserInput) (domain.User, error) {
	return domain.User{}, errNotImplemented
}

// UpdateUser keeps the hash when in.Password is empty; the last ACTIVE ADMIN cannot be demoted or deactivated.
func (s *Service) UpdateUser(ctx context.Context, id string, in UserInput) (domain.User, error) {
	return domain.User{}, errNotImplemented
}

func (s *Service) DeleteUser(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *Service) Profile(ctx context.Context, userID string) (domain.User, error) {
	return domain.User{}, errNotImplemented
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, in ProfileInput) (domain.User, error) {
	return domain.User{}, errNotImplemented
}
