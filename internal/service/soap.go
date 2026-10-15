package service

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/importer/castlemock"
	"github.com/duv0-x/mockingbird/internal/mock/soap/wsdl"
	"github.com/duv0-x/mockingbird/internal/storage"
	"github.com/duv0-x/mockingbird/internal/storage/jsonfile"
)

// Read methods return computed fields filled (SPEC.md §5.2).

func findSoapOperation(p *domain.SoapProject, id string) *domain.SoapOperation {
	for i := range p.Ports {
		if o := p.Ports[i].Operation(id); o != nil {
			return o
		}
	}
	return nil
}

func findSoapMockResponse(p *domain.SoapProject, id string) (*domain.SoapOperation, int) {
	for i := range p.Ports {
		for j := range p.Ports[i].Operations {
			o := &p.Ports[i].Operations[j]
			if idx := slices.IndexFunc(o.MockResponses, func(mr domain.SoapMockResponse) bool { return mr.ID == id }); idx >= 0 {
				return o, idx
			}
		}
	}
	return nil, -1
}

func soapOperationIDs(ports ...domain.SoapPort) []string {
	var ids []string
	for _, po := range ports {
		for _, o := range po.Operations {
			ids = append(ids, o.ID)
		}
	}
	return ids
}

func soapStatusCount(ops []domain.SoapOperation) map[domain.SoapOperationStatus]int {
	m := domain.NewSoapStatusCount()
	for _, o := range ops {
		m[o.Status]++
	}
	return m
}

func (s *Service) projectOfSoap(ctx context.Context, kind, id string) (domain.SoapProject, error) {
	pid, err := s.soap().ProjectOf(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return domain.SoapProject{}, notFound(kind, id)
		}
		return domain.SoapProject{}, fmt.Errorf("resolve %s %s: %w", kind, id, err)
	}
	p, err := s.soap().Get(ctx, pid)
	if err != nil {
		return domain.SoapProject{}, fmt.Errorf("load soap project %s: %w", pid, err)
	}
	return p, nil
}

func (s *Service) updateSoapEntity(ctx context.Context, kind, id string, fn func(*domain.SoapProject) error) (domain.SoapProject, error) {
	pid, err := s.soap().ProjectOf(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return domain.SoapProject{}, notFound(kind, id)
		}
		return domain.SoapProject{}, fmt.Errorf("resolve %s %s: %w", kind, id, err)
	}
	p, err := s.soap().Update(ctx, pid, fn)
	if err != nil {
		return domain.SoapProject{}, fmt.Errorf("update %s %s: %w", kind, id, err)
	}
	return p, nil
}

// bulkSoap mirrors bulkRest: unknown ids are skipped silently.
func (s *Service) bulkSoap(ctx context.Context, ids []string, fn func(p *domain.SoapProject, id string)) error {
	byProject := map[string][]string{}
	var order []string
	for _, id := range ids {
		pid, err := s.soap().ProjectOf(ctx, id)
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
		_, err := s.soap().Update(ctx, pid, func(p *domain.SoapProject) error {
			for _, id := range byProject[pid] {
				fn(p, id)
			}
			return nil
		})
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("update soap project %s: %w", pid, err)
		}
	}
	return nil
}

// Views (ReadSoap*Service.prepare*).

func soapProjectView(p domain.SoapProject) domain.SoapProject {
	p.EnsureSlices()
	for i := range p.Resources {
		p.Resources[i].Content = ""
	}
	for i := range p.Ports {
		po := &p.Ports[i]
		po.StatusCount = soapStatusCount(po.Operations)
		po.Operations = []domain.SoapOperation{}
	}
	return p
}

func soapDefaultResponseName(o domain.SoapOperation) string {
	if o.DefaultMockResponseID == "" {
		return ""
	}
	if mr := o.MockResponse(o.DefaultMockResponseID); mr != nil {
		return mr.Name
	}
	return ""
}

func (s *Service) soapPortView(ctx context.Context, po domain.SoapPort) domain.SoapPort {
	po.EnsureSlices()
	po.StatusCount = soapStatusCount(po.Operations)
	for i := range po.Operations {
		o := &po.Operations[i]
		o.DefaultResponseName = soapDefaultResponseName(*o)
		o.CurrentResponseSequenceIndex = s.sequenceIndex(ctx, o.ID)
		o.MockResponses = []domain.SoapMockResponse{}
	}
	return po
}

