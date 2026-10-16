package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/duv0-x/mockingbird/internal/config"
)

const (
	ctxPath       = "/castlemock"
	adminPassword = "e2e-admin"
)

// seedProject is a native REST project file served from MOCKINGBIRD_SEED_DIR.
const seedProject = `{
  "schemaVersion": 1,
  "kind": "RestProject",
  "id": "SeedP1",
  "name": "Seeded",
  "created": "2024-05-01T10:20:30.123Z",
  "updated": "2024-05-01T10:20:30.123Z",
  "applications": [{
    "id": "SeedA1", "name": "seeded-app",
    "resources": [{
      "id": "SeedR1", "name": "/ping", "uri": "/ping",
      "methods": [{
        "id": "SeedM1", "name": "ping", "httpMethod": "GET", "status": "MOCKED", "responseStrategy": "RANDOM",
        "mockResponses": [{"id": "SeedX1", "name": "pong", "status": "ENABLED", "httpStatusCode": 200, "body": "pong",
          "httpHeaders": [{"name": "Content-Type", "value": "text/plain"}]}]
      }]
    }]
  }]
}
`

// testServer boots the real wiring (newApp) on dataDir.
type testServer struct {
	*httptest.Server
	app *app
}

func startServer(t *testing.T, env map[string]string) *testServer {
	t.Helper()
	cfg, err := config.Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	a, err := newApp(context.Background(), cfg, logger)
	if err != nil {
		t.Fatalf("newApp: %v", err)
	}
	srv := httptest.NewServer(a.handler)
	ts := &testServer{Server: srv, app: a}
	t.Cleanup(func() {
		srv.Close()
		if err := a.close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	return ts
}

// client is a browser-like client with its own cookie jar.
type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func (s *testServer) client(t *testing.T) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: s.URL, http: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}}
}

type response struct {
	status int
	header http.Header
	body   string
}

func (c *client) do(method, path string, body io.Reader, headers map[string]string) response {
	c.t.Helper()
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		c.t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return response{status: resp.StatusCode, header: resp.Header, body: string(b)}
}

// json sends v as JSON and decodes the answer into out (when not nil),
// failing unless the status is want.
func (c *client) json(method, path string, v any, want int, out any) {
	c.t.Helper()
	var body io.Reader
	if v != nil {
		b, _ := json.Marshal(v)
		body = bytes.NewReader(b)
	}
	r := c.do(method, path, body, map[string]string{"Content-Type": "application/json"})
	if r.status != want {
		c.t.Fatalf("%s %s = %d, want %d: %s", method, path, r.status, want, r.body)
	}
	if out != nil {
		if err := json.Unmarshal([]byte(r.body), out); err != nil {
			c.t.Fatalf("%s %s: decode %s: %v", method, path, r.body, err)
		}
	}
}

func (c *client) login(username, password string) {
	c.t.Helper()
	c.json(http.MethodPost, ctxPath+"/api/rest/core/login", map[string]string{"username": username, "password": password}, 200, nil)
}

func (c *client) upload(path, filename string, data []byte, want int) string {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write(data)
	_ = mw.Close()
	r := c.do(http.MethodPost, path, &buf, map[string]string{"Content-Type": mw.FormDataContentType()})
	if r.status != want {
		c.t.Fatalf("upload %s = %d, want %d: %s", path, r.status, want, r.body)
	}
	return r.body
}

type entity struct {
	ID string `json:"id"`
}

func baseEnv(dataDir string) map[string]string {
	return map[string]string{
		"MOCKINGBIRD_DATA_DIR":       dataDir,
		"MOCKINGBIRD_ADMIN_PASSWORD": adminPassword,
		"MOCKINGBIRD_LOG_LEVEL":      "error",
	}
}

