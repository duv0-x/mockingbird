package definition

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func convert(t *testing.T, defType domain.RestDefinitionType, fixture string, generate bool) domain.RestApplication {
	t.Helper()
	apps, err := Convert(context.Background(), defType, Source{Filename: fixture[strings.LastIndex(fixture, "/")+1:], Data: readFixture(t, fixture)},
		Options{GenerateResponse: generate})
	if err != nil {
		t.Fatalf("Convert(%s): %v", fixture, err)
	}
	if len(apps) != 1 {
		t.Fatalf("applications = %d, want 1", len(apps))
	}
	return apps[0]
}

func findRes(t *testing.T, app domain.RestApplication, name string) domain.RestResource {
	t.Helper()
	for _, r := range app.Resources {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("resource %q not found in %+v", name, app.Resources)
	return domain.RestResource{}
}

func findMeth(t *testing.T, res domain.RestResource, name string) domain.RestMethod {
	t.Helper()
	for _, m := range res.Methods {
		if m.Name == name {
			return m
		}
	}
	t.Fatalf("method %q not found in resource %s", name, res.Name)
	return domain.RestMethod{}
}

func findResp(t *testing.T, m domain.RestMethod, name string) domain.RestMockResponse {
	t.Helper()
	for _, r := range m.MockResponses {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("mock response %q not found in method %s: %+v", name, m.Name, m.MockResponses)
	return domain.RestMockResponse{}
}

type wantResponse struct {
	name    string
	code    int
	enabled bool
	body    string
	headers []domain.HTTPHeader
}

func checkResponse(t *testing.T, m domain.RestMethod, w wantResponse) {
	t.Helper()
	r := findResp(t, m, w.name)
	status := domain.RestMockResponseDisabled
	if w.enabled {
		status = domain.RestMockResponseEnabled
	}
	if r.HTTPStatusCode != w.code || r.Status != status || r.Body != w.body || !domain.Deref(r.UsingExpressions) || len(r.ContentEncodings) != 0 {
		t.Errorf("%s/%s = code %d status %s body %q expr %v", m.Name, w.name, r.HTTPStatusCode, r.Status, r.Body, r.UsingExpressions)
	}
	if len(r.HTTPHeaders) != len(w.headers) {
		t.Fatalf("%s/%s headers = %v, want %v", m.Name, w.name, r.HTTPHeaders, w.headers)
	}
	for i := range w.headers {
		if r.HTTPHeaders[i] != w.headers[i] {
			t.Errorf("%s/%s headers = %v, want %v", m.Name, w.name, r.HTTPHeaders, w.headers)
		}
	}
	if r.ID == "" || r.MethodID != m.ID {
		t.Errorf("%s/%s ids = %q/%q", m.Name, w.name, r.ID, r.MethodID)
	}
}

func checkMethod(t *testing.T, m domain.RestMethod, httpMethod domain.HTTPMethod, forward string, strategy domain.RestResponseStrategy, responses int) {
	t.Helper()
	if m.HTTPMethod != httpMethod || m.Status != domain.RestMethodMocked || m.ResponseStrategy != strategy || m.ForwardedEndpoint != forward {
		t.Errorf("method %s = %s %s %s %q", m.Name, m.HTTPMethod, m.Status, m.ResponseStrategy, m.ForwardedEndpoint)
	}
	if len(m.MockResponses) != responses {
		t.Errorf("method %s responses = %d, want %d", m.Name, len(m.MockResponses), responses)
	}
	if m.ID == "" || m.ResourceID == "" {
		t.Errorf("method %s without ids", m.Name)
	}
}

const (
	mockXML  = `<?xml version="1.0" encoding="UTF-8" standalone="no"?><Mock><name>${RANDOM_STRING()}</name><id>${RANDOM_LONG()}</id><createdBy>${RANDOM_STRING()}</createdBy><mockStatus>${RANDOM_INTEGER()}</mockStatus></Mock>`
	mockJSON = `{"name":"${RANDOM_STRING()}","id":"${RANDOM_LONG()}","createdBy":"${RANDOM_STRING()}","mockStatus":"${RANDOM_INTEGER()}"}`
)

// SwaggerRestDefinitionConverterTest.
func TestSwagger(t *testing.T) {
	t.Parallel()
	for _, generate := range []bool{false, true} {
		app := convert(t, domain.RestDefinitionSwagger, "swagger/full.json", generate)
		if app.Name != "Castle Mock Swagger" || len(app.Resources) != 2 {
			t.Fatalf("application = %q with %d resources", app.Name, len(app.Resources))
		}
		const forward = "http://castlemock.com/v1"
		mock := findRes(t, app, "/mock")
		if mock.URI != "/mock" {
			t.Errorf("uri = %q", mock.URI)
		}
		n := func(i int) int {
			if generate {
				return i
			}
			return 0
		}
		getAll := findMeth(t, mock, "getAllMockServices")
		checkMethod(t, getAll, domain.MethodGET, forward, domain.RestStrategySequence, n(4))
		if domain.Deref(getAll.SimulateNetworkDelay) || domain.Deref(getAll.NetworkDelay) != 0 || domain.Deref(getAll.AutomaticForward) {
			t.Errorf("delay settings = %v %v %v", getAll.SimulateNetworkDelay, getAll.NetworkDelay, getAll.AutomaticForward)
		}
		checkMethod(t, findMeth(t, mock, "createMock"), domain.MethodPOST, forward, domain.RestStrategySequence, n(1))
		checkMethod(t, findMeth(t, mock, "headMock"), domain.MethodHEAD, forward, domain.RestStrategySequence, n(1))
		checkMethod(t, findMeth(t, mock, "optionsMock"), domain.MethodOPTIONS, forward, domain.RestStrategySequence, n(1))
		byID := findRes(t, app, "/mock/{mockId}")
		getByID := findMeth(t, byID, "Get mock by mock id")
		checkMethod(t, getByID, domain.MethodGET, forward, domain.RestStrategySequence, n(4))
		update := findMeth(t, byID, "updateMock")
		checkMethod(t, update, domain.MethodPUT, forward, domain.RestStrategySequence, n(2))
		del := findMeth(t, byID, "deleteMock")
		checkMethod(t, del, domain.MethodDELETE, forward, domain.RestStrategySequence, n(1))
		if !generate {
			continue
		}
		for _, m := range []domain.RestMethod{getAll, getByID} {
			checkResponse(t, m, wantResponse{name: "successful operation (application/xml)", code: 200, enabled: true, body: mockXML,
				headers: []domain.HTTPHeader{{Name: "Content-Type", Value: "application/xml"}}})
			checkResponse(t, m, wantResponse{name: "successful operation (application/json)", code: 200, enabled: true, body: mockJSON,
				headers: []domain.HTTPHeader{{Name: "Content-Type", Value: "application/json"}}})
			checkResponse(t, m, wantResponse{name: "Invalid mock id supplied", code: 400})
			checkResponse(t, m, wantResponse{name: "Mock not found", code: 404})
		}
		checkResponse(t, findMeth(t, mock, "createMock"), wantResponse{name: "Invalid mock supplied", code: 400})
		checkResponse(t, update, wantResponse{name: "Invalid mock supplied", code: 400})
		checkResponse(t, update, wantResponse{name: "Mock not found", code: 404})
		checkResponse(t, del, wantResponse{name: "Mock not found", code: 404})
		checkResponse(t, findMeth(t, mock, "headMock"), wantResponse{name: autoGeneratedName, code: 200, enabled: true})
	}
}

func TestSwaggerBodies(t *testing.T) {
	t.Parallel()
	spec := `swagger: "2.0"
info: {title: Bodies}
host: example.com
paths:
  /pets:
    get:
      produces: [application/json, application/xml, text/plain]
      responses:
        "200":
          description: list
          schema: {$ref: "#/definitions/Pets"}
        default:
          description: one
          schema: {$ref: "#/definitions/Pet"}
        "201":
          description: tags
          schema: {type: array, maxItems: 2, items: {type: string, enum: [a, b]}}
        "202":
          description: missing
          schema: {$ref: "#/definitions/Nope"}
        "203":
          description: scalar
          schema: {type: number}
definitions:
  Pets:
    type: array
    items: {$ref: "#/definitions/Pet"}
  Pet:
    type: object
    properties:
      id: {type: integer}
      price: {type: number, format: double}
      born: {type: string, format: date-time}
      tags: {type: array, maxItems: 1, items: {type: string}}
      owner: {$ref: "#/definitions/Owner"}
  Owner:
    xml: {name: owner}
    properties:
      name: {type: string, format: password}
`
	apps, err := Convert(context.Background(), domain.RestDefinitionSwagger, Source{Data: []byte(spec)}, Options{GenerateResponse: true})
	if err != nil {
		t.Fatal(err)
	}
	m := apps[0].Resources[0].Methods[0]
	if m.ForwardedEndpoint != "http://example.com"+"null" {
		t.Errorf("forward = %q", m.ForwardedEndpoint)
	}
	pet := `{"id":"","price":"${RANDOM_DOUBLE()}","born":"${RANDOM_DATE_TIME()}","tags":["${RANDOM_STRING()}"],"owner":{"name":"${RANDOM_PASSWORD()}"}}`
	petXML := `<Result><id/><price>${RANDOM_DOUBLE()}</price><born>${RANDOM_DATE_TIME()}</born><tags><tags>${RANDOM_STRING()}</tags></tags><owner><name>${RANDOM_PASSWORD()}</name></owner></Result>`
	tests := []wantResponse{
		// An array model inside the object Jackson opened is illegal: the
		// generator stops and auto-closes, leaving "{}".
		{name: "list (application/json)", code: 200, enabled: true, body: "{}", headers: []domain.HTTPHeader{{Name: "Content-Type", Value: "application/json"}}},
		{name: "list (application/xml)", code: 200, enabled: true, body: xmlDeclaration + "<Result>" + strings.Repeat("<Result>"+petXML[len("<Result>"):], 10) + "</Result>",
			headers: []domain.HTTPHeader{{Name: "Content-Type", Value: "application/xml"}}},
		{name: "one (application/json)", code: 200, enabled: true, body: pet, headers: []domain.HTTPHeader{{Name: "Content-Type", Value: "application/json"}}},
		{name: "tags (application/json)", code: 201, body: `["${RANDOM_ENUM(values=[\"a\",\"b\"])}","${RANDOM_ENUM(values=[\"a\",\"b\"])}"]`,
			headers: []domain.HTTPHeader{{Name: "Content-Type", Value: "application/json"}}},
		{name: "tags (application/xml)", code: 201, body: xmlDeclaration + `<array><array>${RANDOM_ENUM(values=["a","b"])}</array><array>${RANDOM_ENUM(values=["a","b"])}</array></array>`,
			headers: []domain.HTTPHeader{{Name: "Content-Type", Value: "application/xml"}}},
		{name: "missing (application/xml)", code: 202, body: xmlDeclaration, headers: []domain.HTTPHeader{{Name: "Content-Type", Value: "application/xml"}}},
		{name: "scalar (application/json)", code: 203, body: `"${RANDOM_DECIMAL()}"`, headers: []domain.HTTPHeader{{Name: "Content-Type", Value: "application/json"}}},
	}
	for _, w := range tests {
		checkResponse(t, m, w)
	}
	for _, r := range m.MockResponses {
		if r.Name == "missing (application/json)" || strings.Contains(r.Name, "text/plain") {
			t.Errorf("unexpected response %q", r.Name)
		}
	}
}

// OpenApiRestDefinitionConverterTest.
func TestOpenAPI(t *testing.T) {
	t.Parallel()
	for _, generate := range []bool{false, true} {
		app := convert(t, domain.RestDefinitionOpenAPI, "openapi/petstore.json", generate)
		if app.Name != "Swagger Petstore - OpenAPI 3.0" || len(app.Resources) != 2 {
			t.Fatalf("application = %q with %d resources", app.Name, len(app.Resources))
		}
		n := func(i int) int {
			if generate {
				return i
			}
			return 0
		}
		pet := findRes(t, app, "/pet")
		update := findMeth(t, pet, "updatePet")
		checkMethod(t, update, domain.MethodPUT, "/api/v3", domain.RestStrategySequence, n(4))
		add := findMeth(t, pet, "addPet")
		checkMethod(t, add, domain.MethodPOST, "/api/v3", domain.RestStrategySequence, n(2))
		byID := findRes(t, app, "/pet/{petId}")
		get := findMeth(t, byID, "getPetById")
		checkMethod(t, get, domain.MethodGET, "/api/v3", domain.RestStrategySequence, n(3))
		del := findMeth(t, byID, "deletePet")
		checkMethod(t, del, domain.MethodDELETE, "/api/v3", domain.RestStrategySequence, n(1))
		if byID.InvokeAddress != "" || get.DefaultBody != "" {
			t.Errorf("unexpected invoke address/default body")
		}
		if !generate {
			continue
		}
		checkResponse(t, update, wantResponse{name: "Successful operation", code: 200, enabled: true})
		checkResponse(t, update, wantResponse{name: "Invalid ID supplied", code: 400})
		checkResponse(t, update, wantResponse{name: "Pet not found", code: 404})
		checkResponse(t, update, wantResponse{name: "Validation exception", code: 405})
		checkResponse(t, add, wantResponse{name: "Invalid input", code: 405})
		checkResponse(t, get, wantResponse{name: "successful operation", code: 200, enabled: true})
		checkResponse(t, del, wantResponse{name: "Invalid pet value", code: 400})
	}
}

func TestOpenAPIMalformed(t *testing.T) {
	t.Parallel()
	app := convert(t, domain.RestDefinitionOpenAPI, "openapi/petstore_malformed_openapi.json", true)
	if app.Name != "/api/v3" || len(app.Resources) != 2 {
		t.Fatalf("application = %q with %d resources", app.Name, len(app.Resources))
	}
	pet := findRes(t, app, "/pet")
	update := findMeth(t, pet, "Update an existing pet")
	checkResponse(t, update, wantResponse{name: "Successful operation", code: 200, enabled: true})
	add := findMeth(t, pet, "addPet")
	checkResponse(t, add, wantResponse{name: "Successful operation", code: 200, enabled: true,
		headers: []domain.HTTPHeader{{Name: "X-Rate-Limit"}, {Name: "X-Expires-After"}}})
	get := findMeth(t, findRes(t, app, "/pet/{petId}"), "getPetById")
	checkResponse(t, get, wantResponse{name: "Pet not found", code: 200, enabled: true})
	del := findMeth(t, findRes(t, app, "/pet/{petId}"), "deletePet")
	checkMethod(t, del, domain.MethodDELETE, "/api/v3", domain.RestStrategySequence, 1)
	checkResponse(t, del, wantResponse{name: autoGeneratedName, code: 200, enabled: true})
}

func TestOpenAPIVariants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, spec, app, forward string
	}{
		{"yaml without servers", "openapi: 3.0.0\ninfo: {description: Only description}\npaths:\n  /a:\n    patch: {}\n    get: {}\n", "Only description", "/"},
		{"server description", "openapi: 3.0.0\nservers: [{url: 'http://x/api', description: Desc}]\npaths: {/a: {get: {}}}\n", "Desc", "http://x/api"},
		{"swagger 2 converted", `{"swagger": "2.0", "host": "h.com", "basePath": "/v2", "schemes": ["https"], "paths": {"/a": {"get": {}}}}`, "https://h.com/v2", "https://h.com/v2"},
		{"path item ref", "openapi: 3.0.0\ninfo: {title: T}\npaths:\n  /a: {$ref: '#/components/pathItems/A'}\ncomponents:\n  pathItems:\n    A: {get: {operationId: fromRef}}\n", "T", "/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			apps, err := Convert(context.Background(), domain.RestDefinitionOpenAPI, Source{Data: []byte(tt.spec)}, Options{GenerateResponse: true})
			if err != nil {
				t.Fatal(err)
			}
			app := apps[0]
			if app.Name != tt.app || len(app.Resources) != 1 {
				t.Fatalf("application = %q %+v", app.Name, app.Resources)
			}
			if got := app.Resources[0].Methods; len(got) != 1 || got[0].ForwardedEndpoint != tt.forward || got[0].HTTPMethod != domain.MethodGET {
				t.Errorf("methods = %+v", got)
			}
		})
	}
}

