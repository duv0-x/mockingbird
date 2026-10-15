// Package service holds every admin business operation (CastleMock's
// service/** layer): validation, defaults, cascades, computed read fields and
// the import/export entry points. It is the single source of behavior for both
// the JSON admin API (internal/api) and the templ+htmx UI (internal/web), so
// neither of them talks to storage directly for writes (SPEC.md §5.0).
//
// Owner: T4. Consumers: T4 (api), T5a/T5b (web).
//
// Entity lookups are by the entity's own id (storage.ProjectOf), never by the
// parent ids of a URL. Errors: storage.ErrNotFound, storage.ErrConflict, or a
// *ValidationError; everything else is internal.
package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/duv0-x/mockingbird/internal/config"
	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// ValidationError is returned for invalid input (HTTP 400 / form error).
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return e.Field + ": " + e.Message
}

func invalid(field, format string, args ...any) error {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

func notFound(kind, id string) error {
	return fmt.Errorf("%s %s: %w", kind, id, storage.ErrNotFound)
}

type Deps struct {
	Config  config.Config
	Store   storage.Store
	Version string
	Logger  *slog.Logger
	// HTTPClient downloads definitions and WSDLs given by URL. Nil builds one
	// from Config (ForwardTimeout, TLSVerify).
	HTTPClient *http.Client
}

type Service struct {
	deps Deps
}

func New(deps Deps) *Service {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.HTTPClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if !deps.Config.TLSVerify {
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // MOCKINGBIRD_TLS_VERIFY=false
		}
		deps.HTTPClient = &http.Client{Timeout: deps.Config.ForwardTimeout, Transport: transport}
	}
	return &Service{deps: deps}
}

func (s *Service) rest() storage.RestRepository { return s.deps.Store.Rest() }
func (s *Service) soap() storage.SoapRepository { return s.deps.Store.Soap() }

// newIDs returns n ids unused by any project or child entity. They are
// generated before entering an Update callback so that no repository call
// happens while the store holds a project lock.
func (s *Service) newIDs(ctx context.Context, n int) ([]string, error) {
	ids := make([]string, 0, n)
	for len(ids) < n {
		id := domain.NewID()
		if slices.Contains(ids, id) {
			continue
		}
		used, err := s.idUsed(ctx, id)
		if err != nil {
			return nil, err
		}
		if !used {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (s *Service) newID(ctx context.Context) (string, error) {
	ids, err := s.newIDs(ctx, 1)
	if err != nil {
		return "", err
	}
	return ids[0], nil
}

func (s *Service) idUsed(ctx context.Context, id string) (bool, error) {
	checks := []func() error{
		func() error { _, err := s.rest().ProjectOf(ctx, id); return err },
		func() error { _, err := s.soap().ProjectOf(ctx, id); return err },
		func() error { _, err := s.rest().Get(ctx, id); return err },
		func() error { _, err := s.soap().Get(ctx, id); return err },
	}
	for _, check := range checks {
		err := check()
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, storage.ErrNotFound) {
			return false, fmt.Errorf("check id %s: %w", id, err)
		}
	}
	return false, nil
}

func (s *Service) deleteSequenceIndexes(ctx context.Context, keys []string) {
	for _, k := range keys {
		if err := s.deps.Store.State().DeleteSequenceIndex(ctx, k); err != nil {
			s.deps.Logger.Warn("delete sequence index", "key", k, "err", err)
		}
	}
}

func (s *Service) sequenceIndex(ctx context.Context, key string) int {
	i, err := s.deps.Store.State().SequenceIndex(ctx, key)
	if err != nil {
		s.deps.Logger.Warn("read sequence index", "key", key, "err", err)
		return 0
	}
	return i
}

func validEnum[T ~string](v T, all ...T) bool { return slices.Contains(all, v) }

var (
	restStrategies = []domain.RestResponseStrategy{domain.RestStrategyRandom, domain.RestStrategySequence,
		domain.RestStrategyQueryMatch, domain.RestStrategyXPath, domain.RestStrategyJSONPath, domain.RestStrategyHeaderQueryMatch}
	soapStrategies   = []domain.SoapResponseStrategy{domain.SoapStrategyRandom, domain.SoapStrategySequence, domain.SoapStrategyXPathInput}
	identifyStrategy = []domain.SoapOperationIdentifyStrategy{domain.IdentifyByElement, domain.IdentifyByElementNamespace}
	restMRStatuses   = []domain.RestMockResponseStatus{domain.RestMockResponseEnabled, domain.RestMockResponseDisabled}
	soapMRStatuses   = []domain.SoapMockResponseStatus{domain.SoapMockResponseEnabled, domain.SoapMockResponseDisabled}
	definitionTypes  = []domain.RestDefinitionType{domain.RestDefinitionSwagger, domain.RestDefinitionOpenAPI, domain.RestDefinitionWADL, domain.RestDefinitionRAML}
	resourceTypes    = []domain.SoapResourceType{domain.SoapResourceWSDL, domain.SoapResourceSchema, domain.SoapResourceWSDLImport}
	userStatuses     = []domain.UserStatus{domain.UserActive, domain.UserInactive, domain.UserLocked}
	roles            = []domain.Role{domain.RoleReader, domain.RoleModifier, domain.RoleAdmin}
)

func emptyIfNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// firstNonBlank returns the first byte that is not whitespace (or a UTF-8
// BOM), used to sniff uploaded project files (SPEC.md §6.3).
func firstNonBlank(data []byte) byte {
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case ' ', '\t', '\r', '\n':
			continue
		case 0xEF:
			if i+2 < len(data) && data[i+1] == 0xBB && data[i+2] == 0xBF {
				i += 2
				continue
			}
		}
		return data[i]
	}
	return 0
}
