package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/importer/castlemock"
	"github.com/duv0-x/mockingbird/internal/mock/rest/definition"
	"github.com/duv0-x/mockingbird/internal/storage"
	"github.com/duv0-x/mockingbird/internal/storage/jsonfile"
)

// Read methods return computed fields filled (SPEC.md §5.2).

// Tree lookups by leaf id within a loaded project.

func findRestResource(p *domain.RestProject, id string) *domain.RestResource {
	for i := range p.Applications {
		if r := p.Applications[i].Resource(id); r != nil {
			return r
		}
	}
	return nil
}

func findRestMethod(p *domain.RestProject, id string) (*domain.RestResource, *domain.RestMethod) {
	for i := range p.Applications {
		for j := range p.Applications[i].Resources {
			r := &p.Applications[i].Resources[j]
			if m := r.Method(id); m != nil {
				return r, m
			}
		}
	}
	return nil, nil
}

func findRestMockResponse(p *domain.RestProject, id string) (*domain.RestMethod, int) {
	for i := range p.Applications {
		for j := range p.Applications[i].Resources {
			r := &p.Applications[i].Resources[j]
			for k := range r.Methods {
				m := &r.Methods[k]
				if idx := slices.IndexFunc(m.MockResponses, func(mr domain.RestMockResponse) bool { return mr.ID == id }); idx >= 0 {
					return m, idx
				}
			}
		}
	}
	return nil, -1
}

func restMethodIDs(apps ...domain.RestApplication) []string {
	var ids []string
	for _, a := range apps {
		for _, r := range a.Resources {
			for _, m := range r.Methods {
				ids = append(ids, m.ID)
			}
		}
	}
	return ids
}

func resourceMethodIDs(r domain.RestResource) []string {
	ids := make([]string, 0, len(r.Methods))
	for _, m := range r.Methods {
		ids = append(ids, m.ID)
	}
	return ids
}

func restStatusCount(methods []domain.RestMethod, into map[domain.RestMethodStatus]int) {
	for _, m := range methods {
		into[m.Status]++
	}
}

// projectOfRest resolves the project that holds a child entity.
func (s *Service) projectOfRest(ctx context.Context, kind, id string) (domain.RestProject, error) {
	pid, err := s.rest().ProjectOf(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return domain.RestProject{}, notFound(kind, id)
		}
		return domain.RestProject{}, fmt.Errorf("resolve %s %s: %w", kind, id, err)
	}
	p, err := s.rest().Get(ctx, pid)
	if err != nil {
		return domain.RestProject{}, fmt.Errorf("load rest project %s: %w", pid, err)
	}
	return p, nil
}

func (s *Service) updateRestEntity(ctx context.Context, kind, id string, fn func(*domain.RestProject) error) (domain.RestProject, error) {
	pid, err := s.rest().ProjectOf(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return domain.RestProject{}, notFound(kind, id)
		}
		return domain.RestProject{}, fmt.Errorf("resolve %s %s: %w", kind, id, err)
	}
	p, err := s.rest().Update(ctx, pid, fn)
	if err != nil {
		return domain.RestProject{}, fmt.Errorf("update %s %s: %w", kind, id, err)
	}
	return p, nil
}

// bulkRest groups ids by project and applies fn once per project. Unknown ids
// are skipped silently, like CastleMock's bulk services.
func (s *Service) bulkRest(ctx context.Context, ids []string, fn func(p *domain.RestProject, id string)) error {
	byProject := map[string][]string{}
	var order []string
	for _, id := range ids {
		pid, err := s.rest().ProjectOf(ctx, id)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("resolve %s: %w", id, err)
		}
		if _, ok := byProject[pid]; !ok {
			order = append(order, pid)
		}
		byProject[pid] = append(byProject[pid], id)
	}
	for _, pid := range order {
		_, err := s.rest().Update(ctx, pid, func(p *domain.RestProject) error {
			for _, id := range byProject[pid] {
				fn(p, id)
			}
			return nil
		})
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("update rest project %s: %w", pid, err)
		}
	}
	return nil
}

