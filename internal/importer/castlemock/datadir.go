package castlemock

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// Data dir layout (app/src/main/resources/file-repository.properties).
// configuration/, token/ and tmpFiles/ are deliberately ignored.
var layout = struct {
	user, restProject, restApplication, restResource, restMethod, restResponse, restEvent string
	soapProject, soapPort, soapOperation, soapResponse, soapResource, soapEvent           string
}{
	user:            "user",
	restProject:     "rest/project/v2",
	restApplication: "rest/application/v2",
	restResource:    "rest/resource/v2",
	restMethod:      "rest/method/v2",
	restResponse:    "rest/response/v2",
	restEvent:       "rest/event/v1",
	soapProject:     "soap/project/v2",
	soapPort:        "soap/port/v2",
	soapOperation:   "soap/operation/v2",
	soapResponse:    "soap/response/v2",
	soapResource:    "soap/resource/v2",
	soapEvent:       "soap/event/v1",
}

// Pre-1.x project layouts that mockingbird does not read.
var legacyV1Dirs = []string{
	"rest/project/v1", "rest/application/v1", "rest/resource/v1", "rest/method/v1", "rest/response/v1",
	"soap/project/v1", "soap/port/v1", "soap/operation/v1", "soap/response/v1", "soap/resource/v1",
}

// ImportDataDir converts a CastleMock base.file.directory into store
// (SPEC.md §3.6.4). The source dir is only read. Files that do not parse and
// entities whose parent is missing are listed in Report.Skipped; existing
// projects and users are skipped unless opts.Overwrite.
func ImportDataDir(ctx context.Context, dir string, store storage.Store, opts ImportOptions) (Report, error) {
	im := &dataDirImport{dir: dir, store: store, opts: opts}
	for _, d := range legacyV1Dirs {
		if _, err := os.Stat(filepath.Join(dir, d)); err == nil {
			im.warn("%s: pre-1.x data (v1) is not supported and was ignored", d)
		}
	}
	steps := []func(context.Context) error{im.importUsers, im.importRest, im.importSoap}
	if !opts.SkipEvents {
		steps = append(steps, im.importEvents)
	}
	for _, step := range steps {
		if err := step(ctx); err != nil {
			return im.report, err
		}
	}
	return im.report, nil
}

type dataDirImport struct {
	dir    string
	store  storage.Store
	opts   ImportOptions
	report Report
}

func (im *dataDirImport) warn(format string, args ...any) {
	im.report.Warnings = append(im.report.Warnings, fmt.Sprintf(format, args...))
}

func (im *dataDirImport) skip(format string, args ...any) {
	im.report.Skipped = append(im.report.Skipped, fmt.Sprintf(format, args...))
}

// readEntities parses every <id><ext> file of a layout dir whose root
// element is root, in file name order. Unparsable files are skipped.
func (im *dataDirImport) readEntities(sub, ext, root string, fn func(n *node, file string) error) error {
	dir := filepath.Join(im.dir, sub)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ext) {
			continue
		}
		file := filepath.Join(sub, e.Name())
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", file, err)
		}
		n, err := parseXML(data)
		if err != nil || n.name != root {
			if root == "soapResource" {
				// WSDL_IMPORT content shares this dir and extension (§3.6.2).
				continue
			}
			if err == nil {
				err = fmt.Errorf("root element <%s>, want <%s>", n.name, root)
			}
			im.skip("%s: %v", file, err)
			continue
		}
		if err := fn(n, file); err != nil {
			im.skip("%s: %v", file, err)
		}
	}
	return nil
}

