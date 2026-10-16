package castlemock

import (
	"fmt"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// Element → domain converters shared by the data dir (JAXB *File classes)
// and the export files (Jackson on the model classes). Field names are the
// same in both; dates are xsd:dateTime in the data dir and epoch millis in
// exports, and anyTime accepts both. Missing enums get the SPEC.md §2.2
// defaults, unknown elements are ignored.

func anyTime(n *node, name string) (domain.Time, error) {
	t, err := exportTime(n.str(name))
	if err != nil {
		return domain.Time{}, fmt.Errorf("%s: %w", name, err)
	}
	return domain.Time{Time: t}, nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func statusCode(n *node) int {
	if c := n.intVal("httpStatusCode"); c != 0 {
		return c
	}
	return 200
}

func projectFields(n *node) (domain.Project, error) {
	created, err := anyTime(n, "created")
	if err != nil {
		return domain.Project{}, err
	}
	updated, err := anyTime(n, "updated")
	if err != nil {
		return domain.Project{}, err
	}
	return domain.Project{
		ID:          n.str("id"),
		Name:        n.str("name"),
		Description: n.raw("description"),
		Created:     created,
		Updated:     updated,
	}, nil
}

func httpHeaders(n *node, wrapper string) []domain.HTTPHeader {
	out := []domain.HTTPHeader{}
	for _, h := range headers(n, wrapper) {
		out = append(out, domain.HTTPHeader{Name: h.name, Value: h.value})
	}
	return out
}

func httpParameters(n *node, wrapper string) []domain.HTTPParameter {
	out := []domain.HTTPParameter{}
	for _, h := range headers(n, wrapper) {
		out = append(out, domain.HTTPParameter{Name: h.name, Value: h.value})
	}
	return out
}

func encodings(n *node) []domain.ContentEncoding {
	out := []domain.ContentEncoding{}
	for _, e := range stringList(n, "contentEncodings") {
		if e != "" {
			out = append(out, domain.ContentEncoding(e))
		}
	}
	return out
}

func expressions(n *node, wrapper string) []string {
	out := []string{}
	for _, item := range listItems(n, wrapper) {
		out = append(out, item.raw("expression"))
	}
	return out
}

func isTrue(n *node, name string) bool { return n.str(name) == "true" }

func restApplication(n *node) (domain.RestApplication, string) {
	return domain.RestApplication{ID: n.str("id"), Name: n.str("name"), Resources: []domain.RestResource{}}, n.str("projectId")
}

func restResource(n *node) (domain.RestResource, string) {
	return domain.RestResource{ID: n.str("id"), Name: n.str("name"), URI: n.str("uri"), Methods: []domain.RestMethod{}}, n.str("applicationId")
}

// restMethod also returns the persisted SEQUENCE cursor (D-13 moves it to State()).
func restMethod(n *node) (domain.RestMethod, string, int) {
	return domain.RestMethod{
		ID:                    n.str("id"),
		Name:                  n.str("name"),
		DefaultBody:           n.raw("defaultBody"),
		HTTPMethod:            domain.HTTPMethod(orDefault(n.str("httpMethod"), string(domain.MethodGET))),
		ForwardedEndpoint:     n.str("forwardedEndpoint"),
		Status:                domain.RestMethodStatus(orDefault(n.str("status"), string(domain.RestMethodMocked))),
		ResponseStrategy:      domain.RestResponseStrategy(orDefault(n.str("responseStrategy"), string(domain.RestStrategyRandom))),
		SimulateNetworkDelay:  n.boolPtr("simulateNetworkDelay"),
		NetworkDelay:          n.intPtr("networkDelay"),
		DefaultMockResponseID: n.str("defaultMockResponseId"),
		AutomaticForward:      n.boolPtr("automaticForward"),
		MockResponses:         []domain.RestMockResponse{},
	}, n.str("resourceId"), n.intVal("currentResponseSequenceIndex")
}

func restMockResponse(n *node) (domain.RestMockResponse, string) {
	mr := domain.RestMockResponse{
		ID:                  n.str("id"),
		Name:                n.str("name"),
		Body:                n.raw("body"),
		HTTPStatusCode:      statusCode(n),
		Status:              domain.RestMockResponseStatus(orDefault(n.str("status"), string(domain.RestMockResponseEnabled))),
		UsingExpressions:    n.boolPtr("usingExpressions"),
		HTTPHeaders:         httpHeaders(n, "httpHeaders"),
		ContentEncodings:    encodings(n),
		ParameterQueries:    []domain.RestParameterQuery{},
		XPathExpressions:    []domain.RestXPathExpression{},
		JSONPathExpressions: []domain.RestJSONPathExpression{},
		HeaderQueries:       []domain.RestHeaderQuery{},
	}
	if mr.UsingExpressions == nil {
		mr.UsingExpressions = domain.Ptr(false)
	}
	for _, q := range listItems(n, "parameterQueries") {
		mr.ParameterQueries = append(mr.ParameterQueries, domain.RestParameterQuery{
			Parameter: q.str("parameter"), Query: q.raw("query"),
			MatchCase: isTrue(q, "matchCase"), MatchAny: isTrue(q, "matchAny"),
			MatchRegex: isTrue(q, "matchRegex"), URLEncoded: isTrue(q, "urlEncoded"),
		})
	}
	for _, q := range listItems(n, "headerQueries") {
		mr.HeaderQueries = append(mr.HeaderQueries, domain.RestHeaderQuery{
			Header: q.str("header"), Query: q.raw("query"),
			MatchCase: isTrue(q, "matchCase"), MatchAny: isTrue(q, "matchAny"), MatchRegex: isTrue(q, "matchRegex"),
		})
	}
	for _, e := range expressions(n, "xpathExpressions") {
		mr.XPathExpressions = append(mr.XPathExpressions, domain.RestXPathExpression{Expression: e})
	}
	for _, e := range expressions(n, "jsonPathExpressions") {
		mr.JSONPathExpressions = append(mr.JSONPathExpressions, domain.RestJSONPathExpression{Expression: e})
	}
	return mr, n.str("methodId")
}

func soapPort(n *node) (domain.SoapPort, string) {
	return domain.SoapPort{ID: n.str("id"), Name: n.str("name"), URI: n.str("uri"), Operations: []domain.SoapOperation{}}, n.str("projectId")
}

// soapOperation applies rule §3.6.4.2: without operationIdentifier, the
// legacy "identifier" string (or the name) becomes the identifier name.
func soapOperation(n *node) (domain.SoapOperation, string, int) {
	op := domain.SoapOperation{
		ID:                    n.str("id"),
		Name:                  n.str("name"),
		ResponseStrategy:      domain.SoapResponseStrategy(orDefault(n.str("responseStrategy"), string(domain.SoapStrategyRandom))),
		Status:                domain.SoapOperationStatus(orDefault(n.str("status"), string(domain.SoapOperationMocked))),
		HTTPMethod:            domain.HTTPMethod(orDefault(n.str("httpMethod"), string(domain.MethodPOST))),
		SoapVersion:           domain.SoapVersion(orDefault(n.str("soapVersion"), string(domain.SOAP11))),
		DefaultBody:           n.raw("defaultBody"),
		ForwardedEndpoint:     n.str("forwardedEndpoint"),
		OriginalEndpoint:      n.str("originalEndpoint"),
		SimulateNetworkDelay:  n.boolPtr("simulateNetworkDelay"),
		NetworkDelay:          n.intPtr("networkDelay"),
		DefaultMockResponseID: n.str("defaultMockResponseId"),
		MockOnFailure:         n.boolPtr("mockOnFailure"),
		IdentifyStrategy:      domain.SoapOperationIdentifyStrategy(orDefault(n.str("identifyStrategy"), string(domain.IdentifyByElementNamespace))),
		AutomaticForward:      n.boolPtr("automaticForward"),
		MockResponses:         []domain.SoapMockResponse{},
	}
	if id := n.child("operationIdentifier"); id != nil && id.str("name") != "" {
		op.OperationIdentifier = domain.SoapOperationIdentifier{Name: id.str("name"), Namespace: id.str("namespace")}
	} else {
		op.OperationIdentifier = domain.SoapOperationIdentifier{Name: orDefault(n.str("identifier"), op.Name)}
	}
	return op, n.str("portId"), n.intVal("currentResponseSequenceIndex")
}

func soapMockResponse(n *node) (domain.SoapMockResponse, string) {
	mr := domain.SoapMockResponse{
		ID:               n.str("id"),
		Name:             n.str("name"),
		Body:             n.raw("body"),
		Status:           domain.SoapMockResponseStatus(orDefault(n.str("status"), string(domain.SoapMockResponseEnabled))),
		HTTPStatusCode:   statusCode(n),
		UsingExpressions: n.boolPtr("usingExpressions"),
		HTTPHeaders:      httpHeaders(n, "httpHeaders"),
		ContentEncodings: encodings(n),
		XPathExpressions: []domain.SoapXPathExpression{},
	}
	if mr.UsingExpressions == nil {
		mr.UsingExpressions = domain.Ptr(false)
	}
	for _, e := range expressions(n, "xpathExpressions") {
		mr.XPathExpressions = append(mr.XPathExpressions, domain.SoapXPathExpression{Expression: e})
	}
	return mr, n.str("operationId")
}

func soapResource(n *node) (domain.SoapResource, string) {
	return domain.SoapResource{
		ID:   n.str("id"),
		Name: n.str("name"),
		Type: domain.SoapResourceType(orDefault(n.str("type"), string(domain.SoapResourceWSDL))),
	}, n.str("projectId")
}

func user(n *node) (domain.User, error) {
	created, err := anyTime(n, "created")
	if err != nil {
		return domain.User{}, err
	}
	updated, err := anyTime(n, "updated")
	if err != nil {
		return domain.User{}, err
	}
	return domain.User{
		ID:       n.str("id"),
		Username: n.str("username"),
		Password: n.str("password"),
		Email:    n.str("email"),
		FullName: n.str("fullName"),
		Created:  created,
		Updated:  updated,
		Status:   domain.UserStatus(orDefault(n.str("status"), string(domain.UserActive))),
		Role:     domain.Role(orDefault(n.str("role"), string(domain.RoleReader))),
	}, nil
}

func eventFields(n *node) (domain.Event, error) {
	start, err := anyTime(n, "startDate")
	if err != nil {
		return domain.Event{}, err
	}
	end, err := anyTime(n, "endDate")
	if err != nil {
		return domain.Event{}, err
	}
	return domain.Event{ID: n.str("id"), ResourceName: n.str("resourceName"), StartDate: start, EndDate: end}, nil
}

// restEvent reads a data dir restEvent. RestRequestFile annotates its
// httpParameters field with the "httpHeaders" wrapper and vice versa, so the
// two lists are swapped back here (SPEC.md §3.6.3).
func restEvent(n *node) (domain.RestEvent, error) {
	ev, err := eventFields(n)
	if err != nil {
		return domain.RestEvent{}, err
	}
	out := domain.RestEvent{
		Event:         ev,
		ProjectID:     n.str("projectId"),
		ApplicationID: n.str("applicationId"),
		ResourceID:    n.str("resourceId"),
		MethodID:      n.str("methodId"),
		Request:       domain.RestRequest{HTTPHeaders: []domain.HTTPHeader{}, HTTPParameters: []domain.HTTPParameter{}},
	}
	if req := n.child("request"); req != nil {
		out.Request = domain.RestRequest{
			Body:           req.raw("body"),
			ContentType:    req.str("contentType"),
			URI:            req.str("uri"),
			HTTPMethod:     domain.HTTPMethod(req.str("httpMethod")),
			HTTPHeaders:    httpHeaders(req, "httpParameters"),
			HTTPParameters: httpParameters(req, "httpHeaders"),
		}
	}
	if resp := n.child("response"); resp != nil && len(resp.children) > 0 {
		out.Response = &domain.RestResponse{
			Body:             resp.raw("body"),
			MockResponseName: resp.str("mockResponseName"),
			HTTPStatusCode:   resp.intVal("httpStatusCode"),
			ContentType:      resp.str("contentType"),
			HTTPHeaders:      httpHeaders(resp, "httpHeaders"),
			ContentEncodings: encodings(resp),
		}
	}
	return out, nil
}

func soapEvent(n *node) (domain.SoapEvent, error) {
	ev, err := eventFields(n)
	if err != nil {
		return domain.SoapEvent{}, err
	}
	out := domain.SoapEvent{
		Event:       ev,
		ProjectID:   n.str("projectId"),
		PortID:      n.str("portId"),
		OperationID: n.str("operationId"),
		Request:     domain.SoapRequest{HTTPHeaders: []domain.HTTPHeader{}},
	}
	if req := n.child("request"); req != nil {
		out.Request = domain.SoapRequest{
			Body:          req.raw("body"),
			Envelope:      req.raw("envelope"),
			ContentType:   req.str("contentType"),
			URI:           req.str("uri"),
			HTTPMethod:    domain.HTTPMethod(req.str("httpMethod")),
			OperationName: req.str("operationName"),
			SoapVersion:   domain.SoapVersion(orDefault(req.str("soapVersion"), string(domain.SOAP11))),
			HTTPHeaders:   httpHeaders(req, "httpHeaders"),
		}
		if id := req.child("operationIdentifier"); id != nil {
			out.Request.OperationIdentifier = domain.SoapOperationIdentifier{Name: id.str("name"), Namespace: id.str("namespace")}
		}
	}
	if resp := n.child("response"); resp != nil && len(resp.children) > 0 {
		out.Response = &domain.SoapResponse{
			Body:             resp.raw("body"),
			MockResponseName: resp.str("mockResponseName"),
			HTTPStatusCode:   resp.intVal("httpStatusCode"),
			ContentType:      resp.str("contentType"),
			HTTPHeaders:      httpHeaders(resp, "httpHeaders"),
			ContentEncodings: encodings(resp),
		}
	}
	return out, nil
}

// --- tree assembly (flat entities linked by parent ids) ---

type restParts struct {
	apps      []parented[domain.RestApplication]
	resources []parented[domain.RestResource]
	methods   []parented[domain.RestMethod]
	responses []parented[domain.RestMockResponse]
	sequence  map[string]int
}

type parented[T any] struct {
	value  T
	parent string
	source string // file name or "export", for orphan reports
}

// assembleRest links entities to projects; entities whose parent is missing
// are returned as orphan descriptions.
func assembleRest(projects map[string]*domain.RestProject, parts restParts) []string {
	var orphans []string
	apps := map[string]*domain.RestApplication{}
	for _, it := range parts.apps {
		p, ok := projects[it.parent]
		if !ok {
			orphans = append(orphans, fmt.Sprintf("rest application %s (%s): project %s not found", it.value.ID, it.source, it.parent))
			continue
		}
		p.Applications = append(p.Applications, it.value)
	}
	for _, p := range projects {
		for i := range p.Applications {
			apps[p.Applications[i].ID] = &p.Applications[i]
		}
	}
	for _, it := range parts.resources {
		a, ok := apps[it.parent]
		if !ok {
			orphans = append(orphans, fmt.Sprintf("rest resource %s (%s): application %s not found", it.value.ID, it.source, it.parent))
			continue
		}
		a.Resources = append(a.Resources, it.value)
	}
	resources := map[string]*domain.RestResource{}
	for _, a := range apps {
		for i := range a.Resources {
			resources[a.Resources[i].ID] = &a.Resources[i]
		}
	}
	for _, it := range parts.methods {
		r, ok := resources[it.parent]
		if !ok {
			orphans = append(orphans, fmt.Sprintf("rest method %s (%s): resource %s not found", it.value.ID, it.source, it.parent))
			continue
		}
		r.Methods = append(r.Methods, it.value)
	}
	methods := map[string]*domain.RestMethod{}
	for _, r := range resources {
		for i := range r.Methods {
			methods[r.Methods[i].ID] = &r.Methods[i]
		}
	}
	for _, it := range parts.responses {
		m, ok := methods[it.parent]
		if !ok {
			orphans = append(orphans, fmt.Sprintf("rest mock response %s (%s): method %s not found", it.value.ID, it.source, it.parent))
			continue
		}
		m.MockResponses = append(m.MockResponses, it.value)
	}
	return orphans
}

type soapParts struct {
	ports      []parented[domain.SoapPort]
	operations []parented[domain.SoapOperation]
	responses  []parented[domain.SoapMockResponse]
	resources  []parented[domain.SoapResource]
	sequence   map[string]int
}

func assembleSoap(projects map[string]*domain.SoapProject, parts soapParts) []string {
	var orphans []string
	for _, it := range parts.resources {
		p, ok := projects[it.parent]
		if !ok {
			orphans = append(orphans, fmt.Sprintf("soap resource %s (%s): project %s not found", it.value.ID, it.source, it.parent))
			continue
		}
		p.Resources = append(p.Resources, it.value)
	}
	for _, it := range parts.ports {
		p, ok := projects[it.parent]
		if !ok {
			orphans = append(orphans, fmt.Sprintf("soap port %s (%s): project %s not found", it.value.ID, it.source, it.parent))
			continue
		}
		p.Ports = append(p.Ports, it.value)
	}
	ports := map[string]*domain.SoapPort{}
	for _, p := range projects {
		for i := range p.Ports {
			ports[p.Ports[i].ID] = &p.Ports[i]
		}
	}
	for _, it := range parts.operations {
		po, ok := ports[it.parent]
		if !ok {
			orphans = append(orphans, fmt.Sprintf("soap operation %s (%s): port %s not found", it.value.ID, it.source, it.parent))
			continue
		}
		po.Operations = append(po.Operations, it.value)
	}
	ops := map[string]*domain.SoapOperation{}
	for _, po := range ports {
		for i := range po.Operations {
			ops[po.Operations[i].ID] = &po.Operations[i]
		}
	}
	for _, it := range parts.responses {
		o, ok := ops[it.parent]
		if !ok {
			orphans = append(orphans, fmt.Sprintf("soap mock response %s (%s): operation %s not found", it.value.ID, it.source, it.parent))
			continue
		}
		o.MockResponses = append(o.MockResponses, it.value)
	}
	return orphans
}

// setParents fills the parent id fields implied by nesting.
func setRestParents(p *domain.RestProject) {
	for i := range p.Applications {
		a := &p.Applications[i]
		a.ProjectID = p.ID
		for j := range a.Resources {
			r := &a.Resources[j]
			r.ApplicationID = a.ID
			for k := range r.Methods {
				m := &r.Methods[k]
				m.ResourceID = r.ID
				for l := range m.MockResponses {
					m.MockResponses[l].MethodID = m.ID
				}
			}
		}
	}
}

func setSoapParents(p *domain.SoapProject) {
	for i := range p.Resources {
		p.Resources[i].ProjectID = p.ID
	}
	for i := range p.Ports {
		po := &p.Ports[i]
		po.ProjectID = p.ID
		for j := range po.Operations {
			o := &po.Operations[j]
			o.PortID = po.ID
			for k := range o.MockResponses {
				o.MockResponses[k].OperationID = o.ID
			}
		}
	}
}
