package definition

import (
	"errors"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/mock/expression"
)

const maxResponseItems = 10

// maxModelDepth stops self-referencing definitions; CastleMock overflows the
// stack on them.
const maxModelDepth = 32

type swaggerDoc struct {
	root        *yaml.Node
	definitions *yaml.Node
}

// convertSwagger ports SwaggerRestDefinitionConverter (swagger-parser 1.x models).
func convertSwagger(data []byte, _ string, generate bool) ([]domain.RestApplication, error) {
	root, err := parseTree(data)
	if err != nil {
		return nil, errors.Join(errors.New("unable to parse the Swagger content"), err)
	}
	if v, ok := str(get(root, "swagger")); !ok || !strings.HasPrefix(v, "2") {
		return nil, errors.New("unable to parse the Swagger content")
	}
	doc := swaggerDoc{root: root, definitions: get(root, "definitions")}
	name, err := doc.applicationName()
	if err != nil {
		return nil, err
	}
	forward := doc.forwardAddress()
	app := domain.RestApplication{Name: name}
	for _, p := range entries(get(root, "paths")) {
		item := deref(root, p.value)
		res := domain.RestResource{Name: p.key, URI: p.key}
		for _, httpMethod := range operationMethods {
			op := get(item, strings.ToLower(string(httpMethod)))
			if op == nil {
				continue
			}
			res.Methods = append(res.Methods, doc.method(op, httpMethod, forward, generate))
		}
		app.Resources = append(app.Resources, res)
	}
	return []domain.RestApplication{app}, nil
}

func (d swaggerDoc) applicationName() (string, error) {
	if title, ok := str(lookup(d.root, "info", "title")); ok {
		return title, nil
	}
	if host, ok := str(get(d.root, "host")); ok {
		return host, nil
	}
	if basePath, ok := str(get(d.root, "basePath")); ok {
		return basePath, nil
	}
	return "", errors.New("unable to extract application name from the Swagger content")
}

// forwardAddress is scheme + "://" + host + basePath. Missing parts print as
// "null", exactly like the Java string concatenation.
func (d swaggerDoc) forwardAddress() string {
	scheme := "http"
	if schemes := items(get(d.root, "schemes")); len(schemes) > 0 {
		scheme = strOr(schemes[0], scheme)
	}
	return scheme + "://" + strOr(get(d.root, "host"), "null") + strOr(get(d.root, "basePath"), "null")
}

func (d swaggerDoc) method(op *yaml.Node, httpMethod domain.HTTPMethod, forward string, generate bool) domain.RestMethod {
	m := domain.RestMethod{
		Name:                 operationName(op, httpMethod),
		HTTPMethod:           httpMethod,
		Status:               domain.RestMethodMocked,
		ResponseStrategy:     domain.RestStrategySequence,
		ForwardedEndpoint:    forward,
		SimulateNetworkDelay: domain.Ptr(false),
		NetworkDelay:         domain.Ptr(int64(0)),
		AutomaticForward:     domain.Ptr(false),
	}
	if !generate {
		return m
	}
	responses := entries(get(op, "responses"))
	if len(responses) == 0 {
		m.MockResponses = []domain.RestMockResponse{generatedResponse()}
		return m
	}
	// Only the operation's own "produces" is read: without it no response is
	// generated at all.
	producesNode := get(op, "produces")
	if producesNode == nil {
		return m
	}
	var produces []string
	for _, p := range items(producesNode) {
		if v, ok := str(p); ok {
			produces = append(produces, v)
		}
	}
	for _, r := range responses {
		resp := deref(d.root, r.value)
		code := statusCode(r.key)
		base := d.response(resp, code)
		generated := false
		for _, contentType := range produces {
			var body string
			switch {
			case strings.EqualFold(contentType, "application/xml"):
				body = d.xmlBody(get(resp, "schema"))
			case strings.EqualFold(contentType, "application/json"):
				body = d.jsonBody(get(resp, "schema"))
			}
			if body == "" {
				continue
			}
			withBody := base
			withBody.Name = base.Name + " (" + contentType + ")"
			withBody.Body = body
			withBody.HTTPHeaders = []domain.HTTPHeader{{Name: "Content-Type", Value: contentType}}
			m.MockResponses = append(m.MockResponses, withBody)
			generated = true
		}
		if !generated {
			m.MockResponses = append(m.MockResponses, base)
		}
	}
	return m
}

