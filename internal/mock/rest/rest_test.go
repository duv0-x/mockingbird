package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/mock/forward"
)

func stringReader(s string) io.Reader { return strings.NewReader(s) }

const (
	projectID     = "PRJ001"
	applicationID = "APP001"
	mockBase      = "/castlemock/mock/rest/project/" + projectID + "/application/" + applicationID
	xmlBody       = "<response><name>Response</name></response>"
)

var xmlHeaders = []domain.HTTPHeader{{Name: "Content-Type", Value: "application/xml"}, {Name: "Accept", Value: "application/xml"}}

func response(id, name, body string, mutate ...func(*domain.RestMockResponse)) domain.RestMockResponse {
	r := domain.RestMockResponse{ID: id, Name: name, Body: body, HTTPStatusCode: 200, Status: domain.RestMockResponseEnabled,
		HTTPHeaders: xmlHeaders}
	for _, m := range mutate {
		m(&r)
	}
	r.EnsureSlices()
	return r
}

func method(id string, httpMethod domain.HTTPMethod, mutate func(*domain.RestMethod), responses ...domain.RestMockResponse) domain.RestMethod {
	m := domain.RestMethod{ID: id, Name: "method " + id, HTTPMethod: httpMethod, Status: domain.RestMethodMocked,
		ResponseStrategy: domain.RestStrategyRandom, MockResponses: responses}
	if mutate != nil {
		mutate(&m)
	}
	return m
}

func project(resources ...domain.RestResource) domain.RestProject {
	p := domain.RestProject{
		Project:      domain.Project{ID: projectID, Name: "Project"},
		Applications: []domain.RestApplication{{ID: applicationID, Name: "app", Resources: resources}},
	}
	p.EnsureSlices()
	return p
}

func resource(id, uri string, methods ...domain.RestMethod) domain.RestResource {
	return domain.RestResource{ID: id, Name: uri, URI: uri, Methods: methods}
}

type call struct {
	method  string
	path    string
	body    string
	headers map[string]string
}

func do(t *testing.T, e *Engine, c call) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	e.Wait()
	return rec
}

func newEngine(store *fakeStore) *Engine {
	return NewEngine(Deps{Store: store, Forward: forward.NewClient(forward.Options{Timeout: 5 * time.Second, TLSVerify: true}), ContextPath: "/castlemock"})
}