// Views (ReadRest*Service.prepare*).

func restProjectView(p domain.RestProject) domain.RestProject {
	p.EnsureSlices()
	for i := range p.Applications {
		a := &p.Applications[i]
		a.StatusCount = domain.NewRestStatusCount()
		for _, r := range a.Resources {
			restStatusCount(r.Methods, a.StatusCount)
		}
		a.Resources = []domain.RestResource{}
	}
	return p
}

func restApplicationView(a domain.RestApplication) domain.RestApplication {
	a.EnsureSlices()
	a.StatusCount = domain.NewRestStatusCount()
	for i := range a.Resources {
		r := &a.Resources[i]
		restStatusCount(r.Methods, a.StatusCount)
		r.StatusCount = domain.NewRestStatusCount()
		restStatusCount(r.Methods, r.StatusCount)
		r.Methods = []domain.RestMethod{}
	}
	return a
}

func (s *Service) restResourceView(ctx context.Context, r domain.RestResource) domain.RestResource {
	r.EnsureSlices()
	r.StatusCount = domain.NewRestStatusCount()
	restStatusCount(r.Methods, r.StatusCount)
	for i := range r.Methods {
		m := &r.Methods[i]
		m.URI = r.URI
		m.DefaultResponseName = restDefaultResponseName(*m)
		m.CurrentResponseSequenceIndex = s.sequenceIndex(ctx, m.ID)
		m.MockResponses = []domain.RestMockResponse{}
	}
	return r
}

func restDefaultResponseName(m domain.RestMethod) string {
	if m.DefaultMockResponseID == "" {
		return ""
	}
	if mr := m.MockResponse(m.DefaultMockResponseID); mr != nil {
		return mr.Name
	}
	return ""
}

func (s *Service) restMethodView(ctx context.Context, r *domain.RestResource, m domain.RestMethod) domain.RestMethod {
	m.EnsureSlices()
	m.URI = r.URI
	m.DefaultResponseName = restDefaultResponseName(m)
	m.CurrentResponseSequenceIndex = s.sequenceIndex(ctx, m.ID)
	return m
}

// Projects.

func (s *Service) CreateRestProject(ctx context.Context, in ProjectInput) (domain.RestProject, error) {
	if in.Name == "" {
		return domain.RestProject{}, invalid("name", "project name cannot be empty")
	}
	id, err := s.newID(ctx)
	if err != nil {
		return domain.RestProject{}, err
	}
	now := domain.Now()
	p := domain.RestProject{
		Project:      domain.Project{ID: id, Name: in.Name, Description: in.Description, Created: now, Updated: now},
		Applications: []domain.RestApplication{},
	}
	if err := s.rest().Create(ctx, p); err != nil {
		return domain.RestProject{}, fmt.Errorf("create rest project: %w", err)
	}
	return restProjectView(p), nil
}

func (s *Service) RestProject(ctx context.Context, id string) (domain.RestProject, error) {
	p, err := s.rest().Get(ctx, id)
	if err != nil {
		return domain.RestProject{}, fmt.Errorf("load rest project %s: %w", id, err)
	}
	return restProjectView(p), nil
}

func (s *Service) UpdateRestProject(ctx context.Context, id string, in ProjectInput) (domain.RestProject, error) {
	if in.Name == "" {
		return domain.RestProject{}, invalid("name", "project name cannot be empty")
	}
	other, err := s.rest().FindByName(ctx, in.Name)
	switch {
	case err == nil && other.ID != id:
		return domain.RestProject{}, fmt.Errorf("project name %q is already taken: %w", in.Name, storage.ErrConflict)
	case err != nil && !errors.Is(err, storage.ErrNotFound):
		return domain.RestProject{}, fmt.Errorf("find rest project by name: %w", err)
	}
	p, err := s.rest().Update(ctx, id, func(p *domain.RestProject) error {
		p.Name = in.Name
		p.Description = in.Description
		p.Updated = domain.Now()
		return nil
	})
	if err != nil {
		return domain.RestProject{}, fmt.Errorf("update rest project %s: %w", id, err)
	}
	return restProjectView(p), nil
}