func (im *dataDirImport) importRest(ctx context.Context) error {
	projects := map[string]*domain.RestProject{}
	var order []string
	parts := restParts{sequence: map[string]int{}}
	steps := []struct {
		sub, ext, root string
		fn             func(n *node, file string) error
	}{
		{layout.restProject, ".prj", "restProject", func(n *node, file string) error {
			fields, err := projectFields(n)
			if err != nil {
				return err
			}
			if fields.ID == "" {
				return errors.New("project without id")
			}
			projects[fields.ID] = &domain.RestProject{Project: fields, Applications: []domain.RestApplication{}}
			order = append(order, fields.ID)
			return nil
		}},
		{layout.restApplication, ".app", "restApplication", func(n *node, file string) error {
			a, parent := restApplication(n)
			parts.apps = append(parts.apps, parented[domain.RestApplication]{a, parent, file})
			return nil
		}},
		{layout.restResource, ".res", "restResource", func(n *node, file string) error {
			r, parent := restResource(n)
			parts.resources = append(parts.resources, parented[domain.RestResource]{r, parent, file})
			return nil
		}},
		{layout.restMethod, ".met", "restMethod", func(n *node, file string) error {
			m, parent, seq := restMethod(n)
			parts.methods = append(parts.methods, parented[domain.RestMethod]{m, parent, file})
			parts.sequence[m.ID] = seq
			return nil
		}},
		{layout.restResponse, ".rest", "restMockResponse", func(n *node, file string) error {
			mr, parent := restMockResponse(n)
			parts.responses = append(parts.responses, parented[domain.RestMockResponse]{mr, parent, file})
			return nil
		}},
	}
	for _, st := range steps {
		if err := im.readEntities(st.sub, st.ext, st.root, st.fn); err != nil {
			return err
		}
	}
	for _, o := range assembleRest(projects, parts) {
		im.skip("%s", o)
	}
	for _, id := range order {
		p := projects[id]
		setRestParents(p)
		ok, err := im.putRest(ctx, *p)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		im.report.RestProjects++
		for _, a := range p.Applications {
			for _, r := range a.Resources {
				for _, m := range r.Methods {
					if err := im.setSequence(ctx, m.ID, parts.sequence[m.ID]); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (im *dataDirImport) setSequence(ctx context.Context, key string, index int) error {
	if index == 0 {
		return im.store.State().DeleteSequenceIndex(ctx, key)
	}
	return im.store.State().SetSequenceIndex(ctx, key, index)
}

// putRest creates the project, or replaces it with Overwrite. It reports
// false when the project was skipped.
func (im *dataDirImport) putRest(ctx context.Context, p domain.RestProject) (bool, error) {
	_, err := im.store.Rest().Get(ctx, p.ID)
	exists := err == nil
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return false, fmt.Errorf("check rest project %s: %w", p.ID, err)
	}
	switch {
	case exists && !im.opts.Overwrite:
		im.skip("rest project %s (%s): already exists (use --overwrite)", p.ID, p.Name)
		return false, nil
	case exists:
		err = im.store.Rest().Put(ctx, p)
	default:
		err = im.store.Rest().Create(ctx, p)
	}
	if errors.Is(err, storage.ErrConflict) {
		im.skip("rest project %s (%s): %v", p.ID, p.Name, err)
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("store rest project %s: %w", p.ID, err)
	}
	return true, nil
}

func (im *dataDirImport) importSoap(ctx context.Context) error {
	projects := map[string]*domain.SoapProject{}
	var order []string
	parts := soapParts{sequence: map[string]int{}}
	steps := []struct {
		sub, ext, root string
		fn             func(n *node, file string) error
	}{
		{layout.soapProject, ".prj", "soapProject", func(n *node, file string) error {
			fields, err := projectFields(n)
			if err != nil {
				return err
			}
			if fields.ID == "" {
				return errors.New("project without id")
			}
			projects[fields.ID] = &domain.SoapProject{Project: fields, Ports: []domain.SoapPort{}, Resources: []domain.SoapResource{}}
			order = append(order, fields.ID)
			return nil
		}},
		{layout.soapPort, ".port", "soapPort", func(n *node, file string) error {
			po, parent := soapPort(n)
			parts.ports = append(parts.ports, parented[domain.SoapPort]{po, parent, file})
			return nil
		}},
		{layout.soapOperation, ".opr", "soapOperation", func(n *node, file string) error {
			o, parent, seq := soapOperation(n)
			parts.operations = append(parts.operations, parented[domain.SoapOperation]{o, parent, file})
			parts.sequence[o.ID] = seq
			return nil
		}},
		{layout.soapResponse, ".res", "soapMockResponse", func(n *node, file string) error {
			mr, parent := soapMockResponse(n)
			parts.responses = append(parts.responses, parented[domain.SoapMockResponse]{mr, parent, file})
			return nil
		}},
		{layout.soapResource, ".rsc", "soapResource", func(n *node, file string) error {
			r, parent := soapResource(n)
			parts.resources = append(parts.resources, parented[domain.SoapResource]{r, parent, file})
			return nil
		}},
	}
	for _, st := range steps {
		if err := im.readEntities(st.sub, st.ext, st.root, st.fn); err != nil {
			return err
		}
	}
	for _, o := range assembleSoap(projects, parts) {
		im.skip("%s", o)
	}
	for _, id := range order {
		p := projects[id]
		setSoapParents(p)
		contents := im.soapContents(p)
		ok, err := im.putSoap(ctx, *p)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		im.report.SoapProjects++
		for _, r := range p.Resources {
			c, has := contents[r.ID]
			if !has {
				continue
			}
			if err := im.store.Soap().PutResourceContent(ctx, p.ID, r.ID, c); err != nil {
				return fmt.Errorf("store soap resource %s content: %w", r.ID, err)
			}
		}
		for _, po := range p.Ports {
			for _, o := range po.Operations {
				if err := im.setSequence(ctx, o.ID, parts.sequence[o.ID]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// soapContents loads the resource texts (SoapResourceFileRepository):
// WSDL in wsdl/<id>.rsc, SCHEMA in schema/<id>.rsc, WSDL_IMPORT in <id>.rsc
// next to the metadata. The last one is the metadata file itself, so
// CastleMock lost one of the two on save; such content is reported missing.
func (im *dataDirImport) soapContents(p *domain.SoapProject) map[string]string {
	contents := map[string]string{}
	for _, r := range p.Resources {
		var sub string
		switch r.Type {
		case domain.SoapResourceWSDL:
			sub = "wsdl"
		case domain.SoapResourceSchema:
			sub = "schema"
		default:
			im.warn("soap resource %s (%s) of project %s: %s content is not recoverable from the data dir", r.ID, r.Name, p.ID, r.Type)
			continue
		}
		data, err := os.ReadFile(filepath.Join(im.dir, layout.soapResource, sub, r.ID+".rsc"))
		if err != nil {
			im.warn("soap resource %s (%s) of project %s: content not found: %v", r.ID, r.Name, p.ID, err)
			continue
		}
		contents[r.ID] = string(data)
	}
	return contents
}

func (im *dataDirImport) putSoap(ctx context.Context, p domain.SoapProject) (bool, error) {
	_, err := im.store.Soap().Get(ctx, p.ID)
	exists := err == nil
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return false, fmt.Errorf("check soap project %s: %w", p.ID, err)
	}
	switch {
	case exists && !im.opts.Overwrite:
		im.skip("soap project %s (%s): already exists (use --overwrite)", p.ID, p.Name)
		return false, nil
	case exists:
		err = im.store.Soap().Put(ctx, p)
	default:
		err = im.store.Soap().Create(ctx, p)
	}
	if errors.Is(err, storage.ErrConflict) {
		im.skip("soap project %s (%s): %v", p.ID, p.Name, err)
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("store soap project %s: %w", p.ID, err)
	}
	return true, nil
}

// importUsers copies users with their bcrypt hashes (Spring's $2a$10$ is
// compatible). Conflicts on id or username are skipped unless Overwrite,
// except that the target's untouched default admin is always replaced by the
// imported admin (§3.6.4.5).
func (im *dataDirImport) importUsers(ctx context.Context) error {
	var users []domain.User
	err := im.readEntities(layout.user, ".user", "user", func(n *node, file string) error {
		u, err := user(n)
		if err != nil {
			return err
		}
		if u.ID == "" || u.Username == "" {
			return errors.New("user without id or username")
		}
		users = append(users, u)
		return nil
	})
	if err != nil {
		return err
	}
	repo := im.store.Users()
	for _, u := range users {
		byID, err := repo.Get(ctx, u.ID)
		idTaken := err == nil
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("check user %s: %w", u.ID, err)
		}
		byName, err := repo.GetByUsername(ctx, u.Username)
		nameTaken := err == nil && byName.ID != u.ID
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("check user %s: %w", u.Username, err)
		}
		replaceDefault := nameTaken && im.isDefaultAdmin(byName)
		if (idTaken || nameTaken) && !im.opts.Overwrite && !replaceDefault {
			im.skip("user %s (%s): already exists (use --overwrite)", u.ID, u.Username)
			continue
		}
		if nameTaken {
			if _, err := repo.Delete(ctx, byName.ID); err != nil {
				return fmt.Errorf("replace user %s: %w", byName.Username, err)
			}
		}
		if idTaken && byID.Username != u.Username {
			if _, err := repo.Delete(ctx, u.ID); err != nil {
				return fmt.Errorf("replace user %s: %w", u.ID, err)
			}
		}
		if err := repo.Save(ctx, u); err != nil {
			return fmt.Errorf("save user %s: %w", u.Username, err)
		}
		im.report.Users++
	}
	return nil
}

// isDefaultAdmin recognizes the admin the target store created on its own
// (UserFileRepository.postInitiate): username admin whose password still
// verifies against the configured default.
func (im *dataDirImport) isDefaultAdmin(u domain.User) bool {
	password := cmp.Or(im.opts.AdminPassword, "admin")
	return strings.EqualFold(u.Username, "admin") && u.Role == domain.RoleAdmin &&
		bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) == nil
}

// importEvents adds events oldest first, so the store's per-kind maximum
// (it evicts the oldest) keeps the newest ones.
func (im *dataDirImport) importEvents(ctx context.Context) error {
	var restEvents []domain.RestEvent
	err := im.readEntities(layout.restEvent, ".event", "restEvent", func(n *node, file string) error {
		e, err := restEvent(n)
		if err != nil {
			return err
		}
		restEvents = append(restEvents, e)
		return nil
	})
	if err != nil {
		return err
	}
	var soapEvents []domain.SoapEvent
	err = im.readEntities(layout.soapEvent, ".event", "soapEvent", func(n *node, file string) error {
		e, err := soapEvent(n)
		if err != nil {
			return err
		}
		soapEvents = append(soapEvents, e)
		return nil
	})
	if err != nil {
		return err
	}
	sort.SliceStable(restEvents, func(i, j int) bool { return restEvents[i].StartDate.Before(restEvents[j].StartDate.Time) })
	sort.SliceStable(soapEvents, func(i, j int) bool { return soapEvents[i].StartDate.Before(soapEvents[j].StartDate.Time) })
	ev := im.store.Events()
	for _, e := range restEvents {
		if _, err := ev.GetRest(ctx, e.ID); err == nil {
			im.skip("rest event %s: already exists", e.ID)
			continue
		}
		if err := ev.AddRest(ctx, e); err != nil {
			return fmt.Errorf("store rest event %s: %w", e.ID, err)
		}
		im.report.RestEvents++
	}
	for _, e := range soapEvents {
		if _, err := ev.GetSoap(ctx, e.ID); err == nil {
			im.skip("soap event %s: already exists", e.ID)
			continue
		}
		if err := ev.AddSoap(ctx, e); err != nil {
			return fmt.Errorf("store soap event %s: %w", e.ID, err)
		}
		im.report.SoapEvents++
	}
	return nil
}