func TestInvalidDefinitions(t *testing.T) {
	t.Parallel()
	empty := readFixture(t, "openapi/petstore_empty.json")
	tests := []struct {
		name    string
		defType domain.RestDefinitionType
		data    string
	}{
		{"empty openapi", domain.RestDefinitionOpenAPI, string(empty)},
		{"openapi garbage", domain.RestDefinitionOpenAPI, "{not json"},
		{"swagger without version", domain.RestDefinitionSwagger, `{"info": {"title": "x"}, "paths": {}}`},
		{"swagger without name", domain.RestDefinitionSwagger, `{"swagger": "2.0", "paths": {}}`},
		{"raml without header", domain.RestDefinitionRAML, "title: x\n"},
		{"raml without title", domain.RestDefinitionRAML, "#%RAML 1.0\n/a:\n  get:\n"},
		{"wadl not xml", domain.RestDefinitionWADL, "nope"},
		{"wadl bad method", domain.RestDefinitionWADL, `<application><resources base="http://x/y"><resource path="a"><method name="FETCH"/></resource></resources></application>`},
		{"wadl relative base", domain.RestDefinitionWADL, `<application><resources base="/y"><resource path="a"/></resources></application>`},
		{"unknown type", domain.RestDefinitionType("XML"), "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Convert(context.Background(), tt.defType, Source{Data: []byte(tt.data)}, Options{GenerateResponse: true})
			if !errors.Is(err, ErrInvalidDefinition) {
				t.Errorf("err = %v, want ErrInvalidDefinition", err)
			}
		})
	}
	_, err := Convert(context.Background(), domain.RestDefinitionOpenAPI, Source{Data: empty}, Options{})
	if err == nil || !strings.Contains(err.Error(), "unable to parse the OpenApi content") {
		t.Errorf("empty file error = %v", err)
	}
}

