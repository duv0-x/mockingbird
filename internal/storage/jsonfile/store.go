package jsonfile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// store is the in-memory image of the data dir. Everything is loaded at
// Open; reads are served from memory and never touch disk (SPEC.md §3.1).
//
// Locking: s.mu serializes all project and user access, which makes every
// per-project change atomic and serialized (the SPEC.md §3.3 contract).
// Events and sequence state have their own locks because mock traffic hits
// them on every request. Update callbacks run while s.mu is held and must
// not call back into the project or user repositories (deadlock).
type store struct {
	opts Options
	dir  string

	mu        sync.Mutex
	rest      map[string]domain.RestProject
	restOrder []string
	restIndex map[string]string // entity id (incl. project id) -> project id
	soap      map[string]domain.SoapProject
	soapOrder []string
	soapIndex map[string]string
	contents  map[string]map[string]string // soap project id -> resource id -> text

	users       map[string]domain.User
	userOrder   []string
	usersByName map[string]string // lowercase username -> user id

	events eventsStore
	state  stateStore
}

// openStore loads (creating if needed) the data dir, applies seeds and
// returns the ready store.
func openStore(ctx context.Context, opts Options) (*store, error) {
	if opts.SeedMode == "" {
		opts.SeedMode = SeedCreateOnly
	}
	if opts.SeedMode != SeedCreateOnly && opts.SeedMode != SeedOverwrite {
		return nil, fmt.Errorf("jsonfile: unsupported seed mode %q", opts.SeedMode)
	}
	if opts.RestEventMax == 0 {
		opts.RestEventMax = defaultEventMax
	}
	if opts.SoapEventMax == 0 {
		opts.SoapEventMax = defaultEventMax
	}
	if opts.Version == "" {
		opts.Version = "dev"
	}

	s := &store{
		opts:        opts,
		dir:         opts.DataDir,
		rest:        map[string]domain.RestProject{},
		restIndex:   map[string]string{},
		soap:        map[string]domain.SoapProject{},
		soapIndex:   map[string]string{},
		contents:    map[string]map[string]string{},
		users:       map[string]domain.User{},
		usersByName: map[string]string{},
	}
	s.events = newEventsStore(opts.DataDir, opts.RestEventMax, opts.SoapEventMax)
	s.state = newStateStore(filepath.Join(opts.DataDir, "state", "sequence.json"), stateFlushDelay)

	for _, d := range []struct {
		path string
		perm os.FileMode
	}{
		{opts.DataDir, dirPerm},
		{s.restDir(), dirPerm},
		{s.soapDir(), dirPerm},
		{s.restEventsDir(), dirPerm},
		{s.soapEventsDir(), dirPerm},
		{filepath.Join(opts.DataDir, "state"), dirPerm},
		{filepath.Join(opts.DataDir, "secrets"), secretDirPerm},
	} {
		if err := os.MkdirAll(d.path, d.perm); err != nil {
			return nil, fmt.Errorf("jsonfile: create %s: %w", d.path, err)
		}
	}

	if err := s.loadMeta(); err != nil {
		return nil, err
	}
	if err := s.loadUsers(); err != nil {
		return nil, err
	}
	if err := s.loadRestProjects(); err != nil {
		return nil, err
	}
	if err := s.loadSoapProjects(); err != nil {
		return nil, err
	}
	if err := s.events.load(); err != nil {
		return nil, err
	}
	if err := s.state.load(); err != nil {
		return nil, err
	}
	if len(s.users) == 0 {
		// UserFileRepository.postInitiate: an empty store gets a default admin.
		if err := s.createDefaultAdmin(); err != nil {
			return nil, err
		}
	}
	if opts.SeedDir != "" {
		if err := s.applySeeds(ctx); err != nil {
			return nil, fmt.Errorf("jsonfile: seed %s: %w", opts.SeedDir, err)
		}
	}
	return s, nil
}

const (
	defaultEventMax = 100
	stateFlushDelay = time.Second
	adminUsername   = "admin"
)

func (s *store) Close() error {
	return s.state.close()
}

// --- data dir layout helpers ---

