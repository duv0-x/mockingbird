// Package wsdl parses WSDL 1.1 documents into SOAP ports/operations and merges
// them into a SOAP project (SPEC.md §6.2).
//
// Owner: T3. Consumer: internal/api (T4) for /wsdl/file and /wsdl/link.
package wsdl

import (
	"context"
	"errors"
	"net/http"

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
	// IncludeImports follows wsdl:import locations recursively (link import only).
	IncludeImports bool
	HTTPClient     *http.Client
}

// Import converts src and merges it into the project (CreateSoapPortsService semantics),
// replacing the project's WSDL/WSDL_IMPORT resources.
func Import(ctx context.Context, repo storage.SoapRepository, projectID string, src Source, opts Options) error {
	return errors.New("wsdl: not implemented (T3)")
}