// TestEndToEnd is the SPEC.md §10 rule 4 acceptance test.
func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e")
	}
	dataDir, seedDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(seedDir, "seeded.json"), []byte(seedProject), 0o644); err != nil {
		t.Fatal(err)
	}
	env := baseEnv(dataDir)
	env["MOCKINGBIRD_SEED_DIR"] = seedDir
	srv := startServer(t, env)
	anon := srv.client(t)

	{ // health and redirects
		for _, p := range []string{"/healthz", "/readyz"} {
			if r := anon.do(http.MethodGet, p, nil, nil); r.status != 200 || strings.TrimSpace(r.body) != "ok" {
				t.Errorf("%s = %d %q", p, r.status, r.body)
			}
		}
		if r := anon.do(http.MethodGet, "/", nil, nil); r.status != http.StatusFound || r.header.Get("Location") != ctxPath+"/" {
			t.Errorf("/ = %d %s", r.status, r.header.Get("Location"))
		}
		if r := anon.do(http.MethodGet, ctxPath+"/", nil, nil); r.status != http.StatusFound || r.header.Get("Location") != ctxPath+"/web" {
			t.Errorf("ctx = %d %s", r.status, r.header.Get("Location"))
		}
		if r := anon.do(http.MethodGet, ctxPath+"/web", nil, nil); r.status != http.StatusFound || !strings.Contains(r.header.Get("Location"), "/web/login") {
			t.Errorf("anonymous UI = %d %s", r.status, r.header.Get("Location"))
		}
		if r := anon.do(http.MethodGet, ctxPath+"/web/login", nil, nil); r.status != 200 || !strings.Contains(r.body, "<form") {
			t.Errorf("login page = %d", r.status)
		}
		if r := anon.do(http.MethodGet, ctxPath+"/api/rest/core/project", nil, nil); r.status != http.StatusUnauthorized {
			t.Errorf("anonymous API = %d", r.status)
		}
	}

	{ // seeded mock
		r := anon.do(http.MethodGet, ctxPath+"/mock/rest/project/SeedP1/application/SeedA1/ping", nil, nil)
		if r.status != 200 || r.body != "pong" || r.header.Get("Content-Type") != "text/plain" {
			t.Errorf("seeded mock = %d %q %v", r.status, r.body, r.header)
		}
	}

	admin := srv.client(t)
	admin.login("admin", adminPassword)
	if r := anon.do(http.MethodPost, ctxPath+"/api/rest/core/login", strings.NewReader(`{"username":"admin","password":"bad"}`),
		map[string]string{"Content-Type": "application/json"}); r.status != http.StatusUnauthorized {
		t.Errorf("bad password = %d", r.status)
	}

	var project, application, resource, method, mockResp entity
	{ // create a REST mock through the API and call it
		admin.json(http.MethodPost, ctxPath+"/api/rest/rest/project", map[string]any{"name": "E2E", "description": "created by e2e"}, 200, &project)
		base := ctxPath + "/api/rest/rest/project/" + project.ID
		admin.json(http.MethodPost, base+"/application", map[string]any{"name": "shop"}, 200, &application)
		base += "/application/" + application.ID
		admin.json(http.MethodPost, base+"/resource", map[string]any{"name": "items", "uri": "/items/{id}"}, 200, &resource)
		base += "/resource/" + resource.ID
		admin.json(http.MethodPost, base+"/method", map[string]any{"name": "getItem", "httpMethod": "GET"}, 200, &method)
		base += "/method/" + method.ID
		admin.json(http.MethodPost, base+"/mockresponse", map[string]any{
			"name": "ok", "body": `{"id": "${PATH_PARAMETER(parameter="id")}"}`, "httpStatusCode": 200, "status": "ENABLED",
			"usingExpressions": true, "httpHeaders": []map[string]string{{"name": "Content-Type", "value": "application/json"}},
		}, 200, &mockResp)

		mock := ctxPath + "/mock/rest/project/" + project.ID + "/application/" + application.ID + "/items/42"
		r := anon.do(http.MethodGet, mock, nil, nil)
		if r.status != 200 || r.body != `{"id": "42"}` || r.header.Get("Content-Type") != "application/json" {
			t.Fatalf("mock = %d %q %v", r.status, r.body, r.header)
		}
		if r := anon.do(http.MethodPost, mock, nil, nil); r.status != 500 || !strings.Contains(r.body, `"status":500`) {
			t.Errorf("unknown method = %d %s", r.status, r.body)
		}
		srv.app.restEngine.Wait()
		var events []map[string]any
		admin.json(http.MethodGet, ctxPath+"/api/rest/core/event", nil, 200, &events)
		if len(events) < 2 {
			t.Errorf("events = %d, want the mock calls logged", len(events))
		}
	}

	// One user per role, all through the admin API.
	users := map[string]*client{"ADMIN": admin}
	for _, role := range []string{"MODIFIER", "READER"} {
		name := strings.ToLower(role) + "1"
		admin.json(http.MethodPost, ctxPath+"/api/rest/core/user", map[string]any{
			"username": name, "password": "secret-" + name, "role": role, "status": "ACTIVE", "email": name + "@example.com",
		}, 200, nil)
		c := srv.client(t)
		c.login(name, "secret-"+name)
		users[role] = c
	}

	{ // main UI pages for every role
		restP := ctxPath + "/web/rest/project/" + project.ID
		pages := []struct {
			path      string
			adminOnly bool
		}{
			{ctxPath + "/web", false},
			{restP, false},
			{restP + "/application/" + application.ID, false},
			{restP + "/application/" + application.ID + "/resource/" + resource.ID, false},
			{restP + "/application/" + application.ID + "/resource/" + resource.ID + "/method/" + method.ID, false},
			{restP + "/application/" + application.ID + "/resource/" + resource.ID + "/method/" + method.ID + "/mockresponse/" + mockResp.ID, false},
			{ctxPath + "/web/event", false},
			{ctxPath + "/web/profile", false},
			{ctxPath + "/web/user", true},
			{ctxPath + "/web/system", true},
		}
		for role, c := range users {
			for _, p := range pages {
				r := c.do(http.MethodGet, p.path, nil, nil)
				want := 200
				if p.adminOnly && role != "ADMIN" {
					want = http.StatusForbidden
				}
				if r.status != want {
					t.Errorf("%s %s = %d, want %d", role, p.path, r.status, want)
				}
				if want == 200 && !strings.Contains(r.body, "</html>") {
					t.Errorf("%s %s is not a full page", role, p.path)
				}
			}
		}
		// Role checks in the API.
		users["READER"].json(http.MethodPost, ctxPath+"/api/rest/rest/project", map[string]any{"name": "nope"}, http.StatusForbidden, nil)
		users["MODIFIER"].json(http.MethodGet, ctxPath+"/api/rest/core/user", nil, http.StatusForbidden, nil)
	}

	{ // export and re-import round trip
		for _, format := range []string{"", "?format=json"} {
			r := admin.do(http.MethodGet, ctxPath+"/api/rest/rest/project/"+project.ID+"/export"+format, nil, nil)
			if r.status != 200 || len(r.body) == 0 {
				t.Fatalf("export%s = %d", format, r.status)
			}
			admin.json(http.MethodDelete, ctxPath+"/api/rest/rest/project/"+project.ID, nil, 200, nil)
			mock := ctxPath + "/mock/rest/project/" + project.ID + "/application/" + application.ID + "/items/7"
			if got := anon.do(http.MethodGet, mock, nil, nil); got.status != 500 {
				t.Fatalf("deleted project still serves: %d", got.status)
			}
			admin.upload(ctxPath+"/api/rest/rest/project/import", "export.xml", []byte(r.body), 200)
			if got := anon.do(http.MethodGet, mock, nil, nil); got.status != 200 || got.body != `{"id": "7"}` {
				t.Fatalf("re-imported (%q) mock = %d %q", format, got.status, got.body)
			}
		}
	}
}

