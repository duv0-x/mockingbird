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
	"errors"
	"log/slog"

	"github.com/duv0-x/mockingbird/internal/config"
	"github.com/duv0-x/mockingbird/internal/storage"
)

var errNotImplemented = errors.New("service: not implemented (T4)")

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

type Deps struct {
	Config  config.Config
	Store   storage.Store
	Version string
	Logger  *slog.Logger
}

type Service struct {
	deps Deps
}

func New(deps Deps) *Service { return &Service{deps: deps} }