// WADLRestDefinitionConverterTest.
func TestWADL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		file, resource, method string
		httpMethod             domain.HTTPMethod
	}{
		{"get", "getUser", "getUser", domain.MethodGET},
		{"post", "createUser", "createUser", domain.MethodPOST},
		{"put", "updateUser", "updateUser", domain.MethodPUT},
		{"delete", "deleteUser", "deleteUser", domain.MethodDELETE},
		{"head", "headUser", "headUser", domain.MethodHEAD},
		{"options", "optionsUser", "optionsUser", domain.MethodOPTIONS},
		{"patch", "patchUser", "patchUser", domain.MethodPATCH},
		{"trace", "traceUser", "traceUser", domain.MethodTRACE},
	}
	for _, tt := range tests {
		for _, generate := range []bool{false, true} {
			app := convert(t, domain.RestDefinitionWADL, "wadl/"+tt.file+".wadl", generate)
			if app.Name != tt.file || len(app.Resources) != 1 {
				t.Fatalf("%s: application = %q with %d resources", tt.file, app.Name, len(app.Resources))
			}
			res := app.Resources[0]
			if res.Name != tt.resource || res.URI != "/UserService/V1/"+tt.resource {
				t.Errorf("%s: resource = %q %q", tt.file, res.Name, res.URI)
			}
			if len(res.Methods) != 1 {
				t.Fatalf("%s: methods = %d", tt.file, len(res.Methods))
			}
			m := res.Methods[0]
			want := 0
			if generate {
				want = 1
			}
			checkMethod(t, m, tt.httpMethod, "", domain.RestStrategyRandom, want)
			if m.Name != tt.method || domain.Deref(m.SimulateNetworkDelay) || m.NetworkDelay != nil {
				t.Errorf("%s: method = %+v", tt.file, m)
			}
			if generate {
				checkResponse(t, m, wantResponse{name: autoGeneratedName, code: 200, enabled: true})
			}
		}
	}
}