func (s *store) restDir() string { return filepath.Join(s.dir, "projects", "rest") }
func (s *store) soapDir() string { return filepath.Join(s.dir, "projects", "soap") }
func (s *store) restProjectFile(id string) string {
	return filepath.Join(s.restDir(), id+".json")
}
func (s *store) soapProjectFile(id string) string {
	return filepath.Join(s.soapDir(), id+".json")
}
func (s *store) soapResourceDir(projectID string) string {
	return filepath.Join(s.soapDir(), projectID, "resources")
}
func (s *store) soapContentFile(projectID, resourceID, typ string) string {
	ext := ".wsdl"
	if typ == string(domain.SoapResourceSchema) {
		ext = ".xsd"
	}
	return filepath.Join(s.soapResourceDir(projectID), resourceID+ext)
}
func (s *store) restEventsDir() string { return filepath.Join(s.dir, "events", "rest") }
func (s *store) soapEventsDir() string { return filepath.Join(s.dir, "events", "soap") }
func (s *store) restEventFile(id string) string {
	return filepath.Join(s.restEventsDir(), id+".json")
}
func (s *store) soapEventFile(id string) string {
	return filepath.Join(s.soapEventsDir(), id+".json")
}

// --- loading ---

func (s *store) loadMeta() error {
	path := filepath.Join(s.dir, "meta.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return writeFile(path, metaFile{SchemaVersion: schemaVersion, CreatedBy: "mockingbird " + s.opts.Version})
		}
		return fmt.Errorf("jsonfile: read meta.json: %w", err)
	}
	var meta metaFile
	if err := unmarshalFile(data, &meta); err != nil {
		// A corrupt meta file is not fatal; a future legit write replaces it.
		slog.Error("jsonfile: skipping unparsable file", "path", path, "err", err)
		return nil
	}
	if err := checkSchemaVersion(meta.SchemaVersion, "meta.json"); err != nil {
		return fmt.Errorf("jsonfile: %w", err)
	}
	return nil
}

// loadJSONDir reads every *.ext file in dir, sorted by name. read receives
// the file base name (without extension) and its contents.
func loadJSONDir(dir, ext string, read func(base string, data []byte) error) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ext) {
			continue
		}
		name := e.Name()
		base := strings.TrimSuffix(name, ext)
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if err := read(base, data); err != nil {
			return fmt.Errorf("%s: %w", filepath.Join(dir, name), err)
		}
	}
	return nil
}

func (s *store) loadRestProjects() error {
	return loadJSONDir(s.restDir(), ".json", func(base string, data []byte) error {
		f, err := decodeRestProjectFile(data)
		if err != nil {
			if errors.Is(err, ErrUnsupportedSchema) {
				return err
			}
			slog.Error("jsonfile: skipping unparsable file", "path", s.restProjectFile(base), "err", err)
			return nil
		}
		if f.ID == "" || f.ID != base {
			slog.Error("jsonfile: skipping project file with mismatched id", "path", s.restProjectFile(base), "id", f.ID)
			return nil
		}
		p := restProjectToDomain(f)
		if err := s.restChildConflicts(p); err != nil {
			slog.Error("jsonfile: skipping project with duplicate child id", "path", s.restProjectFile(base), "err", err)
			return nil
		}
		s.putRestIndex(p)
		s.rest[f.ID] = p
		s.restOrder = append(s.restOrder, f.ID)
		return nil
	})
}

func (s *store) loadSoapProjects() error {
	return loadJSONDir(s.soapDir(), ".json", func(base string, data []byte) error {
		f, err := decodeSoapProjectFile(data)
		if err != nil {
			if errors.Is(err, ErrUnsupportedSchema) {
				return err
			}
			slog.Error("jsonfile: skipping unparsable file", "path", s.soapProjectFile(base), "err", err)
			return nil
		}
		if f.ID == "" || f.ID != base {
			slog.Error("jsonfile: skipping project file with mismatched id", "path", s.soapProjectFile(base), "id", f.ID)
			return nil
		}
		p := soapProjectToDomain(f)
		if err := s.soapChildConflicts(p); err != nil {
			slog.Error("jsonfile: skipping project with duplicate child id", "path", s.soapProjectFile(base), "err", err)
			return nil
		}
		s.putSoapIndex(p)
		s.soap[f.ID] = p
		s.soapOrder = append(s.soapOrder, f.ID)
		// Resource contents live next to the project file, in
		// projects/soap/<id>/resources/.
		resDir := s.soapResourceDir(f.ID)
		contentFiles, err := os.ReadDir(resDir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("read %s: %w", resDir, err)
		}
		for _, e := range contentFiles {
			if e.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(resDir, e.Name()))
			if err != nil {
				return fmt.Errorf("read %s: %w", e.Name(), err)
			}
			rid := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			if s.contents[f.ID] == nil {
				s.contents[f.ID] = map[string]string{}
			}
			s.contents[f.ID][rid] = string(data)
		}
		return nil
	})
}

