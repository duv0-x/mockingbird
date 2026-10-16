package jsonfile

import (
	"context"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

type SeedMode string

const (
	SeedCreateOnly SeedMode = "create-only"
	SeedOverwrite  SeedMode = "overwrite"
)

type Options struct {
	DataDir      string
	SeedDir      string
	SeedMode     SeedMode
	RestEventMax int
	SoapEventMax int
	// AdminPassword is used only when the users file is empty and the default
	// admin is created; "" means "admin".
	AdminPassword string
	// Version is recorded in meta.json ("createdBy"); "" means "dev".
	Version string
}

// Open loads (creating if needed) the data dir, applies seeds and returns a
// ready Store.
func Open(ctx context.Context, opts Options) (storage.Store, error) {
	s, err := openStore(ctx, opts)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *store) Rest() storage.RestRepository    { return restRepo{s: s} }
func (s *store) Soap() storage.SoapRepository    { return soapRepo{s: s} }
func (s *store) Events() storage.EventRepository { return &s.events }
func (s *store) Users() storage.UserRepository   { return usersRepo{s: s} }
func (s *store) State() storage.StateRepository  { return &s.state }

var _ storage.Store = (*store)(nil)

// EncodeRestProject writes the native project file format (SPEC.md §3.2.2),
// used for storage, seeds and GET .../export?format=json.
func EncodeRestProject(p domain.RestProject) ([]byte, error) {
	return marshalFile(restProjectFromDomain(p))
}

// DecodeRestProject reads the native REST project format, generating missing
// ids and filling defaults for absent optional fields (SPEC.md §3.4).
func DecodeRestProject(data []byte) (domain.RestProject, error) {
	f, err := decodeRestProjectFile(data)
	if err != nil {
		return domain.RestProject{}, err
	}
	p := restProjectToDomain(f)
	generateMissingIDs(&p)
	return p, nil
}

// EncodeSoapProject writes the native SOAP project file; contents (resource
// id -> text) are inlined as resources[].content, as in seeds and JSON
// exports.
func EncodeSoapProject(p domain.SoapProject, contents map[string]string) ([]byte, error) {
	f := soapProjectFromDomain(p)
	for id, content := range contents {
		if res := f.resource(id); res != nil {
			res.Content = content
		}
	}
	return marshalFile(f)
}

// DecodeSoapProject reads the native SOAP project format; inline
// resources[].content is returned separately so the store can move it to
// the content files (SPEC.md §3.2.3).
func DecodeSoapProject(data []byte) (domain.SoapProject, map[string]string, error) {
	f, err := decodeSoapProjectFile(data)
	if err != nil {
		return domain.SoapProject{}, nil, err
	}
	contents := map[string]string{}
	for i := range f.Resources {
		if f.Resources[i].Content != "" {
			contents[f.Resources[i].ID] = f.Resources[i].Content
			f.Resources[i].Content = ""
		}
	}
	p := soapProjectToDomain(f)
	generateMissingSoapIDs(&p)
	return p, contents, nil
}

func (f *soapProjectFile) resource(id string) *soapResourceFile {
	for i := range f.Resources {
		if f.Resources[i].ID == id {
			return &f.Resources[i]
		}
	}
	return nil
}