// TestEndToEndCastleMockImport imports the CastleMock data dir fixture with
// the CLI and serves its REST and SOAP mocks.
func TestEndToEndCastleMockImport(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e")
	}
	dataDir := t.TempDir()
	env := baseEnv(dataDir)
	var out bytes.Buffer
	fixture := filepath.Join("..", "..", "internal", "importer", "castlemock", "testdata", "castlemock-home")
	if err := run([]string{"import", "--castlemock", fixture}, func(k string) string { return env[k] }, &out); err != nil {
		t.Fatalf("import: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "imported 1 REST projects, 1 SOAP projects, 2 users") {
		t.Errorf("import report = %s", out.String())
	}
	srv := startServer(t, env)
	anon := srv.client(t)

	// Legacy credentials keep working (bcrypt hash imported as is).
	admin := srv.client(t)
	admin.login("admin", "legacy-admin")

	{ // REST mock
		start := time.Now()
		r := anon.do(http.MethodGet, ctxPath+"/mock/rest/project/Pr01ab/application/Ap01cd/pet/1", nil, nil)
		want := "{\n  \"id\": 1,\n  \"name\": \"doggie & co\"\n}"
		if r.status != 200 || r.body != want || r.header.Get("Content-Type") != "application/json" || r.header.Get("Content-Encoding") != "" {
			t.Errorf("pet = %d %q %v", r.status, r.body, r.header)
		}
		if time.Since(start) < 250*time.Millisecond {
			t.Errorf("network delay of the fixture not applied")
		}
		if r := anon.do(http.MethodDelete, ctxPath+"/mock/rest/project/Pr01ab/application/Ap01cd/pet/1", nil, nil); r.status != 500 {
			t.Errorf("disabled method = %d", r.status)
		}
	}

	soapURL := ctxPath + "/mock/soap/project/Sp01ab/CalculatorSoap"
	{ // SOAP mocks and WSDL
		add := `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tem="http://tempuri.org/">
   <soapenv:Body><tem:Add><tem:intA>1</tem:intA><tem:intB>2</tem:intB></tem:Add></soapenv:Body>
</soapenv:Envelope>`
		r := anon.do(http.MethodPost, soapURL, strings.NewReader(add), map[string]string{"Content-Type": "text/xml;charset=UTF-8"})
		want := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\">\n   <soapenv:Body>?</soapenv:Body>\n</soapenv:Envelope>"
		if r.status != 200 || r.body != want || r.header.Get("Content-Type") != `text/xml; charset="utf-8"` {
			t.Errorf("Add = %d %q %v", r.status, r.body, r.header)
		}
		sub := `<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body><SubtractRequest/></soap:Body></soap:Envelope>`
		r = anon.do(http.MethodPost, soapURL, strings.NewReader(sub), map[string]string{"Content-Type": "application/soap+xml"})
		if r.status != 200 || r.body != sub || r.header.Get("Content-Type") != "application/soap+xml" {
			t.Errorf("Subtract (ECHO) = %d %q %v", r.status, r.body, r.header)
		}
		r = anon.do(http.MethodGet, ctxPath+"/mock/soap/project/Sp01ab?wsdl", nil, nil)
		wsdl, _ := os.ReadFile(filepath.Join("..", "..", "internal", "importer", "castlemock", "testdata", "castlemock-home-expected", "projects", "soap", "Sp01ab", "resources", "Rc01wd.wsdl"))
		if r.status != 200 || r.body != string(wsdl) || r.header.Get("Content-Type") != `text/xml; charset="utf-8"` {
			t.Errorf("?wsdl = %d %q", r.status, r.body)
		}
	}

	{ // UI and API over imported data
		for _, p := range []string{
			ctxPath + "/web", ctxPath + "/web/rest/project/Pr01ab", ctxPath + "/web/soap/project/Sp01ab",
			ctxPath + "/web/soap/project/Sp01ab/port/Po01cd", ctxPath + "/web/soap/project/Sp01ab/port/Po01cd/operation/Op01ef",
			ctxPath + "/web/soap/project/Sp01ab/port/Po01cd/operation/Op01ef/mockresponse/Sr01gh",
			ctxPath + "/web/soap/project/Sp01ab/resource/Rc01wd", ctxPath + "/web/event", ctxPath + "/web/user",
			ctxPath + "/web/rest/project/Pr01ab/application/Ap01cd", ctxPath + "/web/rest/project/Pr01ab/application/Ap01cd/resource/Rs01ef",
			ctxPath + "/web/rest/project/Pr01ab/application/Ap01cd/resource/Rs01ef/method/Me01gh",
			ctxPath + "/web/rest/project/Pr01ab/application/Ap01cd/resource/Rs01ef/method/Me01gh/mockresponse/Mr01ij",
			ctxPath + "/web/rest/event/Ev01re", ctxPath + "/web/soap/event/Ev02so", ctxPath + "/web/user/Rd02er",
			ctxPath + "/web/profile", ctxPath + "/web/system",
		} {
			if r := admin.do(http.MethodGet, p, nil, nil); r.status != 200 {
				t.Errorf("%s = %d", p, r.status)
			}
		}
		var projects []map[string]any
		admin.json(http.MethodGet, ctxPath+"/api/rest/core/project", nil, 200, &projects)
		if len(projects) != 2 {
			t.Errorf("projects = %d", len(projects))
		}
		// The LOCKED legacy reader cannot log in.
		if r := anon.do(http.MethodPost, ctxPath+"/api/rest/core/login", strings.NewReader(`{"username":"reader","password":"x"}`),
			map[string]string{"Content-Type": "application/json"}); r.status != http.StatusUnauthorized {
			t.Errorf("locked user login = %d", r.status)
		}
		ui := srv.client(t)
		ui.login("admin", "legacy-admin")
		if r := ui.do(http.MethodGet, ctxPath+"/web/logout", nil, nil); r.status != http.StatusFound && r.status != http.StatusSeeOther {
			t.Errorf("logout = %d", r.status)
		}
		if r := ui.do(http.MethodGet, ctxPath+"/web", nil, nil); r.status != http.StatusFound {
			t.Errorf("after logout /web = %d", r.status)
		}
	}

	{ // SOAP export and re-import round trip
		for _, format := range []string{"", "?format=json"} {
			r := admin.do(http.MethodGet, ctxPath+"/api/rest/soap/project/Sp01ab/export"+format, nil, nil)
			if r.status != 200 {
				t.Fatalf("export%s = %d", format, r.status)
			}
			admin.json(http.MethodDelete, ctxPath+"/api/rest/soap/project/Sp01ab", nil, 200, nil)
			admin.upload(ctxPath+"/api/rest/soap/project/import", "export", []byte(r.body), 200)
			sub := `<Envelope><Body><SubtractRequest/></Body></Envelope>`
			if got := anon.do(http.MethodPost, soapURL, strings.NewReader(sub), map[string]string{"Content-Type": "application/soap+xml"}); got.status != 200 || got.body != sub {
				t.Fatalf("re-imported%s SOAP mock = %d %q", format, got.status, got.body)
			}
			if got := anon.do(http.MethodGet, ctxPath+"/mock/soap/project/Sp01ab?wsdl", nil, nil); got.status != 200 {
				t.Fatalf("re-imported%s ?wsdl = %d", format, got.status)
			}
		}
	}
}
