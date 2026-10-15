package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/config"
	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/service"
	"github.com/duv0-x/mockingbird/internal/service/servicetest"
)

const ctxPath = "/castlemock"

var (
	hashOnce sync.Once
	hashed   string
)

// passwordHash is computed once: bcrypt is deliberately slow.
func passwordHash(t *testing.T) string {
	hashOnce.Do(func() {
		h, err := auth.HashPassword("secret")
		if err != nil {
			t.Fatal(err)
		}
		hashed = h
	})
	return hashed
}

type env struct {
	t      *testing.T
	store  *servicetest.MemStore
	auth   *auth.Service
	engine *gin.Engine
	users  map[domain.Role]domain.User
}

func newEnv(t *testing.T) *env {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store := servicetest.NewMemStore()
	e := &env{t: t, store: store, users: map[domain.Role]domain.User{}}
	for _, r := range []domain.Role{domain.RoleReader, domain.RoleModifier, domain.RoleAdmin} {
		u := domain.User{ID: "u" + string(r)[:3], Username: strings.ToLower(string(r)), Password: passwordHash(t),
			Status: domain.UserActive, Role: r, Created: domain.Now(), Updated: domain.Now()}
		if err := store.Users().Save(context.Background(), u); err != nil {
			t.Fatal(err)
		}
		e.users[r] = u
	}
	a, err := auth.New(store.Users(), auth.Options{KeyFile: filepath.Join(t.TempDir(), "secrets", "jwt.key")})
	if err != nil {
		t.Fatal(err)
	}
	e.auth = a
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Default()
	cfg.ContextPath = ctxPath
	cfg.DataDir = "/data"
	cfg.UploadMaxBytes = 1 << 20
	svc := service.New(service.Deps{Config: cfg, Store: store, Version: "1.2.3", Logger: logger})
	r := gin.New()
	// A route that does not exist answers 418 so tests can tell "not routed"
	// apart from a handler's 404 for an unknown entity.
	r.NoRoute(func(c *gin.Context) { c.Status(http.StatusTeapot) })
	g := r.Group(ctxPath, a.Middleware())
	Register(g, Deps{Config: cfg, Service: svc, Auth: a, Logger: logger})
	e.engine = r
	return e
}

func (e *env) token(role domain.Role) string {
	tok, err := e.auth.Token(e.users[role].ID)
	if err != nil {
		e.t.Fatal(err)
	}
	return tok
}

type reqOpt func(*http.Request)

func as(e *env, role domain.Role) reqOpt {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+e.token(role)) }
}

func (e *env) do(method, path string, body any, opts ...reqOpt) *httptest.ResponseRecorder {
	e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		data, err := json.Marshal(b)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, ctxPath+path, rd)
	if rd != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, o := range opts {
		o(req)
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return v
}

func wantStatus(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d; body %s", w.Code, status, w.Body.String())
	}
}

// TestCastleMockCallCoverage asserts that every call made by CastleMock's
// React frontend is routed with the expected role (SPEC.md §5.6).
func TestCastleMockCallCoverage(t *testing.T) {
	f, err := os.Open("testdata/castlemock_calls.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	e := newEnv(t)
	lower := map[string]domain.Role{"M": domain.RoleReader, "A": domain.RoleModifier}
	allowed := map[string]domain.Role{"R": domain.RoleReader, "M": domain.RoleModifier, "A": domain.RoleAdmin}
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			t.Fatalf("bad line %q", line)
		}
		method, path, role := fields[0], strings.TrimPrefix(strings.ReplaceAll(fields[1], "{x}", "zz9999"), "/api/rest"), fields[2]
		path = "/api/rest" + path
		n++
		t.Run(method+" "+fields[1], func(t *testing.T) {
			if role == "public" {
				w := e.do(method, path, nil)
				// Login without credentials legitimately answers 401.
				unauthorized := w.Code == http.StatusUnauthorized && !strings.HasSuffix(path, "/login")
				if w.Code == http.StatusTeapot || unauthorized || w.Code == http.StatusForbidden {
					t.Fatalf("public call answered %d", w.Code)
				}
				return
			}
			if w := e.do(method, path, nil); w.Code != http.StatusUnauthorized {
				t.Fatalf("without principal: status %d, want 401", w.Code)
			}
			if r, ok := lower[role]; ok {
				if w := e.do(method, path, nil, as(e, r)); w.Code != http.StatusForbidden {
					t.Fatalf("as %s: status %d, want 403", r, w.Code)
				}
			}
			w := e.do(method, path, nil, as(e, allowed[role]))
			if w.Code == http.StatusTeapot || w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden || w.Code >= 500 {
				t.Fatalf("as %s: status %d (%s)", allowed[role], w.Code, w.Body.String())
			}
		})
	}
	if n < 70 {
		t.Fatalf("only %d calls listed", n)
	}
}