func TestMockedStrategies(t *testing.T) {
	t.Parallel()
	queryResponses := []domain.RestMockResponse{
		response("R1", "Query 1", "first", func(r *domain.RestMockResponse) {
			r.ParameterQueries = []domain.RestParameterQuery{{Parameter: "variable", Query: "one"}}
		}),
		response("R2", "Query 2", "second", func(r *domain.RestMockResponse) {
			r.ParameterQueries = []domain.RestParameterQuery{{Parameter: "variable", Query: "two"}}
		}),
		response("RD", "Query default", "default"),
	}
	tests := []struct {
		name       string
		uri        string
		method     domain.RestMethod
		call       call
		wantStatus int
		wantBody   string
		wantHeader map[string]string
	}{
		{
			name:   "random",
			uri:    "/method/test",
			method: method("M1", domain.MethodGET, nil, response("R1", "Mocked response", xmlBody)),
			call:   call{method: "GET", path: mockBase + "/method/test"}, wantStatus: 200, wantBody: xmlBody,
			wantHeader: map[string]string{"Content-Type": "application/xml", "Accept": "application/xml"},
		},
		{
			name: "query match",
			uri:  "/method/{variable}",
			method: method("M1", domain.MethodGET, func(m *domain.RestMethod) {
				m.ResponseStrategy = domain.RestStrategyQueryMatch
				m.DefaultMockResponseID = "RD"
			}, queryResponses...),
			call: call{method: "GET", path: mockBase + "/method/two"}, wantStatus: 200, wantBody: "second",
		},
		{
			name: "query match default response",
			uri:  "/method/{variable}",
			method: method("M1", domain.MethodGET, func(m *domain.RestMethod) {
				m.ResponseStrategy = domain.RestStrategyQueryMatch
				m.DefaultMockResponseID = "RD"
			}, queryResponses...),
			call: call{method: "GET", path: mockBase + "/method/other"}, wantStatus: 200, wantBody: "default",
		},
		{
			name: "query match on query string template",
			uri:  "/search?q={query}",
			method: method("M1", domain.MethodGET, func(m *domain.RestMethod) { m.ResponseStrategy = domain.RestStrategyQueryMatch },
				response("R1", "A", "cats", func(r *domain.RestMockResponse) {
					r.ParameterQueries = []domain.RestParameterQuery{{Parameter: "q", Query: "cats"}}
				}),
				response("R2", "B", "dogs", func(r *domain.RestMockResponse) {
					r.ParameterQueries = []domain.RestParameterQuery{{Parameter: "q", Query: "dogs"}}
				})),
			call: call{method: "GET", path: mockBase + "/search?q=dogs"}, wantStatus: 200, wantBody: "dogs",
		},
		{
			name: "query match without match nor default",
			uri:  "/method/{variable}",
			method: method("M1", domain.MethodGET, func(m *domain.RestMethod) {
				m.ResponseStrategy = domain.RestStrategyQueryMatch
				m.ForwardedEndpoint = "http://127.0.0.1:1"
			}, queryResponses[:2]...),
			call: call{method: "GET", path: mockBase + "/method/other"}, wantStatus: 500,
		},
		{
			name: "xpath",
			uri:  "/method/test",
			method: method("M1", domain.MethodPOST, func(m *domain.RestMethod) { m.ResponseStrategy = domain.RestStrategyXPath },
				response("R1", "A", "no", func(r *domain.RestMockResponse) {
					r.XPathExpressions = []domain.RestXPathExpression{{Expression: "//request/name[text()='Other']"}}
				}),
				response("R2", "B", xmlBody, func(r *domain.RestMockResponse) {
					r.XPathExpressions = []domain.RestXPathExpression{{Expression: "//request/name[text()='Request']"}}
				})),
			call: call{method: "POST", path: mockBase + "/method/test", body: "<request><name>Request</name></request>"},
			wantStatus: 200, wantBody: xmlBody,
		},
		{
			name: "json path",
			uri:  "/method/test",
			method: method("M1", domain.MethodPOST, func(m *domain.RestMethod) { m.ResponseStrategy = domain.RestStrategyJSONPath },
				response("R1", "A", "no", func(r *domain.RestMockResponse) {
					r.JSONPathExpressions = []domain.RestJSONPathExpression{{Expression: "$.request[?(@.name == 'Other')]"}}
				}),
				response("R2", "B", "yes", func(r *domain.RestMockResponse) {
					r.JSONPathExpressions = []domain.RestJSONPathExpression{{Expression: "$.request.name"}}
				})),
			call:       call{method: "POST", path: mockBase + "/method/test", body: `{"request": {"name": "Request"}}`},
			wantStatus: 200, wantBody: "yes",
		},
		{
			name: "header query match",
			uri:  "/method/test",
			method: method("M1", domain.MethodGET, func(m *domain.RestMethod) { m.ResponseStrategy = domain.RestStrategyHeaderQueryMatch },
				response("R1", "A", "qa", func(r *domain.RestMockResponse) {
					r.HeaderQueries = []domain.RestHeaderQuery{{Header: "x-env", Query: "qa"}}
				}),
				response("R2", "B", "prod", func(r *domain.RestMockResponse) {
					r.HeaderQueries = []domain.RestHeaderQuery{{Header: "X-Env", Query: "prod"}}
				})),
			call:       call{method: "GET", path: mockBase + "/method/test", headers: map[string]string{"X-Env": "PROD"}},
			wantStatus: 200, wantBody: "prod",
		},
		{
			name: "accept header narrows candidates",
			uri:  "/method/test",
			method: method("M1", domain.MethodGET, nil,
				response("R1", "XML", "xml"),
				response("R2", "JSON", "json", func(r *domain.RestMockResponse) {
					r.HTTPHeaders = []domain.HTTPHeader{{Name: "Content-Type", Value: "application/json"}}
				})),
			call:       call{method: "GET", path: mockBase + "/method/test", headers: map[string]string{"Accept": "application/json"}},
			wantStatus: 200, wantBody: "json",
		},
		{
			name: "echo",
			uri:  "/method/test",
			method: method("M1", domain.MethodPOST, func(m *domain.RestMethod) { m.Status = domain.RestMethodEcho }),
			call: call{method: "POST", path: mockBase + "/method/test", body: "<request>\r\n</request>\r\n",
				headers: map[string]string{"Content-Type": "application/json"}},
			wantStatus: 200, wantBody: "<request>\n</request>", wantHeader: map[string]string{"Content-Type": "application/json"},
		},
		{
			name:   "disabled",
			uri:    "/method/test",
			method: method("M1", domain.MethodGET, func(m *domain.RestMethod) { m.Status = domain.RestMethodDisabled }, response("R1", "A", "x")),
			call:   call{method: "GET", path: mockBase + "/method/test"}, wantStatus: 500,
		},
		{
			name:   "no enabled response",
			uri:    "/method/test",
			method: method("M1", domain.MethodGET, nil, response("R1", "A", "x", func(r *domain.RestMockResponse) { r.Status = domain.RestMockResponseDisabled })),
			call:   call{method: "GET", path: mockBase + "/method/test"}, wantStatus: 500,
		},
		{
			name:   "unknown method",
			uri:    "/method/test",
			method: method("M1", domain.MethodGET, nil, response("R1", "A", "x")),
			call:   call{method: "DELETE", path: mockBase + "/method/test"}, wantStatus: 500,
		},
		{
			name:   "unknown resource",
			uri:    "/method/test",
			method: method("M1", domain.MethodGET, nil, response("R1", "A", "x")),
			call:   call{method: "GET", path: mockBase + "/method/other"}, wantStatus: 500,
		},
		{
			name: "status code and content encoding header dropped",
			uri:  "/method/test",
			method: method("M1", domain.MethodGET, nil, response("R1", "A", "created", func(r *domain.RestMockResponse) {
				r.HTTPStatusCode = 201
				r.HTTPHeaders = []domain.HTTPHeader{{Name: "Content-Encoding", Value: "gzip"}, {Name: "X-A", Value: "1"}, {Name: "x-a", Value: "2"}}
				r.ContentEncodings = []domain.ContentEncoding{domain.EncodingGZIP}
			})),
			call:       call{method: "GET", path: mockBase + "/method/test"},
			wantStatus: 201, wantBody: "created",
			wantHeader: map[string]string{"Content-Encoding": "", "X-A": "2", "Content-Type": "text/plain;charset=UTF-8"},
		},
		{
			name:   "default content type follows accept",
			uri:    "/method/test",
			method: method("M1", domain.MethodGET, nil, response("R1", "A", "{}", func(r *domain.RestMockResponse) { r.HTTPHeaders = nil })),
			call:   call{method: "GET", path: mockBase + "/method/test", headers: map[string]string{"Accept": "application/json"}},
			wantStatus: 200, wantBody: "{}", wantHeader: map[string]string{"Content-Type": "application/json"},
		},
		{
			name: "unknown status code",
			uri:  "/method/test",
			method: method("M1", domain.MethodGET, nil, response("R1", "A", "x", func(r *domain.RestMockResponse) {
				r.HTTPStatusCode = 299
			})),
			call: call{method: "GET", path: mockBase + "/method/test"}, wantStatus: 500,
		},
		{
			name: "expressions",
			uri:  "/user/{id}?filter={f}",
			method: method("M1", domain.MethodGET, nil, response("R1", "A",
				`${PATH_PARAMETER(parameter="id")}|${PATH_PARAMETER(parameter="filter")}|${QUERY_STRING(query="other")}|${URL_HOST()}|${RANDOM_X}`,
				func(r *domain.RestMockResponse) { r.UsingExpressions = domain.Ptr(true) })),
			call:       call{method: "GET", path: mockBase + "/user/a%20b?filter=x&other=y"},
			wantStatus: 200, wantBody: "a b|x|y|example.com|${RANDOM_X}",
		},
		{
			name: "expressions disabled",
			uri:  "/user",
			method: method("M1", domain.MethodGET, nil, response("R1", "A", "${URL_HOST()}")),
			call: call{method: "GET", path: mockBase + "/user"}, wantStatus: 200, wantBody: "${URL_HOST()}",
		},
		{
			name: "network delay",
			uri:  "/user",
			method: method("M1", domain.MethodGET, func(m *domain.RestMethod) {
				m.SimulateNetworkDelay = domain.Ptr(true)
				m.NetworkDelay = domain.Ptr(int64(20))
			}, response("R1", "A", "late")),
			call: call{method: "GET", path: mockBase + "/user"}, wantStatus: 200, wantBody: "late",
		},
		{
			name:   "empty resource uri",
			uri:    "",
			method: method("M1", domain.MethodGET, nil, response("R1", "A", "root")),
			call:   call{method: "GET", path: mockBase}, wantStatus: 200, wantBody: "root",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := newFakeStore(project(resource("RES1", tt.uri, tt.method)))
			rec := do(t, newEngine(store), tt.call)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus == 500 {
				var e errorBody
				if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Status != 500 || e.Error != "Internal Server Error" || e.Path == "" {
					t.Errorf("error body = %s (%v)", rec.Body, err)
				}
				return
			}
			if rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body, tt.wantBody)
			}
			for k, v := range tt.wantHeader {
				if got := rec.Header().Get(k); got != v {
					t.Errorf("header %s = %q, want %q", k, got, v)
				}
			}
		})
	}
}

