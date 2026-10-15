// Package castlemock reads every CastleMock legacy format (SPEC.md §3.6, §6.3):
//   - the XML data dir (~/.castlemock), converted once into the native store
//     by `mockingbird import --castlemock <dir>`;
//   - the project export files produced by GET .../project/{id}/export
//     (Jackson XmlMapper format), which it also writes for API exports.
//
// Owner: T1. Nothing at runtime depends on the legacy data dir.
package castlemock

import (
	"context"
	"errors"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

type ImportOptions struct {
	// Overwrite replaces projects/users whose id already exists in the target store.
	Overwrite bool
	// SkipEvents ignores rest/event and soap/event.
	SkipEvents bool
}

type Report struct {
	RestProjects int
	SoapProjects int
	Users        int
	RestEvents   int
	SoapEvents   int
	Skipped      []string
	Warnings     []string
}

// ImportDataDir converts a CastleMock base.file.directory into store.
func ImportDataDir(ctx context.Context, dir string, store storage.Store, opts ImportOptions) (Report, error) {
	return Report{}, errors.New("castlemock: not implemented (T1)")
}

// ExportRestProject writes the CastleMock export XML (root <RestExportContainer>).
func ExportRestProject(p domain.RestProject) ([]byte, error) {
	return nil, errors.New("castlemock: not implemented (T1)")
}

// ExportSoapProject writes the CastleMock export XML (root <SoapExportContainer>);
// contents maps SoapResource id -> resource text.
func ExportSoapProject(p domain.SoapProject, contents map[string]string) ([]byte, error) {
	return nil, errors.New("castlemock: not implemented (T1)")
}

// ParseRestExport reads a CastleMock REST export file.
func ParseRestExport(data []byte) (domain.RestProject, error) {
	return domain.RestProject{}, errors.New("castlemock: not implemented (T1)")
}

// ParseSoapExport reads a CastleMock SOAP export file; contents maps resource id -> text.
func ParseSoapExport(data []byte) (domain.SoapProject, map[string]string, error) {
	return domain.SoapProject{}, nil, errors.New("castlemock: not implemented (T1)")
}