func TestRAML(t *testing.T) {
	t.Parallel()
	app := convert(t, domain.RestDefinitionRAML, "raml/api08.raml", true)
	if app.Name != "Castle Mock RAML 0.8" || len(app.Resources) != 3 {
		t.Fatalf("application = %q %+v", app.Name, app.Resources)
	}
	users := findRes(t, app, "/users")
	get := findMeth(t, users, "GET")
	checkMethod(t, get, domain.MethodGET, "", domain.RestStrategyRandom, 2)
	r := findResp(t, get, "Response 1")
	if r.Body != "{\"users\": []}\n" || r.HTTPStatusCode != 200 || r.Status != domain.RestMockResponseEnabled || r.HTTPHeaders[0] != (domain.HTTPHeader{Name: "X-Rate", Value: "10"}) {
		t.Errorf("response 1 = %+v", r)
	}
	if r := findResp(t, get, "Response 2"); r.HTTPStatusCode != 404 || r.Status != domain.RestMockResponseDisabled || r.Body != `{"error": "not found"}` {
		t.Errorf("response 2 = %+v", r)
	}
	byID := findRes(t, app, "/users/{userId}")
	if len(byID.Methods) != 2 || findResp(t, findMeth(t, byID, "GET"), "Response 1").Body != "<user/>" {
		t.Errorf("nested resource = %+v", byID)
	}
	findMeth(t, findRes(t, app, "/empty/child"), "PUT")

	app = convert(t, domain.RestDefinitionRAML, "raml/api10.raml", true)
	if app.Name != "Castle Mock RAML 1.0" || len(app.Resources) != 2 {
		t.Fatalf("application = %q %+v", app.Name, app.Resources)
	}
	users = findRes(t, app, "Users")
	if users.URI != "/users" {
		t.Errorf("uri = %q", users.URI)
	}
	list := findMeth(t, users, "List users")
	if r := findResp(t, list, "Response 1"); r.Body != `{"users":[{"name":"Karl","active":true}]}` {
		t.Errorf("structured example = %q", r.Body)
	}
	if r := findResp(t, list, "Response 2"); r.HTTPStatusCode != 500 || r.Body != "" {
		t.Errorf("response 2 = %+v", r)
	}
	patch := findMeth(t, findRes(t, app, "/{userId}"), "patch")
	if r := findResp(t, patch, "Response 1"); r.HTTPStatusCode != 200 || r.Body != `{"id": 1}` || r.HTTPHeaders[0].Value != "abc" {
		t.Errorf("patch response = %+v", r)
	}
	if findRes(t, app, "/{userId}").URI != "/users/{userId}" {
		t.Errorf("nested uri")
	}
}

