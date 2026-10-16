package jsonfile

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/importer/castlemock"
)

// applySeeds walks the read-only seed dir (SPEC.md §3.5). A seed file that
// fails to parse aborts startup; unsupported files are ignored at DEBUG.
func (s *store) applySeeds(ctx context.Context) error {
	if info, err := os.Stat(s.opts.SeedDir); err != nil {
		return err
	} else if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", s.opts.SeedDir)
	}
	return s.seedWalk(ctx, s.opts.SeedDir)
}

// seedWalk walks recursively, following symlinks to regular files and
// skipping dot names (Kubernetes ConfigMap ..data dirs) to avoid duplicates.
func (s *store) seedWalk(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		if e.Type()&fs.ModeSymlink != 0 {
			info, err := os.Stat(path)
			if err != nil {
				slog.Warn("jsonfile: seed: cannot resolve symlink", "path", path, "err", err)
				continue
			}
			switch {
			case info.IsDir():
				if err := s.seedWalk(ctx, path); err != nil {
					return err
				}
			case info.Mode().IsRegular():
				if err := s.seedFile(ctx, path); err != nil {
					return err
				}
			default:
				slog.Debug("jsonfile: seed: ignoring non-regular file", "path", path)
			}
			continue
		}
		switch {
		case e.IsDir():
			if err := s.seedWalk(ctx, path); err != nil {
				return err
			}
		case e.Type().IsRegular():
			if err := s.seedFile(ctx, path); err != nil {
				return err
			}
		default:
			slog.Debug("jsonfile: seed: ignoring non-regular file", "path", path)
		}
	}
	return nil
}

func (s *store) seedFile(ctx context.Context, path string) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var probe struct {
			Kind string `json:"kind"`
		}
		if err := unmarshalFile(data, &probe); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		switch probe.Kind {
		case "RestProject":
			p, err := DecodeRestProject(data)
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			return s.seedRestProject(ctx, p, path)
		case "SoapProject":
			p, contents, err := DecodeSoapProject(data)
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			return s.seedSoapProject(ctx, p, contents, path)
		default:
			return fmt.Errorf("parse %s: unsupported kind %q (want RestProject or SoapProject)", path, probe.Kind)
		}
	case ".xml":
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		root, err := xmlRootElement(data)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		switch root {
		case "RestExportContainer":
			p, err := castlemock.ParseRestExport(data)
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			return s.seedRestProject(ctx, p, path)
		case "SoapExportContainer":
			p, contents, err := castlemock.ParseSoapExport(data)
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			return s.seedSoapProject(ctx, p, contents, path)
		default:
			return fmt.Errorf("parse %s: unsupported root element %q (want RestExportContainer or SoapExportContainer)", path, root)
		}
	default:
		slog.Debug("jsonfile: seed: ignoring file", "path", path)
		return nil
	}
}

func xmlRootElement(data []byte) (string, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		if start, ok := tok.(xml.StartElement); ok {
			return start.Name.Local, nil
		}
	}
}

func (s *store) seedRestProject(ctx context.Context, p domain.RestProject, path string) error {
	if s.opts.SeedMode != SeedOverwrite && !s.canSeedRest(p, path) {
		return nil
	}
	return s.Rest().Put(ctx, p)
}

func (s *store) seedSoapProject(ctx context.Context, p domain.SoapProject, contents map[string]string, path string) error {
	if s.opts.SeedMode != SeedOverwrite && !s.canSeedSoap(p, path) {
		return nil
	}
	if err := s.Soap().Put(ctx, p); err != nil {
		return err
	}
	return s.putContents(ctx, p, contents)
}

func (s *store) putContents(ctx context.Context, p domain.SoapProject, contents map[string]string) error {
	for id, content := range contents {
		if err := s.Soap().PutResourceContent(ctx, p.ID, id, content); err != nil {
			return err
		}
	}
	return nil
}

// canSeedRest reports whether a create-only seed may be added: an existing
// project id or name, or a child id already used elsewhere, skips the seed
// with a WARN (SPEC.md §3.5).
func (s *store) canSeedRest(p domain.RestProject, path string) bool {
	ctx := context.Background()
	if _, err := s.Rest().Get(ctx, p.ID); err == nil {
		slog.Warn("jsonfile: seed: project id already exists, skipping", "path", path, "id", p.ID)
		return false
	}
	if _, err := s.Rest().FindByName(ctx, p.Name); err == nil {
		slog.Warn("jsonfile: seed: project name already exists, skipping", "path", path, "name", p.Name)
		return false
	}
	s.mu.Lock()
	err := s.restChildConflicts(p)
	s.mu.Unlock()
	if err != nil {
		slog.Warn("jsonfile: seed: id already used, skipping", "path", path, "err", err)
		return false
	}
	return true
}

func (s *store) canSeedSoap(p domain.SoapProject, path string) bool {
	ctx := context.Background()
	if _, err := s.Soap().Get(ctx, p.ID); err == nil {
		slog.Warn("jsonfile: seed: project id already exists, skipping", "path", path, "id", p.ID)
		return false
	}
	if _, err := s.Soap().FindByName(ctx, p.Name); err == nil {
		slog.Warn("jsonfile: seed: project name already exists, skipping", "path", path, "name", p.Name)
		return false
	}
	s.mu.Lock()
	err := s.soapChildConflicts(p)
	s.mu.Unlock()
	if err != nil {
		slog.Warn("jsonfile: seed: id already used, skipping", "path", path, "err", err)
		return false
	}
	return true
}