func TestSequence(t *testing.T) {
	t.Parallel()
	m := method("M1", domain.MethodGET, func(m *domain.RestMethod) { m.ResponseStrategy = domain.RestStrategySequence },
		response("R2", "Response 10", "third"), response("R1", "Response 2", "second"), response("R0", "Response 1", "first"))
	store := newFakeStore(project(resource("RES1", "/seq", m)))
	e := newEngine(store)
	var got []string
	for range 4 {
		got = append(got, do(t, e, call{method: "GET", path: mockBase + "/seq"}).Body.String())
	}
	if want := "first,second,third,first"; strings.Join(got, ",") != want {
		t.Errorf("sequence = %v, want %s", got, want)
	}
}

// IdentifyRestMethodServiceTest.testProcessSamePath and D-4.
func TestMostSpecificResourceWins(t *testing.T) {
	t.Parallel()
	store := newFakeStore(project(
		resource("RES1", "/user/{userId}", method("M1", domain.MethodGET, nil, response("R1", "A", "by id"))),
		resource("RES2", "/user/search", method("M2", domain.MethodGET, nil, response("R2", "A", "search"))),
		resource("RES3", "/user/{a}/{b}", method("M3", domain.MethodPOST, nil, response("R3", "A", "post"))),
		resource("RES4", "/user/{a}/resource", method("M4", domain.MethodGET, nil, response("R4", "A", "nested"))),
	))
	e := newEngine(store)
	tests := map[string]string{
		"GET " + mockBase + "/user/search":     "search",
		"GET " + mockBase + "/user/7":          "by id",
		"GET " + mockBase + "/user/7/resource": "nested",
		"POST " + mockBase + "/user/7/x":       "post",
	}
	for c, want := range tests {
		m, path, _ := strings.Cut(c, " ")
		if got := do(t, e, call{method: m, path: path}).Body.String(); got != want {
			t.Errorf("%s = %q, want %q", c, got, want)
		}
	}
}