func (s *Service) DeleteRestProject(ctx context.Context, id string) (domain.RestProject, error) {
	p, err := s.rest().Delete(ctx, id)
	if err != nil {
		return domain.RestProject{}, fmt.Errorf("delete rest project %s: %w", id, err)
	}
	s.deleteSequenceIndexes(ctx, restMethodIDs(p.Applications...))
	return restProjectView(p), nil
}

func (s *Service) ImportRestDefinition(ctx context.Context, projectID string, defType domain.RestDefinitionType, src Upload, generateResponse bool) error {
	if !validEnum(defType, definitionTypes...) {
		return invalid("definitionType", "unknown definition type %q", defType)
	}
	if len(src.Data) == 0 && src.URL == "" {
		return invalid("file", "neither a file nor a link was provided")
	}
	if _, err := s.rest().Get(ctx, projectID); err != nil {
		return fmt.Errorf("load rest project %s: %w", projectID, err)
	}
	err := definition.Import(ctx, s.rest(), projectID, defType,
		definition.Source{Filename: src.Filename, Data: src.Data, URL: src.URL},
		definition.Options{GenerateResponse: generateResponse, HTTPClient: s.deps.HTTPClient})
	return importError("import rest definition", err)
}

// importError keeps storage sentinels and reports any other converter
// failure (unparsable file, unreachable link) as bad input (D-12).
func importError(op string, err error) error {
	if err == nil {
		return nil
	}
	var ve *ValidationError
	if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrConflict) || errors.As(err, &ve) {
		return fmt.Errorf("%s: %w", op, err)
	}
	return &ValidationError{Field: "file", Message: fmt.Sprintf("%s: %v", op, err)}
}

// ImportRestProject accepts a CastleMock export XML or a native JSON project file.
func (s *Service) ImportRestProject(ctx context.Context, src Upload) (domain.RestProject, error) {
	var (
		p   domain.RestProject
		err error
	)
	switch firstNonBlank(src.Data) {
	case '{':
		p, err = jsonfile.DecodeRestProject(src.Data)
	case '<':
		p, err = castlemock.ParseRestExport(src.Data)
	default:
		return domain.RestProject{}, invalid("file", "unsupported project file: expected CastleMock export XML or mockingbird JSON")
	}
	if err != nil {
		return domain.RestProject{}, invalid("file", "parse rest project: %v", err)
	}
	if p.ID == "" {
		return domain.RestProject{}, invalid("file", "project has no id")
	}
	if err := s.rest().Create(ctx, p); err != nil {
		return domain.RestProject{}, fmt.Errorf("import rest project %s: %w", p.ID, err)
	}
	// ImportRestProjectService resets the SEQUENCE cursor of every method.
	s.deleteSequenceIndexes(ctx, restMethodIDs(p.Applications...))
	return s.RestProject(ctx, p.ID)
}

func (s *Service) ExportRestProject(ctx context.Context, id string, format ExportFormat) (Export, error) {
	if format == "" {
		format = ExportCastleMockXML
	}
	if format != ExportCastleMockXML && format != ExportNativeJSON {
		return Export{}, invalid("format", "unsupported export format %q", format)
	}
	p, err := s.rest().Get(ctx, id)
	if err != nil {
		return Export{}, fmt.Errorf("load rest project %s: %w", id, err)
	}
	p.EnsureSlices()
	for i := range p.Applications {
		a := &p.Applications[i]
		a.StatusCount = domain.NewRestStatusCount()
		for j := range a.Resources {
			r := &a.Resources[j]
			restStatusCount(r.Methods, a.StatusCount)
			r.StatusCount = domain.NewRestStatusCount()
			restStatusCount(r.Methods, r.StatusCount)
			for k := range r.Methods {
				m := &r.Methods[k]
				m.URI = r.URI
				m.DefaultResponseName = restDefaultResponseName(*m)
				m.CurrentResponseSequenceIndex = s.sequenceIndex(ctx, m.ID)
			}
		}
	}
	if format == ExportNativeJSON {
		data, err := jsonfile.EncodeRestProject(p)
		if err != nil {
			return Export{}, fmt.Errorf("encode rest project %s: %w", id, err)
		}
		return Export{Filename: id + ".json", ContentType: "application/json", Data: data}, nil
	}
	data, err := castlemock.ExportRestProject(p)
	if err != nil {
		return Export{}, fmt.Errorf("export rest project %s: %w", id, err)
	}
	return Export{Filename: id + ".xml", ContentType: "application/xml", Data: data}, nil
}