func (d swaggerDoc) response(resp *yaml.Node, code int) domain.RestMockResponse {
	headers := []domain.HTTPHeader{}
	for _, h := range entries(get(resp, "headers")) {
		headers = append(headers, domain.HTTPHeader{Name: h.key, Value: ""})
	}
	return domain.RestMockResponse{
		Name:             strOr(get(resp, "description"), ""),
		HTTPStatusCode:   code,
		Status:           responseStatus(code),
		UsingExpressions: domain.Ptr(true),
		HTTPHeaders:      headers,
	}
}

// propertyKind classifies a schema like swagger-models' PropertyBuilder.
type propertyKind int

const (
	unsupportedProperty propertyKind = iota
	refProperty
	arrayProperty
	stringProperty
	integerProperty
	longProperty
	doubleProperty
	floatProperty
	decimalProperty
	booleanProperty
	uuidProperty
	dateProperty
	dateTimeProperty
	passwordProperty
)

func classify(schema *yaml.Node) propertyKind {
	if _, ok := str(get(schema, "$ref")); ok {
		return refProperty
	}
	typ := strOr(get(schema, "type"), "")
	format := strOr(get(schema, "format"), "")
	switch typ {
	case "array":
		return arrayProperty
	case "string":
		switch format {
		case "date":
			return dateProperty
		case "date-time":
			return dateTimeProperty
		case "uuid":
			return uuidProperty
		case "password":
			return passwordProperty
		case "byte", "binary":
			return unsupportedProperty
		}
		return stringProperty
	case "integer":
		switch format {
		case "int32":
			return integerProperty
		case "int64":
			return longProperty
		}
		return unsupportedProperty // BaseIntegerProperty
	case "number":
		switch format {
		case "float":
			return floatProperty
		case "double":
			return doubleProperty
		}
		return decimalProperty
	case "boolean":
		return booleanProperty
	}
	return unsupportedProperty
}

// expressionFor mirrors getExpressionIdentifier. DecimalProperty maps to
// RANDOM_DECIMAL, which TextParser does not register, so that placeholder
// stays in the rendered body exactly as in CastleMock.
func expressionFor(schema *yaml.Node) (string, bool) {
	var name string
	switch classify(schema) {
	case integerProperty:
		name = "RANDOM_INTEGER"
	case longProperty:
		name = "RANDOM_LONG"
	case stringProperty:
		var values []any
		for _, v := range items(get(schema, "enum")) {
			if s, ok := str(v); ok {
				values = append(values, s)
			}
		}
		if len(values) > 0 {
			return expression.Format("RANDOM_ENUM", expression.FormatArg{Name: "values", Value: values}), true
		}
		name = "RANDOM_STRING"
	case doubleProperty:
		name = "RANDOM_DOUBLE"
	case floatProperty:
		name = "RANDOM_FLOAT"
	case booleanProperty:
		name = "RANDOM_BOOLEAN"
	case uuidProperty:
		name = "RANDOM_UUID"
	case decimalProperty:
		name = "RANDOM_DECIMAL"
	case dateProperty:
		name = "RANDOM_DATE"
	case dateTimeProperty:
		name = "RANDOM_DATE_TIME"
	case passwordProperty:
		name = "RANDOM_PASSWORD"
	default:
		return "", false
	}
	return expression.Format(name), true
}

func maxItems(schema *yaml.Node) int {
	if n, ok := intValue(get(schema, "maxItems")); ok {
		return min(maxResponseItems, n)
	}
	return maxResponseItems
}

// model returns the definition a $ref points to (by simple name).
func (d swaggerDoc) model(schema *yaml.Node) *yaml.Node {
	ref, _ := str(get(schema, "$ref"))
	return get(d.definitions, ref[strings.LastIndex(ref, "/")+1:])
}

// typeName is Property.getType() for element names.
func typeName(schema *yaml.Node) string {
	if classify(schema) == refProperty {
		return "ref"
	}
	return strOr(get(schema, "type"), "")
}

// --- JSON bodies (Jackson JsonGenerator) ---

func (d swaggerDoc) jsonBody(schema *yaml.Node) string {
	if schema == nil {
		return ""
	}
	g := &jsonGen{}
	d.jsonProperty(g, nil, schema, 0)
	return g.close()
}

