package service

import (
	"context"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// Read methods return computed fields filled (SPEC.md §5.2).

func (s *Service) CreateSoapProject(ctx context.Context, in ProjectInput) (domain.SoapProject, error) {
	return domain.SoapProject{}, errNotImplemented
}

func (s *Service) SoapProject(ctx context.Context, id string) (domain.SoapProject, error) {
	return domain.SoapProject{}, errNotImplemented
}

func (s *Service) UpdateSoapProject(ctx context.Context, id string, in ProjectInput) (domain.SoapProject, error) {
	return domain.SoapProject{}, errNotImplemented
}

func (s *Service) DeleteSoapProject(ctx context.Context, id string) (domain.SoapProject, error) {
	return domain.SoapProject{}, errNotImplemented
}

func (s *Service) ImportWSDL(ctx context.Context, projectID string, src Upload, generateResponse, includeImports bool) error {
	return errNotImplemented
}

func (s *Service) ImportSoapProject(ctx context.Context, src Upload) (domain.SoapProject, error) {
	return domain.SoapProject{}, errNotImplemented
}

func (s *Service) ExportSoapProject(ctx context.Context, id string, format ExportFormat) (Export, error) {
	return Export{}, errNotImplemented
}

func (s *Service) SetSoapPortsStatus(ctx context.Context, ids []string, status domain.SoapOperationStatus) error {
	return errNotImplemented
}

func (s *Service) SetSoapPortsForwardedEndpoint(ctx context.Context, ids []string, endpoint string) error {
	return errNotImplemented
}

func (s *Service) SoapPort(ctx context.Context, id string) (domain.SoapPort, error) {
	return domain.SoapPort{}, errNotImplemented
}

func (s *Service) UpdateSoapPort(ctx context.Context, id, uri string) (domain.SoapPort, error) {
	return domain.SoapPort{}, errNotImplemented
}

func (s *Service) DeleteSoapPort(ctx context.Context, id string) (domain.SoapPort, error) {
	return domain.SoapPort{}, errNotImplemented
}

func (s *Service) SetSoapOperationsStatus(ctx context.Context, ids []string, status domain.SoapOperationStatus) error {
	return errNotImplemented
}

func (s *Service) SetSoapOperationsForwardedEndpoint(ctx context.Context, ids []string, endpoint string) error {
	return errNotImplemented
}

func (s *Service) SoapOperation(ctx context.Context, id string) (domain.SoapOperation, error) {
	return domain.SoapOperation{}, errNotImplemented
}

func (s *Service) UpdateSoapOperation(ctx context.Context, id string, in SoapOperationUpdate) (domain.SoapOperation, error) {
	return domain.SoapOperation{}, errNotImplemented
}

func (s *Service) CreateSoapMockResponse(ctx context.Context, operationID string, in SoapMockResponseInput) (domain.SoapMockResponse, error) {
	return domain.SoapMockResponse{}, errNotImplemented
}

func (s *Service) SoapMockResponse(ctx context.Context, id string) (domain.SoapMockResponse, error) {
	return domain.SoapMockResponse{}, errNotImplemented
}

func (s *Service) UpdateSoapMockResponse(ctx context.Context, id string, in SoapMockResponseInput) (domain.SoapMockResponse, error) {
	return domain.SoapMockResponse{}, errNotImplemented
}

func (s *Service) DeleteSoapMockResponse(ctx context.Context, id string) (domain.SoapMockResponse, error) {
	return domain.SoapMockResponse{}, errNotImplemented
}

func (s *Service) DuplicateSoapMockResponses(ctx context.Context, ids []string) error {
	return errNotImplemented
}

func (s *Service) SetSoapMockResponsesStatus(ctx context.Context, ids []string, status domain.SoapMockResponseStatus) error {
	return errNotImplemented
}

func (s *Service) SoapResource(ctx context.Context, id string) (domain.SoapResource, error) {
	return domain.SoapResource{}, errNotImplemented
}

func (s *Service) SoapResourceContent(ctx context.Context, id string) (string, error) {
	return "", errNotImplemented
}

// ImportSoapResource stores a WSDL/XSD; a WSDL replaces the project's existing WSDL resources.
func (s *Service) ImportSoapResource(ctx context.Context, projectID string, resourceType domain.SoapResourceType, src Upload) (domain.SoapResource, error) {
	return domain.SoapResource{}, errNotImplemented
}