// TestRouteTableMatchesCoverageFile checks the declared access of the route
// serving each CastleMock call, independently of the HTTP behavior.
func TestRouteTableMatchesCoverageFile(t *testing.T) {
	data, err := os.ReadFile("testdata/castlemock_calls.txt")
	if err != nil {
		t.Fatal(err)
	}
	routes := Routes()
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		var found *Route
		for i := range routes {
			if routes[i].Method == f[0] && patternMatches(routes[i].Path, f[1]) {
				if found == nil || staticSegments(routes[i].Path) > staticSegments(found.Path) {
					found = &routes[i]
				}
			}
		}
		if found == nil {
			t.Errorf("%s %s: no route", f[0], f[1])
			continue
		}
		if string(found.Access) != f[2] {
			t.Errorf("%s %s: route %s has access %s, want %s", f[0], f[1], found.Path, found.Access, f[2])
		}
	}
}

func patternMatches(pattern, path string) bool {
	ps, xs := strings.Split(pattern, "/"), strings.Split(path, "/")
	if len(ps) != len(xs) {
		return false
	}
	for i := range ps {
		if strings.HasPrefix(ps[i], ":") {
			continue
		}
		if ps[i] != xs[i] {
			return false
		}
	}
	return true
}

func staticSegments(pattern string) int {
	n := 0
	for _, s := range strings.Split(pattern, "/") {
		if !strings.HasPrefix(s, ":") {
			n++
		}
	}
	return n
}

func TestLoginLogout(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name     string
		body     any
		status   int
		username string
	}{
		{"valid", map[string]string{"username": "admin", "password": "secret"}, 200, "admin"},
		{"case-insensitive username", map[string]string{"username": "ADMIN", "password": "secret"}, 200, "admin"},
		{"bad password", map[string]string{"username": "admin", "password": "nope"}, 401, ""},
		{"unknown user", map[string]string{"username": "ghost", "password": "secret"}, 401, ""},
		{"empty body", nil, 401, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := e.do(http.MethodPost, "/api/rest/core/login", tt.body)
			wantStatus(t, w, tt.status)
			if tt.status != 200 {
				return
			}
			resp := decode[map[string]string](t, w)
			if resp["username"] != tt.username || resp["role"] != "ADMIN" || resp["token"] == "" {
				t.Fatalf("response %v", resp)
			}
			ck := w.Header().Get("Set-Cookie")
			for _, want := range []string{"token=" + resp["token"], "Max-Age=604800", "Path=/", "HttpOnly", "SameSite=Lax"} {
				if !strings.Contains(ck, want) {
					t.Fatalf("Set-Cookie %q lacks %q", ck, want)
				}
			}
			// The cookie authenticates later calls.
			w = e.do(http.MethodGet, "/api/rest/core/profile", nil, func(r *http.Request) {
				r.AddCookie(&http.Cookie{Name: "token", Value: resp["token"]})
			})
			wantStatus(t, w, 200)
			if p := decode[domain.User](t, w); p.Username != "admin" || p.Password != "" {
				t.Fatalf("profile %+v", p)
			}
		})
	}

	t.Run("inactive user cannot log in", func(t *testing.T) {
		u := e.users[domain.RoleReader]
		u.Status = domain.UserInactive
		_ = e.store.Users().Save(context.Background(), u)
		defer func() { u.Status = domain.UserActive; _ = e.store.Users().Save(context.Background(), u) }()
		w := e.do(http.MethodPost, "/api/rest/core/login", map[string]string{"username": "reader", "password": "secret"})
		wantStatus(t, w, 401)
	})

	t.Run("logout clears cookies", func(t *testing.T) {
		w := e.do(http.MethodGet, "/api/rest/core/logout", nil)
		wantStatus(t, w, 200)
		cookies := strings.Join(w.Header().Values("Set-Cookie"), "\n")
		for _, want := range []string{"token=; Path=/; Max-Age=0; HttpOnly", "JSESSIONID=; Path=/; Max-Age=0; HttpOnly"} {
			if !strings.Contains(cookies, want) {
				t.Fatalf("Set-Cookie %q lacks %q", cookies, want)
			}
		}
	})
}