func (d swaggerDoc) jsonProperty(g *jsonGen, name *string, schema *yaml.Node, depth int) {
	if depth > maxModelDepth {
		g.fail()
		return
	}
	if name != nil {
		g.fieldName(*name)
	}
	switch classify(schema) {
	case refProperty:
		model := d.model(schema)
		if model == nil {
			return
		}
		d.jsonModel(g, model, depth+1)
	case arrayProperty:
		item := get(schema, "items")
		g.startArray()
		for range maxItems(schema) {
			d.jsonProperty(g, nil, item, depth+1)
		}
		g.endArray()
	default:
		expr, _ := expressionFor(schema)
		g.stringValue(expr)
	}
}

func (d swaggerDoc) jsonModel(g *jsonGen, model *yaml.Node, depth int) {
	if depth > maxModelDepth {
		g.fail()
		return
	}
	g.startObject()
	switch modelKind(model) {
	case arrayModel:
		item := get(model, "items")
		g.startArray()
		for range maxItems(model) {
			d.jsonProperty(g, nil, item, depth+1)
		}
		g.endArray()
	case refModel:
		if sub := d.model(model); sub != nil {
			d.jsonModel(g, sub, depth+1)
		} else {
			g.fail()
		}
	}
	for _, p := range entries(get(model, "properties")) {
		d.jsonProperty(g, &p.key, p.value, depth+1)
	}
	g.endObject()
}

type modelType int

const (
	implModel modelType = iota
	arrayModel
	refModel
	composedModel
)

func modelKind(model *yaml.Node) modelType {
	switch {
	case get(model, "$ref") != nil:
		return refModel
	case strOr(get(model, "type"), "") == "array":
		return arrayModel
	case get(model, "allOf") != nil:
		return composedModel
	}
	return implModel
}

// jsonGen reproduces the Jackson generator closely enough for the converter:
// a write that is illegal in the current context aborts generation, and
// close() terminates every open array/object (AUTO_CLOSE_JSON_CONTENT).
type jsonGen struct {
	b      strings.Builder
	stack  []*jsonContext
	roots  int
	failed bool
}

type jsonContext struct {
	array      bool
	count      int
	afterField bool
}

func (g *jsonGen) fail() { g.failed = true }

func (g *jsonGen) top() *jsonContext {
	if len(g.stack) == 0 {
		return nil
	}
	return g.stack[len(g.stack)-1]
}

// beforeValue checks that a value may be written here and writes separators.
func (g *jsonGen) beforeValue() bool {
	if g.failed {
		return false
	}
	switch c := g.top(); {
	case c == nil:
		if g.roots > 0 {
			g.b.WriteByte(' ')
		}
		g.roots++
	case c.array:
		if c.count > 0 {
			g.b.WriteByte(',')
		}
		c.count++
	case c.afterField:
		c.afterField = false
	default:
		g.failed = true
		return false
	}
	return true
}

func (g *jsonGen) fieldName(name string) {
	c := g.top()
	if g.failed || c == nil || c.array || c.afterField {
		g.failed = true
		return
	}
	if c.count > 0 {
		g.b.WriteByte(',')
	}
	c.count++
	c.afterField = true
	g.b.WriteString(jacksonQuote(name))
	g.b.WriteByte(':')
}

func (g *jsonGen) stringValue(s string) {
	if g.beforeValue() {
		g.b.WriteString(jacksonQuote(s))
	}
}

func (g *jsonGen) startObject() { g.start('{', false) }
func (g *jsonGen) startArray()  { g.start('[', true) }

func (g *jsonGen) start(open byte, array bool) {
	if !g.beforeValue() {
		return
	}
	g.b.WriteByte(open)
	g.stack = append(g.stack, &jsonContext{array: array})
}

func (g *jsonGen) endObject() { g.end(false) }
func (g *jsonGen) endArray()  { g.end(true) }

func (g *jsonGen) end(array bool) {
	c := g.top()
	if g.failed || c == nil || c.array != array {
		g.failed = true
		return
	}
	g.pop()
}

func (g *jsonGen) pop() {
	if g.top().array {
		g.b.WriteByte(']')
	} else {
		g.b.WriteByte('}')
	}
	g.stack = g.stack[:len(g.stack)-1]
}

func (g *jsonGen) close() string {
	for len(g.stack) > 0 {
		g.pop()
	}
	return g.b.String()
}

