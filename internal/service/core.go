package service

import (
	"context"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// Projects returns REST and SOAP projects (GET /core/project).
func (s *Service) Projects(ctx context.Context) ([]domain.OverviewProject, error) {
	return nil, errNotImplemented
}

// Events lists all events, REST then SOAP (GET /core/event).
func (s *Service) Events(ctx context.Context) ([]domain.OverviewEvent, error) {
	return nil, errNotImplemented
}

// ClearEvents deletes every REST and SOAP event.
func (s *Service) ClearEvents(ctx context.Context) error {
	return errNotImplemented
}

func (s *Service) RestEvent(ctx context.Context, id string) (domain.RestEvent, error) {
	return domain.RestEvent{}, errNotImplemented
}

func (s *Service) SoapEvent(ctx context.Context, id string) (domain.SoapEvent, error) {
	return domain.SoapEvent{}, errNotImplemented
}

// ValidateExpression renders responseBody with requestBody as BODY input.
func (s *Service) ValidateExpression(ctx context.Context, requestBody, responseBody string) (string, error) {
	return "", errNotImplemented
}

func (s *Service) SystemInformation(ctx context.Context) (domain.SystemInformation, error) {
	return domain.SystemInformation{}, errNotImplemented
}

func (s *Service) Version() string {
	return s.deps.Version
}
