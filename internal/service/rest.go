package service

import (
	"context"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// Read methods return computed fields filled (SPEC.md §5.2).

func (s *Service) CreateRestProject(ctx context.Context, in ProjectInput) (domain.RestProject, error) {
	return domain.RestProject{}, errNotImplemented
}

func (s *Service) RestProject(ctx context.Context, id string) (domain.RestProject, error) {
	return domain.RestProject{}, errNotImplemented
}

func (s *Service) UpdateRestProject(ctx context.Context, id string, in ProjectInput) (domain.RestProject, error) {
	return domain.RestProject{}, errNotImplemented
}

func (s *Service) DeleteRestProject(ctx context.Context, id string) (domain.RestProject, error) {
	return domain.RestProject{}, errNotImplemented
}

func (s *Service) ImportRestDefinition(ctx context.Context, projectID string, defType domain.RestDefinitionType, src Upload, generateResponse bool) error {
	return errNotImplemented
}

// ImportRestProject accepts a CastleMock export XML or a native JSON project file.
func (s *Service) ImportRestProject(ctx context.Context, src Upload) (domain.RestProject, error) {
	return domain.RestProject{}, errNotImplemented
}

func (s *Service) ExportRestProject(ctx context.Context, id string, format ExportFormat) (Export, error) {
	return Export{}, errNotImplemented
}

func (s *Service) CreateRestApplication(ctx context.Context, projectID, name string) (domain.RestApplication, error) {
	return domain.RestApplication{}, errNotImplemented
}

func (s *Service) RestApplication(ctx context.Context, id string) (domain.RestApplication, error) {
	return domain.RestApplication{}, errNotImplemented
}

func (s *Service) UpdateRestApplication(ctx context.Context, id, name string) (domain.RestApplication, error) {
	return domain.RestApplication{}, errNotImplemented
}

func (s *Service) DeleteRestApplication(ctx context.Context, id string) (domain.RestApplication, error) {
	return domain.RestApplication{}, errNotImplemented
}

// SetRestApplicationsStatus sets the status of every method under the applications.
func (s *Service) SetRestApplicationsStatus(ctx context.Context, ids []string, status domain.RestMethodStatus) error {
	return errNotImplemented
}

func (s *Service) SetRestApplicationsForwardedEndpoint(ctx context.Context, ids []string, endpoint string) error {
	return errNotImplemented
}

func (s *Service) CreateRestResource(ctx context.Context, applicationID, name, uri string) (domain.RestResource, error) {
	return domain.RestResource{}, errNotImplemented
}

func (s *Service) RestResource(ctx context.Context, id string) (domain.RestResource, error) {
	return domain.RestResource{}, errNotImplemented
}

func (s *Service) UpdateRestResource(ctx context.Context, id, name, uri string) (domain.RestResource, error) {
	return domain.RestResource{}, errNotImplemented
}

func (s *Service) DeleteRestResource(ctx context.Context, id string) (domain.RestResource, error) {
	return domain.RestResource{}, errNotImplemented
}

// RestResourceParameters returns the {name} variables of the resource uri.
func (s *Service) RestResourceParameters(ctx context.Context, id string) ([]string, error) {
	return nil, errNotImplemented
}

func (s *Service) SetRestResourcesStatus(ctx context.Context, ids []string, status domain.RestMethodStatus) error {
	return errNotImplemented
}

func (s *Service) SetRestResourcesForwardedEndpoint(ctx context.Context, ids []string, endpoint string) error {
	return errNotImplemented
}

func (s *Service) CreateRestMethod(ctx context.Context, resourceID string, in RestMethodCreate) (domain.RestMethod, error) {
	return domain.RestMethod{}, errNotImplemented
}

func (s *Service) RestMethod(ctx context.Context, id string) (domain.RestMethod, error) {
	return domain.RestMethod{}, errNotImplemented
}

func (s *Service) UpdateRestMethod(ctx context.Context, id string, in RestMethodUpdate) (domain.RestMethod, error) {
	return domain.RestMethod{}, errNotImplemented
}

func (s *Service) DeleteRestMethod(ctx context.Context, id string) (domain.RestMethod, error) {
	return domain.RestMethod{}, errNotImplemented
}

func (s *Service) SetRestMethodsStatus(ctx context.Context, ids []string, status domain.RestMethodStatus) error {
	return errNotImplemented
}

func (s *Service) SetRestMethodsForwardedEndpoint(ctx context.Context, ids []string, endpoint string) error {
	return errNotImplemented
}

func (s *Service) CreateRestMockResponse(ctx context.Context, methodID string, in RestMockResponseInput) (domain.RestMockResponse, error) {
	return domain.RestMockResponse{}, errNotImplemented
}

func (s *Service) RestMockResponse(ctx context.Context, id string) (domain.RestMockResponse, error) {
	return domain.RestMockResponse{}, errNotImplemented
}

func (s *Service) UpdateRestMockResponse(ctx context.Context, id string, in RestMockResponseInput) (domain.RestMockResponse, error) {
	return domain.RestMockResponse{}, errNotImplemented
}

func (s *Service) DeleteRestMockResponse(ctx context.Context, id string) (domain.RestMockResponse, error) {
	return domain.RestMockResponse{}, errNotImplemented
}

func (s *Service) DuplicateRestMockResponses(ctx context.Context, ids []string) error {
	return errNotImplemented
}

func (s *Service) SetRestMockResponsesStatus(ctx context.Context, ids []string, status domain.RestMockResponseStatus) error {
	return errNotImplemented
}
