// Package storage defines the persistence contract (SPEC.md §3.3).
//
// Owner: T0 defines the interfaces; T1 implements them in
// internal/storage/jsonfile. Everything else depends only on these interfaces.
//
// The unit of persistence is the whole project tree (one JSON file per
// project). Child entities (applications, resources, methods, ports,
// operations, mock responses) are created, changed and deleted through
// Update callbacks, which run under a per-project lock and are persisted
// atomically. Implementations must be safe for concurrent use and serve reads
// from memory.
package storage

import (
	"context"
	"errors"

	"github.com/duv0-x/mockingbird/internal/domain"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type Store interface {
	Rest() RestRepository
	Soap() SoapRepository
	Events() EventRepository
	Users() UserRepository
	State() StateRepository
	Close() error
}

// RestRepository returns project trees with persisted fields only; computed
// fields (StatusCount, RestMethod.URI, DefaultResponseName,
// CurrentResponseSequenceIndex) are left zero and filled by callers.
type RestRepository interface {
	List(ctx context.Context) ([]domain.RestProject, error)
	Get(ctx context.Context, projectID string) (domain.RestProject, error)
	// FindByName matches case-insensitively (RestProjectFileRepository.findRestProjectWithName).
	FindByName(ctx context.Context, name string) (domain.RestProject, error)
	// Create fails with ErrConflict if the project id, or any child id, already exists.
	Create(ctx context.Context, project domain.RestProject) error
	// Update loads the tree, applies fn and persists the result unless fn returns an error.
	Update(ctx context.Context, projectID string, fn func(*domain.RestProject) error) (domain.RestProject, error)
	// Put creates or fully replaces a project tree (seeding, importer).
	Put(ctx context.Context, project domain.RestProject) error
	Delete(ctx context.Context, projectID string) (domain.RestProject, error)
	// ProjectOf returns the id of the project that contains the application,
	// resource, method or mock response with the given id. The admin API and
	// the mock engine resolve entities by their own id, ignoring parent ids in
	// URLs, exactly like CastleMock (SPEC.md §5.2).
	ProjectOf(ctx context.Context, entityID string) (string, error)
}

// SoapRepository mirrors RestRepository. SoapResource metadata lives in the
// tree; resource content (WSDL/XSD text) is stored separately. Content of
// resources removed from the tree by Update/Put/Delete is deleted too.
type SoapRepository interface {
	List(ctx context.Context) ([]domain.SoapProject, error)
	Get(ctx context.Context, projectID string) (domain.SoapProject, error)
	FindByName(ctx context.Context, name string) (domain.SoapProject, error)
	Create(ctx context.Context, project domain.SoapProject) error
	Update(ctx context.Context, projectID string, fn func(*domain.SoapProject) error) (domain.SoapProject, error)
	Put(ctx context.Context, project domain.SoapProject) error
	Delete(ctx context.Context, projectID string) (domain.SoapProject, error)
	// ProjectOf returns the id of the project that contains the port,
	// operation, mock response or resource with the given id.
	ProjectOf(ctx context.Context, entityID string) (string, error)

	ResourceContent(ctx context.Context, projectID, resourceID string) (string, error)
	PutResourceContent(ctx context.Context, projectID, resourceID, content string) error
}

// EventRepository keeps request/response logs. Add evicts the oldest event of
// the same kind (by StartDate) when the configured maximum is reached
// (rest.event.max / soap.event.max, default 100).
type EventRepository interface {
	AddRest(ctx context.Context, event domain.RestEvent) error
	GetRest(ctx context.Context, id string) (domain.RestEvent, error)
	ListRest(ctx context.Context) ([]domain.RestEvent, error)
	ClearRest(ctx context.Context) error

	AddSoap(ctx context.Context, event domain.SoapEvent) error
	GetSoap(ctx context.Context, id string) (domain.SoapEvent, error)
	ListSoap(ctx context.Context) ([]domain.SoapEvent, error)
	ClearSoap(ctx context.Context) error
}

type UserRepository interface {
	List(ctx context.Context) ([]domain.User, error)
	Get(ctx context.Context, id string) (domain.User, error)
	// GetByUsername matches case-insensitively (AbstractUserService.findByUsername).
	GetByUsername(ctx context.Context, username string) (domain.User, error)
	Save(ctx context.Context, user domain.User) error
	Delete(ctx context.Context, id string) (domain.User, error)
}

// StateRepository holds runtime state kept out of the Git-friendly project
// files: the SEQUENCE strategy cursor of REST methods and SOAP operations.
// Keys are method ids / operation ids; a missing key reads as 0.
type StateRepository interface {
	SequenceIndex(ctx context.Context, key string) (int, error)
	SetSequenceIndex(ctx context.Context, key string, index int) error
	DeleteSequenceIndex(ctx context.Context, key string) error
}