func TestPrincipalResolution(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name   string
		opt    reqOpt
		status int
	}{
		{"none", func(*http.Request) {}, 401},
		{"bearer", as(e, domain.RoleReader), 200},
		{"any scheme word", func(r *http.Request) { r.Header.Set("Authorization", "Token "+e.token(domain.RoleReader)) }, 200},
		{"basic", func(r *http.Request) { r.SetBasicAuth("Reader", "secret") }, 200},
		{"basic wrong password", func(r *http.Request) { r.SetBasicAuth("reader", "bad") }, 401},
		{"garbage token", func(r *http.Request) { r.Header.Set("Authorization", "Bearer abc") }, 401},
		{"three parts", func(r *http.Request) { r.Header.Set("Authorization", "Bearer x "+e.token(domain.RoleReader)) }, 401},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := e.do(http.MethodGet, "/api/rest/core/project", nil, tt.opt)
			wantStatus(t, w, tt.status)
			if tt.status == 401 {
				if w.Header().Get("WWW-Authenticate") != "" {
					t.Fatal("WWW-Authenticate must not be sent")
				}
				body := decode[map[string]any](t, w)
				if body["status"] != float64(401) || body["error"] != "Unauthorized" || body["path"] != ctxPath+"/api/rest/core/project" {
					t.Fatalf("error body %v", body)
				}
			}
		})
	}

	t.Run("token of a deleted user is rejected and the cookie cleared (D-5)", func(t *testing.T) {
		tok := e.token(domain.RoleModifier)
		if _, err := e.store.Users().Delete(context.Background(), e.users[domain.RoleModifier].ID); err != nil {
			t.Fatal(err)
		}
		w := e.do(http.MethodGet, "/api/rest/core/project", nil, func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: "token", Value: tok})
		})
		wantStatus(t, w, 401)
		if !strings.Contains(w.Header().Get("Set-Cookie"), "token=; Path=/; Max-Age=0") {
			t.Fatalf("cookie not cleared: %q", w.Header().Get("Set-Cookie"))
		}
	})
}

func TestPublicCoreEndpoints(t *testing.T) {
	e := newEnv(t)
	if got := decode[map[string]string](t, e.do(http.MethodGet, "/api/rest/core/version", nil)); got["version"] != "1.2.3" {
		t.Fatalf("version %v", got)
	}
	if got := decode[map[string]string](t, e.do(http.MethodGet, "/api/rest/core/context", nil)); got["context"] != ctxPath {
		t.Fatalf("context %v", got)
	}
	doc := decode[map[string]any](t, e.do(http.MethodGet, "/doc/api/rest", nil))
	paths := doc["paths"].(map[string]any)
	if doc["openapi"] != "3.0.3" || paths["/api/rest/rest/project/{projectId}"] == nil {
		t.Fatalf("openapi doc lacks paths: %v", doc["openapi"])
	}
	sys := e.do(http.MethodGet, "/api/rest/core/system", nil, as(e, domain.RoleAdmin))
	wantStatus(t, sys, 200)
	info := decode[domain.SystemInformation](t, sys)
	if info.JavaVendor != "Go" || info.TomcatVersion != "1.2.3" || info.CastleMockHomeDirectory != "/data" || info.AvailableProcessors == 0 {
		t.Fatalf("system %+v", info)
	}
}