func (s *Service) soapOperationView(ctx context.Context, o domain.SoapOperation) domain.SoapOperation {
	o.EnsureSlices()
	o.DefaultResponseName = soapDefaultResponseName(o)
	o.CurrentResponseSequenceIndex = s.sequenceIndex(ctx, o.ID)
	return o
}

// Projects.

func (s *Service) checkSoapProjectName(ctx context.Context, name, selfID string) error {
	if name == "" {
		return invalid("name", "project name cannot be empty")
	}
	other, err := s.soap().FindByName(ctx, name)
	switch {
	case err == nil && other.ID != selfID:
		return fmt.Errorf("project name %q is already taken: %w", name, storage.ErrConflict)
	case err != nil && !errors.Is(err, storage.ErrNotFound):
		return fmt.Errorf("find soap project by name: %w", err)
	}
	return nil
}

// CreateSoapProject rejects a name already used by another SOAP project
// (AbstractSoapProjectService.save); REST project creation has no such check.
func (s *Service) CreateSoapProject(ctx context.Context, in ProjectInput) (domain.SoapProject, error) {
	if err := s.checkSoapProjectName(ctx, in.Name, ""); err != nil {
		return domain.SoapProject{}, err
	}
	id, err := s.newID(ctx)
	if err != nil {
		return domain.SoapProject{}, err
	}
	now := domain.Now()
	p := domain.SoapProject{
		Project:   domain.Project{ID: id, Name: in.Name, Description: in.Description, Created: now, Updated: now},
		Ports:     []domain.SoapPort{},
		Resources: []domain.SoapResource{},
	}
	if err := s.soap().Create(ctx, p); err != nil {
		return domain.SoapProject{}, fmt.Errorf("create soap project: %w", err)
	}
	return soapProjectView(p), nil
}

func (s *Service) SoapProject(ctx context.Context, id string) (domain.SoapProject, error) {
	p, err := s.soap().Get(ctx, id)
	if err != nil {
		return domain.SoapProject{}, fmt.Errorf("load soap project %s: %w", id, err)
	}
	return soapProjectView(p), nil
}

func (s *Service) UpdateSoapProject(ctx context.Context, id string, in ProjectInput) (domain.SoapProject, error) {
	if err := s.checkSoapProjectName(ctx, in.Name, id); err != nil {
		return domain.SoapProject{}, err
	}
	p, err := s.soap().Update(ctx, id, func(p *domain.SoapProject) error {
		p.Name = in.Name
		p.Description = in.Description
		p.Updated = domain.Now()
		return nil
	})
	if err != nil {
		return domain.SoapProject{}, fmt.Errorf("update soap project %s: %w", id, err)
	}
	return soapProjectView(p), nil
}

func (s *Service) DeleteSoapProject(ctx context.Context, id string) (domain.SoapProject, error) {
	p, err := s.soap().Delete(ctx, id)
	if err != nil {
		return domain.SoapProject{}, fmt.Errorf("delete soap project %s: %w", id, err)
	}
	s.deleteSequenceIndexes(ctx, soapOperationIDs(p.Ports...))
	return soapProjectView(p), nil
}

// ImportWSDL merges the ports of a WSDL file or link into the project
// (CreateSoapPortsService). includeImports only applies to links.
func (s *Service) ImportWSDL(ctx context.Context, projectID string, src Upload, generateResponse, includeImports bool) error {
	if len(src.Data) == 0 && src.URL == "" {
		return invalid("file", "neither a file nor a link was provided")
	}
	if _, err := s.soap().Get(ctx, projectID); err != nil {
		return fmt.Errorf("load soap project %s: %w", projectID, err)
	}
	err := wsdl.Import(ctx, s.soap(), projectID,
		wsdl.Source{Filename: src.Filename, Data: src.Data, URL: src.URL},
		wsdl.Options{GenerateResponse: generateResponse, IncludeImports: includeImports && src.URL != "", HTTPClient: s.deps.HTTPClient})
	return importError("import wsdl", err)
}