func (s *store) loadUsers() error {
	data, err := os.ReadFile(filepath.Join(s.dir, "users.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("jsonfile: read users.json: %w", err)
	}
	f, err := decodeUsersFile(data)
	if err != nil {
		if errors.Is(err, ErrUnsupportedSchema) {
			return err
		}
		slog.Error("jsonfile: skipping unparsable file", "path", filepath.Join(s.dir, "users.json"), "err", err)
		return nil
	}
	for _, uf := range f.Users {
		if uf.ID == "" {
			slog.Error("jsonfile: skipping user without id", "path", filepath.Join(s.dir, "users.json"), "username", uf.Username)
			continue
		}
		u := userToDomain(uf)
		s.users[u.ID] = u
		s.userOrder = append(s.userOrder, u.ID)
		s.usersByName[strings.ToLower(u.Username)] = u.ID
	}
	return nil
}

func (s *store) createDefaultAdmin() error {
	password := s.opts.AdminPassword
	if password == "" {
		password = adminUsername
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("jsonfile: hash admin password: %w", err)
	}
	now := domain.Now()
	u := domain.User{
		ID:       domain.NewID(),
		Username: adminUsername,
		Password: string(hash),
		Email:    "",
		FullName: "Admin Admin",
		Created:  now,
		Updated:  now,
		Status:   domain.UserActive,
		Role:     domain.RoleAdmin,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveUserLocked(u)
}

// --- child-id indexes (storage.Store contract §3.3) ---

func restChildIDs(p domain.RestProject) []string {
	ids := []string{p.ID}
	for i := range p.Applications {
		a := &p.Applications[i]
		ids = append(ids, a.ID)
		for j := range a.Resources {
			r := &a.Resources[j]
			ids = append(ids, r.ID)
			for k := range r.Methods {
				m := &r.Methods[k]
				ids = append(ids, m.ID)
				for l := range m.MockResponses {
					ids = append(ids, m.MockResponses[l].ID)
				}
			}
		}
	}
	return ids
}

func soapChildIDs(p domain.SoapProject) []string {
	ids := []string{p.ID}
	for i := range p.Ports {
		port := &p.Ports[i]
		ids = append(ids, port.ID)
		for j := range port.Operations {
			op := &port.Operations[j]
			ids = append(ids, op.ID)
			for k := range op.MockResponses {
				ids = append(ids, op.MockResponses[k].ID)
			}
		}
	}
	for i := range p.Resources {
		ids = append(ids, p.Resources[i].ID)
	}
	return ids
}

// restChildConflicts rejects a tree whose ids (project or child) already
// belong to another project, REST or SOAP ("anywhere in the store",
// SPEC.md §3.3), or appear twice in the tree; ids of p itself are fine.
func (s *store) restChildConflicts(p domain.RestProject) error {
	return s.idConflicts(restChildIDs(p), p.ID, s.restIndex, s.soapIndex)
}

func (s *store) soapChildConflicts(p domain.SoapProject) error {
	return s.idConflicts(soapChildIDs(p), p.ID, s.soapIndex, s.restIndex)
}

func (s *store) idConflicts(ids []string, projectID string, same, other map[string]string) error {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id == "" {
			return fmt.Errorf("%w: empty id in project %s", storage.ErrConflict, projectID)
		}
		if seen[id] {
			return fmt.Errorf("%w: %s used twice in project %s", storage.ErrConflict, id, projectID)
		}
		seen[id] = true
		if pid, ok := same[id]; ok && pid != projectID {
			return fmt.Errorf("%w: %s already used in project %s", storage.ErrConflict, id, pid)
		}
		if pid, ok := other[id]; ok {
			return fmt.Errorf("%w: %s already used in project %s", storage.ErrConflict, id, pid)
		}
	}
	return nil
}

func (s *store) putRestIndex(p domain.RestProject) {
	for _, id := range restChildIDs(p) {
		s.restIndex[id] = p.ID
	}
}

func (s *store) putSoapIndex(p domain.SoapProject) {
	for _, id := range soapChildIDs(p) {
		s.soapIndex[id] = p.ID
	}
}

func (s *store) dropRestIndex(projectID string) {
	for id, pid := range s.restIndex {
		if pid == projectID {
			delete(s.restIndex, id)
		}
	}
}

func (s *store) dropSoapIndex(projectID string) {
	for id, pid := range s.soapIndex {
		if pid == projectID {
			delete(s.soapIndex, id)
		}
	}
}