func TestRestProjectFlow(t *testing.T) {
	e := newEnv(t)
	mod := as(e, domain.RoleModifier)

	w := e.do(http.MethodPost, "/api/rest/rest/project", map[string]string{"name": "Pets", "projectType": "REST"}, mod)
	wantStatus(t, w, 200)
	p := decode[domain.RestProject](t, w)
	if p.ID == "" || p.Name != "Pets" || p.Applications == nil {
		t.Fatalf("project %+v", p)
	}
	w = e.do(http.MethodPost, "/api/rest/rest/project/"+p.ID+"/application", map[string]string{"name": "petstore"}, mod)
	wantStatus(t, w, 200)
	app := decode[domain.RestApplication](t, w)

	// Parent ids in the URL are ignored: the last id wins (§5.2).
	w = e.do(http.MethodPost, "/api/rest/rest/project/ignored/application/"+app.ID+"/resource", map[string]string{"name": "pet", "uri": "/pet/{petId}"}, mod)
	wantStatus(t, w, 200)
	res := decode[domain.RestResource](t, w)

	w = e.do(http.MethodPost, "/api/rest/rest/project/x/application/x/resource/"+res.ID+"/method", map[string]string{"name": "getPet", "httpMethod": "GET"}, mod)
	wantStatus(t, w, 200)
	m := decode[domain.RestMethod](t, w)
	if m.Status != domain.RestMethodMocked || m.ResponseStrategy != domain.RestStrategyRandom || m.URI != "/pet/{petId}" {
		t.Fatalf("method %+v", m)
	}

	base := "/api/rest/rest/project/x/application/x/resource/x/method/" + m.ID
	w = e.do(http.MethodPost, base+"/mockresponse", map[string]any{"name": "ok", "body": "{}"}, mod)
	wantStatus(t, w, 200)
	mr := decode[domain.RestMockResponse](t, w)
	if mr.HTTPStatusCode != 200 || mr.Status != domain.RestMockResponseEnabled || domain.Deref(mr.UsingExpressions) {
		t.Fatalf("mock response %+v", mr)
	}
	raw := e.do(http.MethodGet, base+"/mockresponse/"+mr.ID, nil, as(e, domain.RoleReader)).Body.String()
	for _, want := range []string{`"httpHeaders":[]`, `"parameterQueries":[]`, `"body":"{}"`} {
		if !strings.Contains(raw, want) {
			t.Fatalf("mock response JSON %s lacks %s", raw, want)
		}
	}

	wantStatus(t, e.do(http.MethodPost, base+"/mockresponse/duplicate", map[string]any{"mockResponseIds": []string{mr.ID, "unknown"}}, mod), 200)
	w = e.do(http.MethodPut, base, map[string]any{"name": "getPet", "httpMethod": "GET", "status": "MOCKED",
		"responseStrategy": "SEQUENCE", "defaultMockResponseId": mr.ID}, mod)
	wantStatus(t, w, 200)
	w = e.do(http.MethodGet, base, nil, as(e, domain.RoleReader))
	got := decode[domain.RestMethod](t, w)
	if len(got.MockResponses) != 2 || got.MockResponses[1].ID == mr.ID || got.MockResponses[1].Name != "ok" || got.DefaultResponseName != "ok" {
		t.Fatalf("method after duplicate %+v", got)
	}

	// D-11: the React frontend's wrong path is accepted as an alias.
	wantStatus(t, e.do(http.MethodPut, "/api/rest/rest/project/"+p.ID+"/port/endpoint/forwarded",
		map[string]any{"applicationIds": []string{app.ID}, "forwardedEndpoint": "http://upstream"}, mod), 200)
	wantStatus(t, e.do(http.MethodPut, "/api/rest/rest/project/"+p.ID+"/application/status",
		map[string]any{"applicationIds": []string{app.ID, "nope"}, "status": "DISABLED"}, mod), 200)
	got = decode[domain.RestMethod](t, e.do(http.MethodGet, base, nil, mod))
	if got.ForwardedEndpoint != "http://upstream" || got.Status != domain.RestMethodDisabled {
		t.Fatalf("bulk updates not applied: %+v", got)
	}
	project := decode[domain.RestProject](t, e.do(http.MethodGet, "/api/rest/rest/project/"+p.ID, nil, mod))
	if project.Applications[0].StatusCount[domain.RestMethodDisabled] != 1 || len(project.Applications[0].Resources) != 0 {
		t.Fatalf("project view %+v", project.Applications[0])
	}

	params := decode[[]string](t, e.do(http.MethodGet, "/api/rest/rest/project/x/application/x/resource/"+res.ID+"/parameter", nil, mod))
	if len(params) != 1 || params[0] != "petId" {
		t.Fatalf("parameters %v", params)
	}

	// Errors (D-12).
	w = e.do(http.MethodGet, "/api/rest/rest/project/nope", nil, mod)
	wantStatus(t, w, 404)
	if w.Body.Len() != 0 {
		t.Fatalf("404 body %q", w.Body.String())
	}
	wantStatus(t, e.do(http.MethodPost, "/api/rest/rest/project", map[string]string{"name": ""}, mod), 400)
	wantStatus(t, e.do(http.MethodPost, "/api/rest/rest/project", "{not json", mod), 400)
	wantStatus(t, e.do(http.MethodPut, base, map[string]any{"name": "x", "httpMethod": "BREW", "status": "MOCKED", "responseStrategy": "RANDOM"}, mod), 400)
	wantStatus(t, e.do(http.MethodPost, "/api/rest/rest/project", map[string]string{"name": "Other"}, mod), 200)
	w = e.do(http.MethodPut, "/api/rest/rest/project/"+p.ID, map[string]string{"name": "other"}, mod)
	wantStatus(t, w, 409)
	if body := decode[map[string]any](t, w); body["error"] != "Conflict" {
		t.Fatalf("409 body %v", body)
	}
	wantStatus(t, e.do(http.MethodGet, "/api/rest/rest/project/"+p.ID+"/export?format=yaml", nil, mod), 400)

	w = e.do(http.MethodDelete, "/api/rest/rest/project/"+p.ID, nil, mod)
	wantStatus(t, w, 200)
	if decode[domain.RestProject](t, w).ID != p.ID {
		t.Fatal("delete did not return the project")
	}
	wantStatus(t, e.do(http.MethodGet, base, nil, mod), 404)
}