// jacksonQuote escapes like Jackson's default character escapes: quotes,
// backslashes and control characters only ("/" and non-ASCII stay as is).
func jacksonQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				b.WriteString(`\u00`)
				b.WriteString(strings.ToUpper(strconv.FormatInt(int64(r)>>4, 16) + strconv.FormatInt(int64(r)&0xF, 16)))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// --- XML bodies (DOM + Transformer) ---

const xmlDeclaration = `<?xml version="1.0" encoding="UTF-8" standalone="no"?>`

type xmlElement struct {
	name     string
	text     string
	children []*xmlElement
}

var errXMLName = errors.New("invalid XML element name")

// xmlBody mirrors generateXmlBody: no schema → no XML body; a failure while
// building the DOM → an empty body.
func (d swaggerDoc) xmlBody(schema *yaml.Node) string {
	if schema == nil {
		return ""
	}
	root, err := d.xmlProperty("", schema, 0)
	if err != nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(xmlDeclaration)
	if root != nil {
		root.write(&b)
	}
	return b.String()
}

// xmlProperty returns nil when a referenced definition is missing
// (Optional.empty in Java).
func (d swaggerDoc) xmlProperty(name string, schema *yaml.Node, depth int) (*xmlElement, error) {
	if depth > maxModelDepth {
		return nil, errors.New("definition nesting too deep")
	}
	if name == "" {
		name = typeName(schema)
	}
	switch classify(schema) {
	case refProperty:
		model := d.model(schema)
		if model == nil {
			return nil, nil
		}
		return d.xmlModel(model, depth+1)
	case arrayProperty:
		el, err := newXMLElement(name)
		if err != nil {
			return nil, err
		}
		item := get(schema, "items")
		for range maxItems(schema) {
			child, err := d.xmlProperty(name, item, depth+1)
			if err != nil {
				return nil, err
			}
			if child != nil {
				el.children = append(el.children, child)
			}
		}
		return el, nil
	default:
		el, err := newXMLElement(name)
		if err != nil {
			return nil, err
		}
		el.text, _ = expressionFor(schema)
		return el, nil
	}
}

func (d swaggerDoc) xmlModel(model *yaml.Node, depth int) (*xmlElement, error) {
	if depth > maxModelDepth {
		return nil, errors.New("definition nesting too deep")
	}
	name := "Result"
	if modelKind(model) == implModel {
		if n, ok := str(lookup(model, "xml", "name")); ok {
			name = n
		}
	}
	el, err := newXMLElement(name)
	if err != nil {
		return nil, err
	}
	switch modelKind(model) {
	case arrayModel:
		item := get(model, "items")
		for range maxItems(model) {
			child, err := d.xmlProperty("array", item, depth+1)
			if err != nil {
				return nil, err
			}
			if child != nil {
				el.children = append(el.children, child)
			}
		}
	case refModel:
		sub := d.model(model)
		if sub == nil {
			return nil, errors.New("missing referenced definition")
		}
		child, err := d.xmlModel(sub, depth+1)
		if err != nil {
			return nil, err
		}
		el.children = append(el.children, child)
	}
	for _, p := range entries(get(model, "properties")) {
		child, err := d.xmlProperty(p.key, p.value, depth+1)
		if err != nil {
			return nil, err
		}
		if child != nil {
			el.children = append(el.children, child)
		}
	}
	return el, nil
}

func newXMLElement(name string) (*xmlElement, error) {
	if !validXMLName(name) {
		return nil, errXMLName
	}
	return &xmlElement{name: name}, nil
}

func validXMLName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		letter := r == '_' || r == ':' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r >= 0xC0
		if i == 0 && !letter {
			return false
		}
		if !letter && !(r >= '0' && r <= '9') && r != '-' && r != '.' && r != 0xB7 {
			return false
		}
	}
	return true
}

func (e *xmlElement) write(b *strings.Builder) {
	b.WriteString("<" + e.name)
	if e.text == "" && len(e.children) == 0 {
		b.WriteString("/>")
		return
	}
	b.WriteByte('>')
	b.WriteString(escapeXMLText(e.text))
	for _, c := range e.children {
		c.write(b)
	}
	b.WriteString("</" + e.name + ">")
}

func escapeXMLText(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\r", "&#13;").Replace(s)
}
