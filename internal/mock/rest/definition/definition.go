// Package definition imports Swagger 2, OpenAPI 3, RAML 0.8/1.0 and WADL
// definitions into a REST project (SPEC.md §6.1).
//
// Owner: T2. Consumer: internal/api (T4) for /definition/file and /definition/link.
package definition

import (
	"context"
	"errors"
	"net/http"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// Source is either uploaded bytes (Filename+Data) or a URL to download.
type Source struct {
	Filename string
	Data     []byte
	URL      string
}

type Options struct {
	GenerateResponse bool
	HTTPClient       *http.Client
}

// Import converts src and merges it into the project (ImportRestDefinitionService semantics).
func Import(ctx context.Context, repo storage.RestRepository, projectID string, defType domain.RestDefinitionType, src Source, opts Options) error {
	return errors.New("definition: not implemented (T2)")
}