func TestEvents(t *testing.T) {
	t.Parallel()
	store := newFakeStore(project(
		resource("RES1", "/ok", method("M1", domain.MethodPOST, nil, response("R1", "Mocked", "hi"))),
		resource("RES2", "/off", method("M2", domain.MethodGET, func(m *domain.RestMethod) { m.Status = domain.RestMethodDisabled })),
	))
	e := newEngine(store)
	do(t, e, call{method: "POST", path: mockBase + "/ok?a=1", body: "payload", headers: map[string]string{"Content-Type": "text/plain"}})
	do(t, e, call{method: "GET", path: mockBase + "/off"})
	do(t, e, call{method: "GET", path: mockBase + "/missing"})
	events := store.restEvents()
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2 (no event when the method is not identified)", len(events))
	}
	ok := events[0]
	if ok.ResourceName != "method M1" || ok.ProjectID != projectID || ok.ApplicationID != applicationID || ok.ResourceID != "RES1" || ok.MethodID != "M1" {
		t.Errorf("event ids = %+v", ok)
	}
	if ok.Request.URI != "/ok" || ok.Request.Body != "payload" || ok.Request.ContentType != "text/plain" || ok.Request.HTTPMethod != domain.MethodPOST {
		t.Errorf("event request = %+v", ok.Request)
	}
	if len(ok.Request.HTTPParameters) != 1 || ok.Request.HTTPParameters[0] != (domain.HTTPParameter{Name: "a", Value: "1"}) {
		t.Errorf("event parameters = %+v", ok.Request.HTTPParameters)
	}
	if ok.Response == nil || ok.Response.Body != "hi" || ok.Response.MockResponseName != "Mocked" || ok.Response.HTTPStatusCode != 200 {
		t.Errorf("event response = %+v", ok.Response)
	}
	if ok.StartDate.IsZero() || ok.EndDate.Before(ok.StartDate.Time) {
		t.Errorf("event dates = %v %v", ok.StartDate, ok.EndDate)
	}
	if events[1].Response != nil {
		t.Errorf("failed request must have a nil response, got %+v", events[1].Response)
	}
}