func (s *Service) ImportSoapProject(ctx context.Context, src Upload) (domain.SoapProject, error) {
	var (
		p        domain.SoapProject
		contents map[string]string
		err      error
	)
	switch firstNonBlank(src.Data) {
	case '{':
		p, contents, err = jsonfile.DecodeSoapProject(src.Data)
	case '<':
		p, contents, err = castlemock.ParseSoapExport(src.Data)
	default:
		return domain.SoapProject{}, invalid("file", "unsupported project file: expected CastleMock export XML or mockingbird JSON")
	}
	if err != nil {
		return domain.SoapProject{}, invalid("file", "parse soap project: %v", err)
	}
	if p.ID == "" {
		return domain.SoapProject{}, invalid("file", "project has no id")
	}
	for i := range p.Resources {
		if c := p.Resources[i].Content; c != "" {
			if contents == nil {
				contents = map[string]string{}
			}
			if _, ok := contents[p.Resources[i].ID]; !ok {
				contents[p.Resources[i].ID] = c
			}
		}
		p.Resources[i].Content = ""
	}
	if err := s.soap().Create(ctx, p); err != nil {
		return domain.SoapProject{}, fmt.Errorf("import soap project %s: %w", p.ID, err)
	}
	for _, r := range p.Resources {
		c, ok := contents[r.ID]
		if !ok {
			continue
		}
		if err := s.soap().PutResourceContent(ctx, p.ID, r.ID, c); err != nil {
			return domain.SoapProject{}, fmt.Errorf("store resource %s content: %w", r.ID, err)
		}
	}
	s.deleteSequenceIndexes(ctx, soapOperationIDs(p.Ports...))
	return s.SoapProject(ctx, p.ID)
}

func (s *Service) ExportSoapProject(ctx context.Context, id string, format ExportFormat) (Export, error) {
	if format == "" {
		format = ExportCastleMockXML
	}
	if format != ExportCastleMockXML && format != ExportNativeJSON {
		return Export{}, invalid("format", "unsupported export format %q", format)
	}
	p, err := s.soap().Get(ctx, id)
	if err != nil {
		return Export{}, fmt.Errorf("load soap project %s: %w", id, err)
	}
	p.EnsureSlices()
	contents := map[string]string{}
	for _, r := range p.Resources {
		c, err := s.soap().ResourceContent(ctx, id, r.ID)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return Export{}, fmt.Errorf("load resource %s content: %w", r.ID, err)
		}
		contents[r.ID] = c
	}
	for i := range p.Ports {
		po := &p.Ports[i]
		po.StatusCount = soapStatusCount(po.Operations)
		for j := range po.Operations {
			o := &po.Operations[j]
			o.DefaultResponseName = soapDefaultResponseName(*o)
			o.CurrentResponseSequenceIndex = s.sequenceIndex(ctx, o.ID)
		}
	}
	if format == ExportNativeJSON {
		data, err := jsonfile.EncodeSoapProject(p, contents)
		if err != nil {
			return Export{}, fmt.Errorf("encode soap project %s: %w", id, err)
		}
		return Export{Filename: id + ".json", ContentType: "application/json", Data: data}, nil
	}
	data, err := castlemock.ExportSoapProject(p, contents)
	if err != nil {
		return Export{}, fmt.Errorf("export soap project %s: %w", id, err)
	}
	return Export{Filename: id + ".xml", ContentType: "application/xml", Data: data}, nil
}

// Ports.

func forEachOperationOfPort(fn func(*domain.SoapOperation)) func(*domain.SoapProject, string) {
	return func(p *domain.SoapProject, id string) {
		po := p.Port(id)
		if po == nil {
			return
		}
		for i := range po.Operations {
			fn(&po.Operations[i])
		}
	}
}

func (s *Service) SetSoapPortsStatus(ctx context.Context, ids []string, status domain.SoapOperationStatus) error {
	if !validEnum(status, domain.SoapOperationStatuses...) {
		return invalid("status", "unknown operation status %q", status)
	}
	return s.bulkSoap(ctx, ids, forEachOperationOfPort(func(o *domain.SoapOperation) { o.Status = status }))
}

func (s *Service) SetSoapPortsForwardedEndpoint(ctx context.Context, ids []string, endpoint string) error {
	return s.bulkSoap(ctx, ids, forEachOperationOfPort(func(o *domain.SoapOperation) { o.ForwardedEndpoint = endpoint }))
}