func TestURLSource(t *testing.T) {
	t.Parallel()
	wadl := readFixture(t, "wadl/get.wadl")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/defs/service.wadl" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(wadl)
	}))
	t.Cleanup(srv.Close)
	apps, err := Convert(context.Background(), domain.RestDefinitionWADL, Source{URL: srv.URL + "/defs/service.wadl"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 || apps[0].Name != "service" {
		t.Errorf("applications = %+v", apps)
	}
	if _, err := Convert(context.Background(), domain.RestDefinitionWADL, Source{URL: srv.URL + "/missing"}, Options{}); err == nil || errors.Is(err, ErrInvalidDefinition) {
		t.Errorf("download failure = %v", err)
	}
}

type fakeRepo struct {
	storage.RestRepository
	project domain.RestProject
}

func (f *fakeRepo) Update(_ context.Context, id string, fn func(*domain.RestProject) error) (domain.RestProject, error) {
	if id != f.project.ID {
		return domain.RestProject{}, storage.ErrNotFound
	}
	if err := fn(&f.project); err != nil {
		return domain.RestProject{}, err
	}
	return f.project, nil
}

func TestImportMerge(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{project: domain.RestProject{
		Project: domain.Project{ID: "P1", Name: "p"},
		Applications: []domain.RestApplication{{
			ID: "A1", Name: "get", ProjectID: "P1",
			Resources: []domain.RestResource{{ID: "R1", Name: "getUser", URI: "/custom", ApplicationID: "A1",
				Methods: []domain.RestMethod{{ID: "M1", Name: "existing", HTTPMethod: domain.MethodPOST, ResourceID: "R1"}}}},
		}},
	}}
	ctx := context.Background()
	if err := Import(ctx, repo, "P1", domain.RestDefinitionWADL, Source{Filename: "get.wadl", Data: readFixture(t, "wadl/get.wadl")}, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := Import(ctx, repo, "P1", domain.RestDefinitionWADL, Source{Filename: "post.wadl", Data: readFixture(t, "wadl/post.wadl")}, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := Import(ctx, repo, "P1", domain.RestDefinitionWADL, Source{Filename: "get.wadl", Data: readFixture(t, "wadl/get.wadl")}, Options{}); err != nil {
		t.Fatal(err)
	}
	p := repo.project
	if len(p.Applications) != 2 {
		t.Fatalf("applications = %+v", p.Applications)
	}
	existing := p.Applications[0]
	if len(existing.Resources) != 1 || existing.Resources[0].URI != "/custom" {
		t.Fatalf("existing resource changed: %+v", existing.Resources)
	}
	methods := existing.Resources[0].Methods
	if len(methods) != 2 || methods[0].ID != "M1" || methods[1].Name != "getUser" || methods[1].ResourceID != "R1" {
		t.Errorf("merged methods = %+v", methods)
	}
	added := p.Applications[1]
	if added.Name != "post" || added.ProjectID != "P1" || added.Resources[0].ApplicationID != added.ID {
		t.Errorf("added application = %+v", added)
	}
	if err := Import(ctx, repo, "P2", domain.RestDefinitionWADL, Source{Data: readFixture(t, "wadl/get.wadl")}, Options{}); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("missing project = %v", err)
	}
}
