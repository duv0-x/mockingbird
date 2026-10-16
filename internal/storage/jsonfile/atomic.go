package jsonfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// Data dir permissions, SPEC.md §3.1.
const (
	dirPerm       os.FileMode = 0o755
	filePerm      os.FileMode = 0o644
	secretDirPerm os.FileMode = 0o700
)

// marshalFile encodes the native file format: UTF-8, 2-space indent,
// trailing newline, HTML escaping off, fixed struct key order.
func marshalFile(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("encode file: %w", err)
	}
	// json.Encoder terminates the document with exactly one "\n".
	return buf.Bytes(), nil
}

func unmarshalFile(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}

// writeAtomic writes data to path via a temp file in the same directory,
// fsync and rename (SPEC.md §3.1: every write is atomic).
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmpName, path, err)
	}
	tmpName = ""
	return nil
}

func writeFile(path string, v any) error {
	data, err := marshalFile(v)
	if err != nil {
		return err
	}
	return writeAtomic(path, data, filePerm)
}

// generateMissingIDs fills in ids for decoded seed/export trees
// (SPEC.md §3.4: decoders MUST generate missing ids).
func generateMissingIDs(p *domain.RestProject) {
	if p.ID == "" {
		p.ID = domain.NewID()
	}
	for i := range p.Applications {
		app := &p.Applications[i]
		if app.ID == "" {
			app.ID = domain.NewID()
		}
		app.ProjectID = p.ID
		for j := range app.Resources {
			res := &app.Resources[j]
			if res.ID == "" {
				res.ID = domain.NewID()
			}
			res.ApplicationID = app.ID
			for k := range res.Methods {
				m := &res.Methods[k]
				if m.ID == "" {
					m.ID = domain.NewID()
				}
				m.ResourceID = res.ID
				for l := range m.MockResponses {
					mr := &m.MockResponses[l]
					if mr.ID == "" {
						mr.ID = domain.NewID()
					}
					mr.MethodID = m.ID
				}
			}
		}
	}
}

func generateMissingSoapIDs(p *domain.SoapProject) {
	if p.ID == "" {
		p.ID = domain.NewID()
	}
	for i := range p.Resources {
		r := &p.Resources[i]
		if r.ID == "" {
			r.ID = domain.NewID()
		}
		r.ProjectID = p.ID
	}
	for i := range p.Ports {
		port := &p.Ports[i]
		if port.ID == "" {
			port.ID = domain.NewID()
		}
		port.ProjectID = p.ID
		for j := range port.Operations {
			op := &port.Operations[j]
			if op.ID == "" {
				op.ID = domain.NewID()
			}
			op.PortID = port.ID
			for k := range op.MockResponses {
				mr := &op.MockResponses[k]
				if mr.ID == "" {
					mr.ID = domain.NewID()
				}
				mr.OperationID = op.ID
			}
		}
	}
}
