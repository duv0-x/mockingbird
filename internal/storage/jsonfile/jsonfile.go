package jsonfile

import (
	"context"
	"errors"

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
}

// Open loads (creating if needed) the data dir, applies seeds and returns a
// ready Store.
func Open(ctx context.Context, opts Options) (storage.Store, error) {
	return nil, errors.New("jsonfile: not implemented (T1)")
}

// EncodeRestProject writes the native project file format (SPEC.md §3.2.2),
// used for storage, seeds and GET .../export?format=json.
func EncodeRestProject(p domain.RestProject) ([]byte, error) {
	return nil, errors.New("jsonfile: not implemented (T1)")
}

func DecodeRestProject(data []byte) (domain.RestProject, error) {
	return domain.RestProject{}, errors.New("jsonfile: not implemented (T1)")
}

// EncodeSoapProject writes the native SOAP project file; contents (resource id
// -> text) are inlined as resources[].content.
func EncodeSoapProject(p domain.SoapProject, contents map[string]string) ([]byte, error) {
	return nil, errors.New("jsonfile: not implemented (T1)")
}

func DecodeSoapProject(data []byte) (domain.SoapProject, map[string]string, error) {
	return domain.SoapProject{}, nil, errors.New("jsonfile: not implemented (T1)")
}