// Applications.

func (s *Service) CreateRestApplication(ctx context.Context, projectID, name string) (domain.RestApplication, error) {
	if name == "" {
		return domain.RestApplication{}, invalid("name", "application name cannot be empty")
	}
	id, err := s.newID(ctx)
	if err != nil {
		return domain.RestApplication{}, err
	}
	a := domain.RestApplication{ID: id, Name: name, ProjectID: projectID, Resources: []domain.RestResource{}}
	_, err = s.rest().Update(ctx, projectID, func(p *domain.RestProject) error {
		p.Applications = append(p.Applications, a)
		return nil
	})
	if err != nil {
		return domain.RestApplication{}, fmt.Errorf("create rest application in %s: %w", projectID, err)
	}
	return restApplicationView(a), nil
}

func (s *Service) RestApplication(ctx context.Context, id string) (domain.RestApplication, error) {
	p, err := s.projectOfRest(ctx, "rest application", id)
	if err != nil {
		return domain.RestApplication{}, err
	}
	a := p.Application(id)
	if a == nil {
		return domain.RestApplication{}, notFound("rest application", id)
	}
	return restApplicationView(*a), nil
}

func (s *Service) UpdateRestApplication(ctx context.Context, id, name string) (domain.RestApplication, error) {
	if name == "" {
		return domain.RestApplication{}, invalid("name", "application name cannot be empty")
	}
	var out domain.RestApplication
	_, err := s.updateRestEntity(ctx, "rest application", id, func(p *domain.RestProject) error {
		a := p.Application(id)
		if a == nil {
			return notFound("rest application", id)
		}
		a.Name = name
		out = *a
		return nil
	})
	if err != nil {
		return domain.RestApplication{}, err
	}
	return restApplicationView(out), nil
}

func (s *Service) DeleteRestApplication(ctx context.Context, id string) (domain.RestApplication, error) {
	var out domain.RestApplication
	_, err := s.updateRestEntity(ctx, "rest application", id, func(p *domain.RestProject) error {
		i := slices.IndexFunc(p.Applications, func(a domain.RestApplication) bool { return a.ID == id })
		if i < 0 {
			return notFound("rest application", id)
		}
		out = p.Applications[i]
		p.Applications = slices.Delete(p.Applications, i, i+1)
		return nil
	})
	if err != nil {
		return domain.RestApplication{}, err
	}
	s.deleteSequenceIndexes(ctx, restMethodIDs(out))
	return restApplicationView(out), nil
}

func forEachMethodOfApplication(fn func(*domain.RestMethod)) func(*domain.RestProject, string) {
	return func(p *domain.RestProject, id string) {
		a := p.Application(id)
		if a == nil {
			return
		}
		for i := range a.Resources {
			for j := range a.Resources[i].Methods {
				fn(&a.Resources[i].Methods[j])
			}
		}
	}
}

// SetRestApplicationsStatus sets the status of every method under the applications.
func (s *Service) SetRestApplicationsStatus(ctx context.Context, ids []string, status domain.RestMethodStatus) error {
	if !validEnum(status, domain.RestMethodStatuses...) {
		return invalid("status", "unknown method status %q", status)
	}
	return s.bulkRest(ctx, ids, forEachMethodOfApplication(func(m *domain.RestMethod) { m.Status = status }))
}