func TestSoapFlow(t *testing.T) {
	e := newEnv(t)
	mod := as(e, domain.RoleModifier)
	w := e.do(http.MethodPost, "/api/rest/soap/project", map[string]string{"name": "Calc"}, mod)
	wantStatus(t, w, 200)
	p := decode[domain.SoapProject](t, w)
	wantStatus(t, e.do(http.MethodPost, "/api/rest/soap/project", map[string]string{"name": "calc"}, mod), 409)

	// Seed a port/operation directly: WSDL import belongs to T3.
	_, err := e.store.Soap().Update(context.Background(), p.ID, func(sp *domain.SoapProject) error {
		sp.Ports = append(sp.Ports, domain.SoapPort{ID: "Port01", Name: "CalcSoap", URI: "CalcSoap", Operations: []domain.SoapOperation{{
			ID: "Oper01", Name: "Add", OperationIdentifier: domain.SoapOperationIdentifier{Name: "Add"},
			Status: domain.SoapOperationMocked, ResponseStrategy: domain.SoapStrategyRandom, HTTPMethod: domain.MethodPOST,
			SoapVersion: domain.SOAP11, IdentifyStrategy: domain.IdentifyByElementNamespace, DefaultBody: "<x/>",
		}}})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	op := "/api/rest/soap/project/" + p.ID + "/port/Port01/operation/Oper01"
	w = e.do(http.MethodPost, op+"/mockresponse", map[string]any{"name": "r1", "body": "<a>&</a>", "status": "ENABLED", "httpStatusCode": 200}, mod)
	wantStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), `"body":"<a>&</a>"`) {
		t.Fatalf("body must not be HTML-escaped: %s", w.Body.String())
	}
	mr := decode[domain.SoapMockResponse](t, w)
	wantStatus(t, e.do(http.MethodPost, op+"/mockresponse/duplicate", map[string]any{"mockResponseIds": []string{mr.ID}}, mod), 200)
	w = e.do(http.MethodPut, op, map[string]any{"status": "ECHO", "responseStrategy": "SEQUENCE", "identifyStrategy": "ELEMENT"}, mod)
	wantStatus(t, w, 200)
	o := decode[domain.SoapOperation](t, w)
	if o.Status != domain.SoapOperationEcho || o.MockOnFailure == nil || *o.MockOnFailure || len(o.MockResponses) != 2 {
		t.Fatalf("operation %+v", o)
	}
	wantStatus(t, e.do(http.MethodPut, "/api/rest/soap/project/"+p.ID+"/port/status", map[string]any{"portIds": []string{"Port01"}, "status": "DISABLED"}, mod), 200)
	sp := decode[domain.SoapProject](t, e.do(http.MethodGet, "/api/rest/soap/project/"+p.ID, nil, mod))
	if sp.Ports[0].StatusCount[domain.SoapOperationDisabled] != 1 || len(sp.Ports[0].Operations) != 0 {
		t.Fatalf("project view %+v", sp.Ports[0])
	}
	w = e.do(http.MethodPut, "/api/rest/soap/project/x/port/Port01", map[string]string{"uri": "Calc2"}, mod)
	wantStatus(t, w, 200)
	if decode[domain.SoapPort](t, w).URI != "Calc2" {
		t.Fatal("port uri not updated")
	}

	// Resources: upload, read metadata/content, public raw endpoint.
	w = uploadFile(t, e, "/api/rest/soap/project/"+p.ID+"/resource/x/import", "calc.wsdl", "<definitions/>", map[string]string{"resourceType": "WSDL"}, mod)
	wantStatus(t, w, 200)
	res := decode[domain.SoapResource](t, w)
	if res.Name != "calc.wsdl" || res.Type != domain.SoapResourceWSDL {
		t.Fatalf("resource %+v", res)
	}
	w = e.do(http.MethodGet, "/api/rest/soap/project/"+p.ID+"/resource/"+res.ID+"/content", nil, mod)
	wantStatus(t, w, 200)
	if w.Body.String() != "<definitions/>" || w.Header().Get("Content-Type") != "text/plain;charset=UTF-8" {
		t.Fatalf("content %q %q", w.Body.String(), w.Header().Get("Content-Type"))
	}
	w = e.do(http.MethodGet, "/resource/soap/project/"+p.ID+"/resource/"+res.ID, nil)
	wantStatus(t, w, 200)
	if w.Header().Get("Content-Type") != "text/xml;charset=UTF-8" {
		t.Fatalf("raw content type %q", w.Header().Get("Content-Type"))
	}
	wantStatus(t, e.do(http.MethodGet, "/resource/soap/project/"+p.ID+"/resource/nope", nil), 404)

	wantStatus(t, e.do(http.MethodDelete, "/api/rest/soap/project/x/port/Port01", nil, mod), 200)
	wantStatus(t, e.do(http.MethodGet, op, nil, mod), 404)
}