func (s *Service) SoapPort(ctx context.Context, id string) (domain.SoapPort, error) {
	p, err := s.projectOfSoap(ctx, "soap port", id)
	if err != nil {
		return domain.SoapPort{}, err
	}
	po := p.Port(id)
	if po == nil {
		return domain.SoapPort{}, notFound("soap port", id)
	}
	return s.soapPortView(ctx, *po), nil
}

func (s *Service) UpdateSoapPort(ctx context.Context, id, uri string) (domain.SoapPort, error) {
	if uri == "" {
		return domain.SoapPort{}, invalid("uri", "port uri cannot be empty")
	}
	var out domain.SoapPort
	_, err := s.updateSoapEntity(ctx, "soap port", id, func(p *domain.SoapProject) error {
		po := p.Port(id)
		if po == nil {
			return notFound("soap port", id)
		}
		po.URI = uri
		out = *po
		return nil
	})
	if err != nil {
		return domain.SoapPort{}, err
	}
	return s.soapPortView(ctx, out), nil
}

func (s *Service) DeleteSoapPort(ctx context.Context, id string) (domain.SoapPort, error) {
	var out domain.SoapPort
	_, err := s.updateSoapEntity(ctx, "soap port", id, func(p *domain.SoapProject) error {
		i := slices.IndexFunc(p.Ports, func(po domain.SoapPort) bool { return po.ID == id })
		if i < 0 {
			return notFound("soap port", id)
		}
		out = p.Ports[i]
		p.Ports = slices.Delete(p.Ports, i, i+1)
		return nil
	})
	if err != nil {
		return domain.SoapPort{}, err
	}
	view := s.soapPortView(ctx, out)
	s.deleteSequenceIndexes(ctx, soapOperationIDs(out))
	return view, nil
}

// Operations.

func forEachOperation(fn func(*domain.SoapOperation)) func(*domain.SoapProject, string) {
	return func(p *domain.SoapProject, id string) {
		if o := findSoapOperation(p, id); o != nil {
			fn(o)
		}
	}
}

func (s *Service) SetSoapOperationsStatus(ctx context.Context, ids []string, status domain.SoapOperationStatus) error {
	if !validEnum(status, domain.SoapOperationStatuses...) {
		return invalid("status", "unknown operation status %q", status)
	}
	return s.bulkSoap(ctx, ids, forEachOperation(func(o *domain.SoapOperation) { o.Status = status }))
}

func (s *Service) SetSoapOperationsForwardedEndpoint(ctx context.Context, ids []string, endpoint string) error {
	return s.bulkSoap(ctx, ids, forEachOperation(func(o *domain.SoapOperation) { o.ForwardedEndpoint = endpoint }))
}

func (s *Service) SoapOperation(ctx context.Context, id string) (domain.SoapOperation, error) {
	p, err := s.projectOfSoap(ctx, "soap operation", id)
	if err != nil {
		return domain.SoapOperation{}, err
	}
	o := findSoapOperation(&p, id)
	if o == nil {
		return domain.SoapOperation{}, notFound("soap operation", id)
	}
	return s.soapOperationView(ctx, *o), nil
}

// UpdateSoapOperation: absent booleans become false and absent
// endpoint/delay/default response become unset (UpdateSoapOperationService).
func (s *Service) UpdateSoapOperation(ctx context.Context, id string, in SoapOperationUpdate) (domain.SoapOperation, error) {
	if !validEnum(in.Status, domain.SoapOperationStatuses...) {
		return domain.SoapOperation{}, invalid("status", "unknown operation status %q", in.Status)
	}
	if !validEnum(in.ResponseStrategy, soapStrategies...) {
		return domain.SoapOperation{}, invalid("responseStrategy", "unknown response strategy %q", in.ResponseStrategy)
	}
	if !validEnum(in.IdentifyStrategy, identifyStrategy...) {
		return domain.SoapOperation{}, invalid("identifyStrategy", "unknown identify strategy %q", in.IdentifyStrategy)
	}
	var out domain.SoapOperation
	_, err := s.updateSoapEntity(ctx, "soap operation", id, func(p *domain.SoapProject) error {
		o := findSoapOperation(p, id)
		if o == nil {
			return notFound("soap operation", id)
		}
		o.Status = in.Status
		o.ResponseStrategy = in.ResponseStrategy
		o.IdentifyStrategy = in.IdentifyStrategy
		o.SimulateNetworkDelay = domain.Ptr(domain.Deref(in.SimulateNetworkDelay))
		o.MockOnFailure = domain.Ptr(domain.Deref(in.MockOnFailure))
		o.AutomaticForward = domain.Ptr(domain.Deref(in.AutomaticForward))
		o.ForwardedEndpoint = domain.Deref(in.ForwardedEndpoint)
		o.NetworkDelay = in.NetworkDelay
		o.DefaultMockResponseID = domain.Deref(in.DefaultMockResponseID)
		out = *o
		return nil
	})
	if err != nil {
		return domain.SoapOperation{}, err
	}
	return s.soapOperationView(ctx, out), nil
}