func (s *Service) SetRestApplicationsForwardedEndpoint(ctx context.Context, ids []string, endpoint string) error {
	return s.bulkRest(ctx, ids, forEachMethodOfApplication(func(m *domain.RestMethod) { m.ForwardedEndpoint = endpoint }))
}

// Resources.

func (s *Service) CreateRestResource(ctx context.Context, applicationID, name, uri string) (domain.RestResource, error) {
	if name == "" {
		return domain.RestResource{}, invalid("name", "resource name cannot be empty")
	}
	id, err := s.newID(ctx)
	if err != nil {
		return domain.RestResource{}, err
	}
	r := domain.RestResource{ID: id, Name: name, URI: uri, ApplicationID: applicationID, Methods: []domain.RestMethod{}}
	_, err = s.updateRestEntity(ctx, "rest application", applicationID, func(p *domain.RestProject) error {
		a := p.Application(applicationID)
		if a == nil {
			return notFound("rest application", applicationID)
		}
		a.Resources = append(a.Resources, r)
		return nil
	})
	if err != nil {
		return domain.RestResource{}, err
	}
	return s.restResourceView(ctx, r), nil
}

func (s *Service) RestResource(ctx context.Context, id string) (domain.RestResource, error) {
	p, err := s.projectOfRest(ctx, "rest resource", id)
	if err != nil {
		return domain.RestResource{}, err
	}
	r := findRestResource(&p, id)
	if r == nil {
		return domain.RestResource{}, notFound("rest resource", id)
	}
	return s.restResourceView(ctx, *r), nil
}

func (s *Service) UpdateRestResource(ctx context.Context, id, name, uri string) (domain.RestResource, error) {
	if name == "" {
		return domain.RestResource{}, invalid("name", "resource name cannot be empty")
	}
	var out domain.RestResource
	_, err := s.updateRestEntity(ctx, "rest resource", id, func(p *domain.RestProject) error {
		r := findRestResource(p, id)
		if r == nil {
			return notFound("rest resource", id)
		}
		r.Name, r.URI = name, uri
		out = *r
		return nil
	})
	if err != nil {
		return domain.RestResource{}, err
	}
	return s.restResourceView(ctx, out), nil
}

func (s *Service) DeleteRestResource(ctx context.Context, id string) (domain.RestResource, error) {
	var out domain.RestResource
	_, err := s.updateRestEntity(ctx, "rest resource", id, func(p *domain.RestProject) error {
		for i := range p.Applications {
			a := &p.Applications[i]
			if j := slices.IndexFunc(a.Resources, func(r domain.RestResource) bool { return r.ID == id }); j >= 0 {
				out = a.Resources[j]
				a.Resources = slices.Delete(a.Resources, j, j+1)
				return nil
			}
		}
		return notFound("rest resource", id)
	})
	if err != nil {
		return domain.RestResource{}, err
	}
	s.deleteSequenceIndexes(ctx, resourceMethodIDs(out))
	return s.restResourceView(ctx, out), nil
}

var pathParameterPattern = regexp.MustCompile(`\{(.+?)\}`)

