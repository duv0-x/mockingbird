package castlemock

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// Export files (SPEC.md §6.3) are what ExportContainerSerializer produces:
// Jackson XmlMapper + Jdk8Module, INDENT_OUTPUT, no JAXB annotations. So
// properties follow the Java field declaration order (superclass first),
// every list is wrapped twice with the property name, null is an empty
// element, Optional.empty on a NON_EMPTY field is omitted and dates are
// epoch milliseconds. The writer below reproduces that layout so that
// CastleMock can import mockingbird exports.

type xmlWriter struct {
	buf   bytes.Buffer
	depth int
}

func (w *xmlWriter) indent() {
	w.buf.WriteString(strings.Repeat("  ", w.depth))
}

func (w *xmlWriter) open(name string) {
	w.indent()
	w.buf.WriteString("<" + name + ">\n")
	w.depth++
}

func (w *xmlWriter) close(name string) {
	w.depth--
	w.indent()
	w.buf.WriteString("</" + name + ">\n")
}

// null writes the empty element Jackson emits for null values and empty
// collections.
func (w *xmlWriter) null(name string) {
	w.indent()
	w.buf.WriteString("<" + name + "/>\n")
}

func (w *xmlWriter) text(name, value string) {
	w.indent()
	w.buf.WriteString("<" + name + ">")
	w.buf.WriteString(escapeText(value))
	w.buf.WriteString("</" + name + ">\n")
}

// nonEmpty writes an @JsonInclude(NON_EMPTY) Optional<String>.
func (w *xmlWriter) nonEmpty(name, value string) {
	if value != "" {
		w.text(name, value)
	}
}

// nullable writes a plain String that Java leaves null when unset.
func (w *xmlWriter) nullable(name, value string) {
	if value == "" {
		w.null(name)
		return
	}
	w.text(name, value)
}

func (w *xmlWriter) millis(name string, t domain.Time) {
	if t.IsZero() {
		w.null(name)
		return
	}
	w.text(name, strconv.FormatInt(t.UnixMilli(), 10))
}

func (w *xmlWriter) boolPtr(name string, b *bool) {
	if b == nil {
		w.null(name)
		return
	}
	w.text(name, strconv.FormatBool(*b))
}

func (w *xmlWriter) int64Ptr(name string, v *int64) {
	if v == nil {
		w.null(name)
		return
	}
	w.text(name, strconv.FormatInt(*v, 10))
}

func (w *xmlWriter) int(name string, v int) { w.text(name, strconv.Itoa(v)) }

// list writes a double-wrapped Jackson list; item writes one element's
// children.
func list[T any](w *xmlWriter, name string, items []T, item func(T)) {
	if len(items) == 0 {
		w.null(name)
		return
	}
	w.open(name)
	for _, it := range items {
		w.open(name)
		item(it)
		w.close(name)
	}
	w.close(name)
}

func scalarList[T ~string](w *xmlWriter, name string, items []T) {
	if len(items) == 0 {
		w.null(name)
		return
	}
	w.open(name)
	for _, it := range items {
		w.text(name, string(it))
	}
	w.close(name)
}

func statusCount[K ~string](w *xmlWriter, counts map[K]int, order []K) {
	if len(counts) == 0 {
		w.null("statusCount")
		return
	}
	w.open("statusCount")
	for _, k := range order {
		if v, ok := counts[k]; ok {
			w.int(string(k), v)
		}
	}
	w.close("statusCount")
}