// Mock responses.

func (s *Service) CreateSoapMockResponse(ctx context.Context, operationID string, in SoapMockResponseInput) (domain.SoapMockResponse, error) {
	if in.Name == "" {
		return domain.SoapMockResponse{}, invalid("name", "mock response name cannot be empty")
	}
	status := in.Status
	if status == "" {
		status = domain.SoapMockResponseEnabled
	}
	if !validEnum(status, soapMRStatuses...) {
		return domain.SoapMockResponse{}, invalid("status", "unknown mock response status %q", status)
	}
	code := in.HTTPStatusCode
	if code == 0 {
		code = 200
	}
	id, err := s.newID(ctx)
	if err != nil {
		return domain.SoapMockResponse{}, err
	}
	mr := domain.SoapMockResponse{
		ID:               id,
		Name:             in.Name,
		Body:             in.Body,
		OperationID:      operationID,
		Status:           status,
		HTTPStatusCode:   code,
		UsingExpressions: domain.Ptr(domain.Deref(in.UsingExpressions)),
		HTTPHeaders:      emptyIfNil(in.HTTPHeaders),
		ContentEncodings: []domain.ContentEncoding{},
		XPathExpressions: emptyIfNil(in.XPathExpressions),
	}
	_, err = s.updateSoapEntity(ctx, "soap operation", operationID, func(p *domain.SoapProject) error {
		o := findSoapOperation(p, operationID)
		if o == nil {
			return notFound("soap operation", operationID)
		}
		o.MockResponses = append(o.MockResponses, mr)
		return nil
	})
	if err != nil {
		return domain.SoapMockResponse{}, err
	}
	return mr, nil
}

func (s *Service) SoapMockResponse(ctx context.Context, id string) (domain.SoapMockResponse, error) {
	p, err := s.projectOfSoap(ctx, "soap mock response", id)
	if err != nil {
		return domain.SoapMockResponse{}, err
	}
	o, i := findSoapMockResponse(&p, id)
	if o == nil {
		return domain.SoapMockResponse{}, notFound("soap mock response", id)
	}
	mr := o.MockResponses[i]
	mr.EnsureSlices()
	return mr, nil
}

// UpdateSoapMockResponse: an empty status or a zero status code keeps the stored value.
func (s *Service) UpdateSoapMockResponse(ctx context.Context, id string, in SoapMockResponseInput) (domain.SoapMockResponse, error) {
	if in.Name == "" {
		return domain.SoapMockResponse{}, invalid("name", "mock response name cannot be empty")
	}
	if in.Status != "" && !validEnum(in.Status, soapMRStatuses...) {
		return domain.SoapMockResponse{}, invalid("status", "unknown mock response status %q", in.Status)
	}
	var out domain.SoapMockResponse
	_, err := s.updateSoapEntity(ctx, "soap mock response", id, func(p *domain.SoapProject) error {
		o, i := findSoapMockResponse(p, id)
		if o == nil {
			return notFound("soap mock response", id)
		}
		mr := &o.MockResponses[i]
		mr.Name = in.Name
		mr.Body = in.Body
		if in.HTTPStatusCode != 0 {
			mr.HTTPStatusCode = in.HTTPStatusCode
		}
		if in.Status != "" {
			mr.Status = in.Status
		}
		mr.HTTPHeaders = emptyIfNil(in.HTTPHeaders)
		mr.UsingExpressions = domain.Ptr(domain.Deref(in.UsingExpressions))
		mr.XPathExpressions = emptyIfNil(in.XPathExpressions)
		out = *mr
		return nil
	})
	if err != nil {
		return domain.SoapMockResponse{}, err
	}
	out.EnsureSlices()
	return out, nil
}