func uploadFile(t *testing.T, e *env, path, name, content string, fields map[string]string, opts ...reqOpt) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte(content))
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, ctxPath+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, o := range opts {
		o(req)
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func TestUploadErrors(t *testing.T) {
	e := newEnv(t)
	mod := as(e, domain.RoleModifier)
	wantStatus(t, e.do(http.MethodPost, "/api/rest/rest/project/import", nil, mod), 400)
	wantStatus(t, uploadFile(t, e, "/api/rest/rest/project/import", "p.txt", "hello", nil, mod), 400)
	big := strings.Repeat("x", 2<<20)
	wantStatus(t, uploadFile(t, e, "/api/rest/soap/project/import", "p.xml", big, nil, mod), http.StatusRequestEntityTooLarge)
}

func TestUsersAndProfile(t *testing.T) {
	e := newEnv(t)
	admin := as(e, domain.RoleAdmin)
	w := e.do(http.MethodPost, "/api/rest/core/user", map[string]string{"username": "bob", "password": "pw", "status": "ACTIVE", "role": "READER"}, admin)
	wantStatus(t, w, 200)
	bob := decode[domain.User](t, w)
	if bob.Password != "" {
		t.Fatal("password hash leaked")
	}
	wantStatus(t, e.do(http.MethodPost, "/api/rest/core/user", map[string]string{"username": "BOB", "password": "pw", "status": "ACTIVE", "role": "READER"}, admin), 409)
	users := decode[[]domain.User](t, e.do(http.MethodGet, "/api/rest/core/user", nil, admin))
	if len(users) != 4 {
		t.Fatalf("users %d", len(users))
	}
	for _, u := range users {
		if u.Password != "" {
			t.Fatal("password hash leaked in list")
		}
	}
	adminID := e.users[domain.RoleAdmin].ID
	wantStatus(t, e.do(http.MethodPut, "/api/rest/core/user/"+adminID, map[string]string{"username": "admin", "status": "ACTIVE", "role": "READER"}, admin), 400)
	wantStatus(t, e.do(http.MethodDelete, "/api/rest/core/user/"+adminID, nil, admin), 400)
	wantStatus(t, e.do(http.MethodDelete, "/api/rest/core/user/nope", nil, admin), 404)
	wantStatus(t, e.do(http.MethodPut, "/api/rest/core/user/nope", map[string]string{"username": "z", "status": "ACTIVE", "role": "READER"}, admin), 404)

	// Profile update without password keeps the hash (D-2).
	login := func(user, pass string) int {
		return e.do(http.MethodPost, "/api/rest/core/login", map[string]string{"username": user, "password": pass}).Code
	}
	bobAuth := func(r *http.Request) { r.SetBasicAuth("bob", "pw") }
	w = e.do(http.MethodPut, "/api/rest/core/profile", map[string]string{"username": "bob", "fullName": "Bob B"}, bobAuth)
	wantStatus(t, w, 200)
	if decode[domain.User](t, w).FullName != "Bob B" || login("bob", "pw") != 200 {
		t.Fatal("profile update broke the password")
	}
	wantStatus(t, e.do(http.MethodPut, "/api/rest/core/profile", map[string]string{"username": "Admin"}, bobAuth), 409)
	wantStatus(t, e.do(http.MethodDelete, "/api/rest/core/user/"+bob.ID, nil, admin), 200)
}