// escapeText escapes what XML requires; CR is written as a reference so it
// survives XML end-of-line normalization.
func escapeText(s string) string {
	if !strings.ContainsAny(s, "<>&\r") {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '&':
			b.WriteString("&amp;")
		case '\r':
			b.WriteString("&#13;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (w *xmlWriter) project(p domain.Project) {
	w.text("id", p.ID)
	w.text("name", p.Name)
	w.millis("updated", p.Updated)
	w.millis("created", p.Created)
	w.nonEmpty("description", p.Description)
}

func (w *xmlWriter) headers(hs []domain.HTTPHeader) {
	list(w, "httpHeaders", hs, func(h domain.HTTPHeader) {
		w.text("name", h.Name)
		w.text("value", h.Value)
	})
}

// ExportRestProject writes the CastleMock export XML (root <RestExportContainer>).
// Computed fields of p (statusCount, method uri, sequence index) are
// written as given.
func ExportRestProject(p domain.RestProject) ([]byte, error) {
	if p.ID == "" {
		return nil, errors.New("export rest project: missing id")
	}
	w := &xmlWriter{}
	w.open("RestExportContainer")
	w.open("project")
	w.project(p.Project)
	w.null("applications")
	w.close("project")

	var (
		resources []domain.RestResource
		methods   []domain.RestMethod
		responses []domain.RestMockResponse
	)
	list(w, "applications", p.Applications, func(a domain.RestApplication) {
		w.text("id", a.ID)
		w.text("name", a.Name)
		w.text("projectId", p.ID)
		w.null("resources")
		statusCount(w, a.StatusCount, domain.RestMethodStatuses)
		resources = append(resources, a.Resources...)
	})
	appOf := map[string]string{}
	for _, a := range p.Applications {
		for _, r := range a.Resources {
			appOf[r.ID] = a.ID
		}
	}
	list(w, "resources", resources, func(r domain.RestResource) {
		w.text("id", r.ID)
		w.text("name", r.Name)
		w.text("uri", r.URI)
		w.text("applicationId", appOf[r.ID])
		w.null("methods")
		statusCount(w, r.StatusCount, domain.RestMethodStatuses)
		for _, m := range r.Methods {
			m.ResourceID = r.ID
			if m.URI == "" {
				m.URI = r.URI
			}
			methods = append(methods, m)
		}
	})
	list(w, "methods", methods, func(m domain.RestMethod) {
		w.text("id", m.ID)
		w.text("name", m.Name)
		w.text("resourceId", m.ResourceID)
		w.nonEmpty("defaultBody", m.DefaultBody)
		w.text("httpMethod", string(m.HTTPMethod))
		w.nonEmpty("forwardedEndpoint", m.ForwardedEndpoint)
		w.text("status", string(m.Status))
		w.text("responseStrategy", string(m.ResponseStrategy))
		w.int("currentResponseSequenceIndex", m.CurrentResponseSequenceIndex)
		w.boolPtr("simulateNetworkDelay", m.SimulateNetworkDelay)
		w.int64Ptr("networkDelay", m.NetworkDelay)
		w.nonEmpty("defaultMockResponseId", m.DefaultMockResponseID)
		w.null("mockResponses")
		w.nullable("uri", m.URI)
		w.nonEmpty("defaultResponseName", m.DefaultResponseName)
		w.boolPtr("automaticForward", m.AutomaticForward)
		for _, mr := range m.MockResponses {
			mr.MethodID = m.ID
			responses = append(responses, mr)
		}
	})
	list(w, "mockResponses", responses, func(mr domain.RestMockResponse) {
		w.text("id", mr.ID)
		w.text("name", mr.Name)
		w.nonEmpty("body", mr.Body)
		w.text("methodId", mr.MethodID)
		w.int("httpStatusCode", mr.HTTPStatusCode)
		w.text("status", string(mr.Status))
		w.boolPtr("usingExpressions", mr.UsingExpressions)
		w.headers(mr.HTTPHeaders)
		scalarList(w, "contentEncodings", mr.ContentEncodings)
		list(w, "parameterQueries", mr.ParameterQueries, func(q domain.RestParameterQuery) {
			w.text("parameter", q.Parameter)
			w.text("query", q.Query)
			w.text("matchCase", strconv.FormatBool(q.MatchCase))
			w.text("matchAny", strconv.FormatBool(q.MatchAny))
			w.text("matchRegex", strconv.FormatBool(q.MatchRegex))
			w.text("urlEncoded", strconv.FormatBool(q.URLEncoded))
		})
		list(w, "xpathExpressions", mr.XPathExpressions, func(x domain.RestXPathExpression) { w.text("expression", x.Expression) })
		list(w, "jsonPathExpressions", mr.JSONPathExpressions, func(x domain.RestJSONPathExpression) { w.text("expression", x.Expression) })
		list(w, "headerQueries", mr.HeaderQueries, func(q domain.RestHeaderQuery) {
			w.text("header", q.Header)
			w.text("query", q.Query)
			w.text("matchCase", strconv.FormatBool(q.MatchCase))
			w.text("matchAny", strconv.FormatBool(q.MatchAny))
			w.text("matchRegex", strconv.FormatBool(q.MatchRegex))
		})
	})
	w.close("RestExportContainer")
	return w.buf.Bytes(), nil
}

// ExportSoapProject writes the CastleMock export XML (root <SoapExportContainer>);
// contents maps SoapResource id -> resource text.
func ExportSoapProject(p domain.SoapProject, contents map[string]string) ([]byte, error) {
	if p.ID == "" {
		return nil, errors.New("export soap project: missing id")
	}
	w := &xmlWriter{}
	w.open("SoapExportContainer")
	w.open("project")
	w.project(p.Project)
	w.null("ports")
	w.null("resources")
	w.null("statusCount")
	w.close("project")

	var (
		operations []domain.SoapOperation
		responses  []domain.SoapMockResponse
	)
	list(w, "ports", p.Ports, func(po domain.SoapPort) {
		w.text("id", po.ID)
		w.text("name", po.Name)
		w.text("uri", po.URI)
		w.text("projectId", p.ID)
		w.nonEmpty("invokeAddress", po.InvokeAddress)
		w.null("operations")
		statusCount(w, po.StatusCount, domain.SoapOperationStatuses)
		for _, o := range po.Operations {
			o.PortID = po.ID
			operations = append(operations, o)
		}
	})
	list(w, "resources", p.Resources, func(r domain.SoapResource) {
		w.text("id", r.ID)
		w.text("name", r.Name)
		w.text("projectId", p.ID)
		content := r.Content
		if c, ok := contents[r.ID]; ok {
			content = c
		}
		w.nonEmpty("content", content)
		w.text("type", string(r.Type))
	})
	list(w, "operations", operations, func(o domain.SoapOperation) {
		w.text("id", o.ID)
		w.text("name", o.Name)
		w.open("operationIdentifier")
		w.text("name", o.OperationIdentifier.Name)
		w.nonEmpty("namespace", o.OperationIdentifier.Namespace)
		w.close("operationIdentifier")
		w.text("responseStrategy", string(o.ResponseStrategy))
		w.text("status", string(o.Status))
		w.text("httpMethod", string(o.HTTPMethod))
		w.text("soapVersion", string(o.SoapVersion))
		w.nonEmpty("defaultBody", o.DefaultBody)
		w.int("currentResponseSequenceIndex", o.CurrentResponseSequenceIndex)
		w.nonEmpty("forwardedEndpoint", o.ForwardedEndpoint)
		w.nonEmpty("originalEndpoint", o.OriginalEndpoint)
		w.boolPtr("simulateNetworkDelay", o.SimulateNetworkDelay)
		w.int64Ptr("networkDelay", o.NetworkDelay)
		w.nonEmpty("defaultMockResponseId", o.DefaultMockResponseID)
		w.text("portId", o.PortID)
		w.boolPtr("mockOnFailure", o.MockOnFailure)
		w.text("identifyStrategy", string(o.IdentifyStrategy))
		w.null("mockResponses")
		w.nonEmpty("invokeAddress", o.InvokeAddress)
		w.nonEmpty("defaultResponseName", o.DefaultResponseName)
		w.boolPtr("automaticForward", o.AutomaticForward)
		for _, mr := range o.MockResponses {
			mr.OperationID = o.ID
			responses = append(responses, mr)
		}
	})
	list(w, "mockResponses", responses, func(mr domain.SoapMockResponse) {
		w.text("id", mr.ID)
		w.text("name", mr.Name)
		w.text("body", mr.Body)
		w.text("operationId", mr.OperationID)
		w.text("status", string(mr.Status))
		w.int("httpStatusCode", mr.HTTPStatusCode)
		w.boolPtr("usingExpressions", mr.UsingExpressions)
		w.headers(mr.HTTPHeaders)
		scalarList(w, "contentEncodings", mr.ContentEncodings)
		list(w, "xpathExpressions", mr.XPathExpressions, func(x domain.SoapXPathExpression) { w.text("expression", x.Expression) })
	})
	w.close("SoapExportContainer")
	return w.buf.Bytes(), nil
}

// ParseRestExport reads a CastleMock REST export file. Lists may be
// double-wrapped (Jackson) or singly wrapped, dates millis or ISO, elements
// in any order; unknown elements are ignored. Entities whose parent is not
// in the file are rejected, since an import must not drop data silently.
func ParseRestExport(data []byte) (domain.RestProject, error) {
	root, err := parseXML(data)
	if err != nil {
		return domain.RestProject{}, fmt.Errorf("parse rest export: %w", err)
	}
	if root.name != "RestExportContainer" {
		return domain.RestProject{}, fmt.Errorf("parse rest export: root element <%s>, want <RestExportContainer>", root.name)
	}
	pn := root.child("project")
	if pn == nil {
		return domain.RestProject{}, errors.New("parse rest export: missing <project>")
	}
	fields, err := projectFields(pn)
	if err != nil {
		return domain.RestProject{}, fmt.Errorf("parse rest export: %w", err)
	}
	if fields.ID == "" {
		return domain.RestProject{}, errors.New("parse rest export: project without id")
	}
	p := &domain.RestProject{Project: fields, Applications: []domain.RestApplication{}}
	parts := restParts{sequence: map[string]int{}}
	for _, n := range listItems(root, "applications") {
		a, parent := restApplication(n)
		parts.apps = append(parts.apps, parented[domain.RestApplication]{a, orDefault(parent, p.ID), "export"})
	}
	for _, n := range listItems(root, "resources") {
		r, parent := restResource(n)
		parts.resources = append(parts.resources, parented[domain.RestResource]{r, parent, "export"})
	}
	for _, n := range listItems(root, "methods") {
		m, parent, seq := restMethod(n)
		m.CurrentResponseSequenceIndex = seq
		parts.methods = append(parts.methods, parented[domain.RestMethod]{m, parent, "export"})
	}
	for _, n := range listItems(root, "mockResponses") {
		mr, parent := restMockResponse(n)
		parts.responses = append(parts.responses, parented[domain.RestMockResponse]{mr, parent, "export"})
	}
	if orphans := assembleRest(map[string]*domain.RestProject{p.ID: p}, parts); len(orphans) > 0 {
		return domain.RestProject{}, fmt.Errorf("parse rest export: %s", strings.Join(orphans, "; "))
	}
	setRestParents(p)
	return *p, nil
}

// ParseSoapExport reads a CastleMock SOAP export file; contents maps resource id -> text.
func ParseSoapExport(data []byte) (domain.SoapProject, map[string]string, error) {
	root, err := parseXML(data)
	if err != nil {
		return domain.SoapProject{}, nil, fmt.Errorf("parse soap export: %w", err)
	}
	if root.name != "SoapExportContainer" {
		return domain.SoapProject{}, nil, fmt.Errorf("parse soap export: root element <%s>, want <SoapExportContainer>", root.name)
	}
	pn := root.child("project")
	if pn == nil {
		return domain.SoapProject{}, nil, errors.New("parse soap export: missing <project>")
	}
	fields, err := projectFields(pn)
	if err != nil {
		return domain.SoapProject{}, nil, fmt.Errorf("parse soap export: %w", err)
	}
	if fields.ID == "" {
		return domain.SoapProject{}, nil, errors.New("parse soap export: project without id")
	}
	p := &domain.SoapProject{Project: fields, Ports: []domain.SoapPort{}, Resources: []domain.SoapResource{}}
	parts := soapParts{sequence: map[string]int{}}
	contents := map[string]string{}
	for _, n := range listItems(root, "ports") {
		po, parent := soapPort(n)
		parts.ports = append(parts.ports, parented[domain.SoapPort]{po, orDefault(parent, p.ID), "export"})
	}
	for _, n := range listItems(root, "resources") {
		r, parent := soapResource(n)
		if n.has("content") {
			contents[r.ID] = n.raw("content")
		}
		parts.resources = append(parts.resources, parented[domain.SoapResource]{r, orDefault(parent, p.ID), "export"})
	}
	for _, n := range listItems(root, "operations") {
		o, parent, seq := soapOperation(n)
		o.CurrentResponseSequenceIndex = seq
		parts.operations = append(parts.operations, parented[domain.SoapOperation]{o, parent, "export"})
	}
	for _, n := range listItems(root, "mockResponses") {
		mr, parent := soapMockResponse(n)
		parts.responses = append(parts.responses, parented[domain.SoapMockResponse]{mr, parent, "export"})
	}
	if orphans := assembleSoap(map[string]*domain.SoapProject{p.ID: p}, parts); len(orphans) > 0 {
		return domain.SoapProject{}, nil, fmt.Errorf("parse soap export: %s", strings.Join(orphans, "; "))
	}
	setSoapParents(p)
	return *p, contents, nil
}