func (s *Service) DeleteSoapMockResponse(ctx context.Context, id string) (domain.SoapMockResponse, error) {
	var out domain.SoapMockResponse
	_, err := s.updateSoapEntity(ctx, "soap mock response", id, func(p *domain.SoapProject) error {
		o, i := findSoapMockResponse(p, id)
		if o == nil {
			return notFound("soap mock response", id)
		}
		out = o.MockResponses[i]
		o.MockResponses = slices.Delete(o.MockResponses, i, i+1)
		return nil
	})
	if err != nil {
		return domain.SoapMockResponse{}, err
	}
	out.EnsureSlices()
	return out, nil
}

func (s *Service) DuplicateSoapMockResponses(ctx context.Context, ids []string) error {
	newIDs, err := s.newIDs(ctx, len(ids))
	if err != nil {
		return err
	}
	next := 0
	return s.bulkSoap(ctx, ids, func(p *domain.SoapProject, id string) {
		o, i := findSoapMockResponse(p, id)
		if o == nil {
			return
		}
		dup := o.MockResponses[i]
		dup.ID = newIDs[next]
		next++
		o.MockResponses = append(o.MockResponses, dup)
	})
}

func (s *Service) SetSoapMockResponsesStatus(ctx context.Context, ids []string, status domain.SoapMockResponseStatus) error {
	if !validEnum(status, soapMRStatuses...) {
		return invalid("status", "unknown mock response status %q", status)
	}
	return s.bulkSoap(ctx, ids, func(p *domain.SoapProject, id string) {
		if o, i := findSoapMockResponse(p, id); o != nil {
			o.MockResponses[i].Status = status
		}
	})
}

// Resources.

func (s *Service) SoapResource(ctx context.Context, id string) (domain.SoapResource, error) {
	p, err := s.projectOfSoap(ctx, "soap resource", id)
	if err != nil {
		return domain.SoapResource{}, err
	}
	r := p.Resource(id)
	if r == nil {
		return domain.SoapResource{}, notFound("soap resource", id)
	}
	out := *r
	out.Content = ""
	return out, nil
}

func (s *Service) SoapResourceContent(ctx context.Context, id string) (string, error) {
	r, err := s.SoapResource(ctx, id)
	if err != nil {
		return "", err
	}
	c, err := s.soap().ResourceContent(ctx, r.ProjectID, id)
	if err != nil {
		return "", fmt.Errorf("load soap resource %s content: %w", id, err)
	}
	return c, nil
}

// ImportSoapResource stores a WSDL/XSD; a WSDL replaces the project's existing WSDL resources.
// CastleMock reused the resource id of the URL; mockingbird always assigns a new one.
func (s *Service) ImportSoapResource(ctx context.Context, projectID string, resourceType domain.SoapResourceType, src Upload) (domain.SoapResource, error) {
	if !validEnum(resourceType, resourceTypes...) {
		return domain.SoapResource{}, invalid("resourceType", "unknown resource type %q", resourceType)
	}
	if len(src.Data) == 0 {
		return domain.SoapResource{}, invalid("file", "no file provided")
	}
	id, err := s.newID(ctx)
	if err != nil {
		return domain.SoapResource{}, err
	}
	r := domain.SoapResource{ID: id, Name: src.Filename, ProjectID: projectID, Type: resourceType}
	_, err = s.soap().Update(ctx, projectID, func(p *domain.SoapProject) error {
		if resourceType == domain.SoapResourceWSDL {
			p.Resources = slices.DeleteFunc(p.Resources, func(r domain.SoapResource) bool { return r.Type == domain.SoapResourceWSDL })
		}
		p.Resources = append(p.Resources, r)
		return nil
	})
	if err != nil {
		return domain.SoapResource{}, fmt.Errorf("import soap resource into %s: %w", projectID, err)
	}
	if err := s.soap().PutResourceContent(ctx, projectID, id, string(src.Data)); err != nil {
		return domain.SoapResource{}, fmt.Errorf("store soap resource %s content: %w", id, err)
	}
	return r, nil
}