func TestForwardingAndRecording(t *testing.T) {
	t.Parallel()
	var seen struct{ method, uri, body, custom string }
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen.method, seen.uri, seen.body, seen.custom = r.Method, r.RequestURI, string(b), r.Header.Get("X-Custom")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Backend", "yes")
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"forwarded":true}`))
	}))
	t.Cleanup(backend.Close)

	statusMethod := func(id string, status domain.RestMethodStatus, httpMethod domain.HTTPMethod) domain.RestMethod {
		return method(id, httpMethod, func(m *domain.RestMethod) {
			m.Status = status
			m.ForwardedEndpoint = backend.URL + "/api"
		})
	}
	store := newFakeStore(project(
		resource("RES1", "/fwd/{id}", statusMethod("M1", domain.RestMethodForwarded, domain.MethodPOST), statusMethod("M4", domain.RestMethodForwarded, domain.MethodGET)),
		resource("RES2", "/rec", statusMethod("M2", domain.RestMethodRecording, domain.MethodGET)),
		resource("RES3", "/once", statusMethod("M3", domain.RestMethodRecordOnce, domain.MethodGET)),
		resource("RES5", "/auto/{v}", method("M5", domain.MethodGET, func(m *domain.RestMethod) {
			m.ResponseStrategy = domain.RestStrategyQueryMatch
			m.AutomaticForward = domain.Ptr(true)
			m.ForwardedEndpoint = backend.URL
		}, response("R5", "never", "x", func(r *domain.RestMockResponse) {
			r.ParameterQueries = []domain.RestParameterQuery{{Parameter: "v", Query: "1"}}
		}))),
		resource("RES6", "/broken", method("M6", domain.MethodGET, func(m *domain.RestMethod) {
			m.Status = domain.RestMethodForwarded
			m.ForwardedEndpoint = "http://127.0.0.1:1"
		})),
	))
	e := newEngine(store)

	rec := do(t, e, call{method: "POST", path: mockBase + "/fwd/7?q=a", body: "data", headers: map[string]string{"X-Custom": "c"}})
	if rec.Code != 202 || rec.Body.String() != `{"forwarded":true}` || rec.Header().Get("X-Backend") != "yes" {
		t.Errorf("forwarded response = %d %q %v", rec.Code, rec.Body, rec.Header())
	}
	if seen.method != "POST" || seen.uri != "/api/fwd/7?q=a" || seen.body != "data" || seen.custom != "c" {
		t.Errorf("backend saw %+v", seen)
	}
	do(t, e, call{method: "GET", path: mockBase + "/fwd/7", body: "ignored"})
	if seen.method != "GET" || seen.body != "" {
		t.Errorf("GET forwards no body, backend saw %+v", seen)
	}

	do(t, e, call{method: "GET", path: mockBase + "/rec"})
	do(t, e, call{method: "GET", path: mockBase + "/once"})
	p := store.project(projectID)
	app := p.Application(applicationID)
	recorded := app.Resource("RES2").Method("M2")
	if len(recorded.MockResponses) != 1 || recorded.Status != domain.RestMethodRecording {
		t.Fatalf("recording method = %+v", recorded)
	}
	r := recorded.MockResponses[0]
	if !strings.HasPrefix(r.Name, "Recorded response ") || r.Body != `{"forwarded":true}` || r.HTTPStatusCode != 202 ||
		r.Status != domain.RestMockResponseEnabled || domain.Deref(r.UsingExpressions) || r.ID == "" || r.MethodID != "M2" {
		t.Errorf("recorded response = %+v", r)
	}
	once := app.Resource("RES3").Method("M3")
	if len(once.MockResponses) != 1 || once.Status != domain.RestMethodMocked {
		t.Errorf("record once method = %+v", once)
	}

	if rec := do(t, e, call{method: "GET", path: mockBase + "/auto/2"}); rec.Code != 202 || seen.uri != "/auto/2" {
		t.Errorf("automatic forward = %d, backend saw %+v", rec.Code, seen)
	}
	if rec := do(t, e, call{method: "GET", path: mockBase + "/broken"}); rec.Code != 500 {
		t.Errorf("unreachable endpoint = %d, want 500", rec.Code)
	}
}

func TestRejectedPaths(t *testing.T) {
	t.Parallel()
	e := newEngine(newFakeStore(project()))
	if rec := do(t, e, call{method: "GET", path: "/castlemock/mock/rest/project/P1"}); rec.Code != 404 {
		t.Errorf("incomplete path = %d, want 404", rec.Code)
	}
	if rec := do(t, e, call{method: "PROPFIND", path: mockBase + "/x"}); rec.Code != 405 {
		t.Errorf("unsupported method = %d, want 405", rec.Code)
	}
}

func TestRootContextPath(t *testing.T) {
	t.Parallel()
	store := newFakeStore(project(resource("RES1", "/x", method("M1", domain.MethodGET, nil, response("R1", "A", "root ctx")))))
	e := NewEngine(Deps{Store: store, ContextPath: ""})
	rec := do(t, e, call{method: "GET", path: "/mock/rest/project/" + projectID + "/application/" + applicationID + "/x"})
	if rec.Body.String() != "root ctx" {
		t.Errorf("body = %q", rec.Body)
	}
}