// PathParameters returns the {name} variables of a resource uri in order of
// appearance, without duplicates (UrlUtility.getPathParameters).
func PathParameters(uri string) []string {
	out := []string{}
	for _, m := range pathParameterPattern.FindAllStringSubmatch(uri, -1) {
		if !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

// RestResourceParameters returns the {name} variables of the resource uri.
func (s *Service) RestResourceParameters(ctx context.Context, id string) ([]string, error) {
	r, err := s.RestResource(ctx, id)
	if err != nil {
		return nil, err
	}
	return PathParameters(r.URI), nil
}

func forEachMethodOfResource(fn func(*domain.RestMethod)) func(*domain.RestProject, string) {
	return func(p *domain.RestProject, id string) {
		r := findRestResource(p, id)
		if r == nil {
			return
		}
		for i := range r.Methods {
			fn(&r.Methods[i])
		}
	}
}

func (s *Service) SetRestResourcesStatus(ctx context.Context, ids []string, status domain.RestMethodStatus) error {
	if !validEnum(status, domain.RestMethodStatuses...) {
		return invalid("status", "unknown method status %q", status)
	}
	return s.bulkRest(ctx, ids, forEachMethodOfResource(func(m *domain.RestMethod) { m.Status = status }))
}

func (s *Service) SetRestResourcesForwardedEndpoint(ctx context.Context, ids []string, endpoint string) error {
	return s.bulkRest(ctx, ids, forEachMethodOfResource(func(m *domain.RestMethod) { m.ForwardedEndpoint = endpoint }))
}

// Methods.

func (s *Service) CreateRestMethod(ctx context.Context, resourceID string, in RestMethodCreate) (domain.RestMethod, error) {
	if in.Name == "" {
		return domain.RestMethod{}, invalid("name", "method name cannot be empty")
	}
	httpMethod, err := domain.ParseHTTPMethod(string(in.HTTPMethod))
	if err != nil {
		return domain.RestMethod{}, invalid("httpMethod", "%v", err)
	}
	id, err := s.newID(ctx)
	if err != nil {
		return domain.RestMethod{}, err
	}
	m := domain.RestMethod{
		ID:               id,
		Name:             in.Name,
		ResourceID:       resourceID,
		HTTPMethod:       httpMethod,
		Status:           domain.RestMethodMocked,
		ResponseStrategy: domain.RestStrategyRandom,
		MockResponses:    []domain.RestMockResponse{},
	}
	var res domain.RestResource
	_, err = s.updateRestEntity(ctx, "rest resource", resourceID, func(p *domain.RestProject) error {
		r := findRestResource(p, resourceID)
		if r == nil {
			return notFound("rest resource", resourceID)
		}
		r.Methods = append(r.Methods, m)
		res = *r
		return nil
	})
	if err != nil {
		return domain.RestMethod{}, err
	}
	return s.restMethodView(ctx, &res, m), nil
}

func (s *Service) RestMethod(ctx context.Context, id string) (domain.RestMethod, error) {
	p, err := s.projectOfRest(ctx, "rest method", id)
	if err != nil {
		return domain.RestMethod{}, err
	}
	r, m := findRestMethod(&p, id)
	if m == nil {
		return domain.RestMethod{}, notFound("rest method", id)
	}
	return s.restMethodView(ctx, r, *m), nil
}

// UpdateRestMethod replaces the editable fields; absent optionals become
// unset (UpdateRestMethodService).
func (s *Service) UpdateRestMethod(ctx context.Context, id string, in RestMethodUpdate) (domain.RestMethod, error) {
	if in.Name == "" {
		return domain.RestMethod{}, invalid("name", "method name cannot be empty")
	}
	httpMethod, err := domain.ParseHTTPMethod(string(in.HTTPMethod))
	if err != nil {
		return domain.RestMethod{}, invalid("httpMethod", "%v", err)
	}
	if !validEnum(in.Status, domain.RestMethodStatuses...) {
		return domain.RestMethod{}, invalid("status", "unknown method status %q", in.Status)
	}
	if !validEnum(in.ResponseStrategy, restStrategies...) {
		return domain.RestMethod{}, invalid("responseStrategy", "unknown response strategy %q", in.ResponseStrategy)
	}
	var (
		out domain.RestMethod
		res domain.RestResource
	)
	_, err = s.updateRestEntity(ctx, "rest method", id, func(p *domain.RestProject) error {
		r, m := findRestMethod(p, id)
		if m == nil {
			return notFound("rest method", id)
		}
		m.Name = in.Name
		m.HTTPMethod = httpMethod
		m.Status = in.Status
		m.ResponseStrategy = in.ResponseStrategy
		m.ForwardedEndpoint = domain.Deref(in.ForwardedEndpoint)
		m.SimulateNetworkDelay = in.SimulateNetworkDelay
		m.NetworkDelay = in.NetworkDelay
		m.DefaultMockResponseID = domain.Deref(in.DefaultMockResponseID)
		m.AutomaticForward = in.AutomaticForward
		out, res = *m, *r
		return nil
	})
	if err != nil {
		return domain.RestMethod{}, err
	}
	return s.restMethodView(ctx, &res, out), nil
}

func (s *Service) DeleteRestMethod(ctx context.Context, id string) (domain.RestMethod, error) {
	var (
		out domain.RestMethod
		res domain.RestResource
	)
	_, err := s.updateRestEntity(ctx, "rest method", id, func(p *domain.RestProject) error {
		r, m := findRestMethod(p, id)
		if m == nil {
			return notFound("rest method", id)
		}
		out, res = *m, *r
		r.Methods = slices.DeleteFunc(r.Methods, func(m domain.RestMethod) bool { return m.ID == id })
		return nil
	})
	if err != nil {
		return domain.RestMethod{}, err
	}
	view := s.restMethodView(ctx, &res, out)
	s.deleteSequenceIndexes(ctx, []string{id})
	return view, nil
}

func forEachMethod(fn func(*domain.RestMethod)) func(*domain.RestProject, string) {
	return func(p *domain.RestProject, id string) {
		if _, m := findRestMethod(p, id); m != nil {
			fn(m)
		}
	}
}

func (s *Service) SetRestMethodsStatus(ctx context.Context, ids []string, status domain.RestMethodStatus) error {
	if !validEnum(status, domain.RestMethodStatuses...) {
		return invalid("status", "unknown method status %q", status)
	}
	return s.bulkRest(ctx, ids, forEachMethod(func(m *domain.RestMethod) { m.Status = status }))
}

func (s *Service) SetRestMethodsForwardedEndpoint(ctx context.Context, ids []string, endpoint string) error {
	return s.bulkRest(ctx, ids, forEachMethod(func(m *domain.RestMethod) { m.ForwardedEndpoint = endpoint }))
}

// Mock responses.

func (s *Service) CreateRestMockResponse(ctx context.Context, methodID string, in RestMockResponseInput) (domain.RestMockResponse, error) {
	if in.Name == "" {
		return domain.RestMockResponse{}, invalid("name", "mock response name cannot be empty")
	}
	status := domain.RestMockResponseEnabled
	if in.Status != nil {
		status = *in.Status
	}
	if !validEnum(status, restMRStatuses...) {
		return domain.RestMockResponse{}, invalid("status", "unknown mock response status %q", status)
	}
	id, err := s.newID(ctx)
	if err != nil {
		return domain.RestMockResponse{}, err
	}
	code := 200
	if in.HTTPStatusCode != nil {
		code = *in.HTTPStatusCode
	}
	mr := domain.RestMockResponse{
		ID:                  id,
		Name:                in.Name,
		Body:                domain.Deref(in.Body),
		MethodID:            methodID,
		HTTPStatusCode:      code,
		Status:              status,
		UsingExpressions:    domain.Ptr(domain.Deref(in.UsingExpressions)),
		HTTPHeaders:         emptyIfNil(in.HTTPHeaders),
		ContentEncodings:    emptyIfNil(in.ContentEncodings),
		ParameterQueries:    emptyIfNil(in.ParameterQueries),
		XPathExpressions:    emptyIfNil(in.XPathExpressions),
		JSONPathExpressions: emptyIfNil(in.JSONPathExpressions),
		HeaderQueries:       emptyIfNil(in.HeaderQueries),
	}
	_, err = s.updateRestEntity(ctx, "rest method", methodID, func(p *domain.RestProject) error {
		_, m := findRestMethod(p, methodID)
		if m == nil {
			return notFound("rest method", methodID)
		}
		m.MockResponses = append(m.MockResponses, mr)
		return nil
	})
	if err != nil {
		return domain.RestMockResponse{}, err
	}
	return mr, nil
}

func (s *Service) RestMockResponse(ctx context.Context, id string) (domain.RestMockResponse, error) {
	p, err := s.projectOfRest(ctx, "rest mock response", id)
	if err != nil {
		return domain.RestMockResponse{}, err
	}
	m, i := findRestMockResponse(&p, id)
	if m == nil {
		return domain.RestMockResponse{}, notFound("rest mock response", id)
	}
	mr := m.MockResponses[i]
	mr.EnsureSlices()
	return mr, nil
}

// UpdateRestMockResponse replaces every list; contentEncodings is kept
// because UpdateRestMockResponseService never copies it (CastleMock quirk).
// A missing status or status code keeps the stored value.
func (s *Service) UpdateRestMockResponse(ctx context.Context, id string, in RestMockResponseInput) (domain.RestMockResponse, error) {
	if in.Name == "" {
		return domain.RestMockResponse{}, invalid("name", "mock response name cannot be empty")
	}
	if in.Status != nil && !validEnum(*in.Status, restMRStatuses...) {
		return domain.RestMockResponse{}, invalid("status", "unknown mock response status %q", *in.Status)
	}
	var out domain.RestMockResponse
	_, err := s.updateRestEntity(ctx, "rest mock response", id, func(p *domain.RestProject) error {
		m, i := findRestMockResponse(p, id)
		if m == nil {
			return notFound("rest mock response", id)
		}
		mr := &m.MockResponses[i]
		mr.Name = in.Name
		mr.Body = domain.Deref(in.Body)
		if in.HTTPStatusCode != nil {
			mr.HTTPStatusCode = *in.HTTPStatusCode
		}
		if in.Status != nil {
			mr.Status = *in.Status
		}
		mr.UsingExpressions = domain.Ptr(domain.Deref(in.UsingExpressions))
		mr.HTTPHeaders = emptyIfNil(in.HTTPHeaders)
		mr.ParameterQueries = emptyIfNil(in.ParameterQueries)
		mr.XPathExpressions = emptyIfNil(in.XPathExpressions)
		mr.JSONPathExpressions = emptyIfNil(in.JSONPathExpressions)
		mr.HeaderQueries = emptyIfNil(in.HeaderQueries)
		out = *mr
		return nil
	})
	if err != nil {
		return domain.RestMockResponse{}, err
	}
	out.EnsureSlices()
	return out, nil
}

func (s *Service) DeleteRestMockResponse(ctx context.Context, id string) (domain.RestMockResponse, error) {
	var out domain.RestMockResponse
	_, err := s.updateRestEntity(ctx, "rest mock response", id, func(p *domain.RestProject) error {
		m, i := findRestMockResponse(p, id)
		if m == nil {
			return notFound("rest mock response", id)
		}
		out = m.MockResponses[i]
		m.MockResponses = slices.Delete(m.MockResponses, i, i+1)
		return nil
	})
	if err != nil {
		return domain.RestMockResponse{}, err
	}
	out.EnsureSlices()
	return out, nil
}

// DuplicateRestMockResponses copies each response (same name, same method)
// with a new id. CastleMock set id=null and failed (D-3).
func (s *Service) DuplicateRestMockResponses(ctx context.Context, ids []string) error {
	newIDs, err := s.newIDs(ctx, len(ids))
	if err != nil {
		return err
	}
	next := 0
	return s.bulkRest(ctx, ids, func(p *domain.RestProject, id string) {
		m, i := findRestMockResponse(p, id)
		if m == nil {
			return
		}
		dup := m.MockResponses[i]
		dup.ID = newIDs[next]
		next++
		m.MockResponses = append(m.MockResponses, dup)
	})
}

func (s *Service) SetRestMockResponsesStatus(ctx context.Context, ids []string, status domain.RestMockResponseStatus) error {
	if !validEnum(status, restMRStatuses...) {
		return invalid("status", "unknown mock response status %q", status)
	}
	return s.bulkRest(ctx, ids, func(p *domain.RestProject, id string) {
		if m, i := findRestMockResponse(p, id); m != nil {
			m.MockResponses[i].Status = status
		}
	})
}
