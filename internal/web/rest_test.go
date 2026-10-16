package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/web/views"
)

func (e *testEnv) restIDs() views.RestIDs {
	return views.RestIDs{Project: e.seed.rest, App: e.seed.app, Resource: e.seed.resource, Method: e.seed.method, Response: e.seed.response}
}

func TestRestPages(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ids := e.restIDs()
	tests := []struct {
		name  string
		path  string
		frags []string
	}{
		{"project", ids.ProjectPath(), []string{"payments-gateway", "payments-v1", "webhooks", "http://mock.test/castlemock/mock/rest/project/" + ids.Project, "Project explorer"}},
		{"application", ids.AppPath(), []string{"payments-v1", "/customers/", "/charges", "/castlemock/mock/rest/project/" + ids.Project + "/application/" + ids.App}},
		{"resource", ids.ResourcePath(), []string{"Get customer", "Delete customer", "http://mock.test/castlemock/mock/rest/project/" + ids.Project + "/application/" + ids.App + "/customers/{id}"}},
		{"method", ids.MethodPath(), []string{"Get customer", "Sequence", "250", "not found", "Default", "curl -i"}},
		{"mock response", ids.ResponsePath(), []string{`name="body"`, "Ada Lovelace", `name="h[0].name"`, `value="Content-Type"`, `name="pq[0].parameter"`, `<option value="id" selected`, "Use expressions"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := e.do(req{path: tt.path, role: domain.RoleReader})
			assertStatus(t, w.Code, http.StatusOK)
			body := w.Body.String()
			assertContains(t, body, tt.frags...)
			for _, hidden := range []string{"/update\"", "/delete\"", "Save response"} {
				if strings.Contains(body, hidden) {
					t.Fatalf("reader sees %q", hidden)
				}
			}
		})
	}
	t.Run("modifier sees actions", func(t *testing.T) {
		w := e.do(req{path: ids.MethodPath(), role: domain.RoleModifier})
		assertContains(t, w.Body.String(), ids.MethodPath()+"/update", "Add response", "Duplicate", `name="ids"`)
		w = e.do(req{path: ids.ResponsePath(), role: domain.RoleModifier})
		assertContains(t, w.Body.String(), "Save response", "tpl-pq")
	})
	t.Run("unknown entity is 404", func(t *testing.T) {
		w := e.do(req{path: views.RestMethodPath(ids.Project, ids.App, ids.Resource, "nope00"), role: domain.RoleReader})
		assertStatus(t, w.Code, http.StatusNotFound)
		assertContains(t, w.Body.String(), "Not found")
	})
	t.Run("tree fragments", func(t *testing.T) {
		w := e.do(req{path: ids.ProjectPath() + "/tree/application/" + ids.App, role: domain.RoleReader, htmx: true})
		assertStatus(t, w.Code, http.StatusOK)
		assertContains(t, w.Body.String(), "/customers/{id}", "/charges")
		w = e.do(req{path: ids.ProjectPath() + "/tree/resource/" + ids.Resource + "?a=" + ids.App, role: domain.RoleReader, htmx: true})
		assertContains(t, w.Body.String(), "Get customer", "badge-method--delete")
	})
	t.Run("export", func(t *testing.T) {
		w := e.do(req{path: ids.ProjectPath() + "/export?format=json", role: domain.RoleReader})
		// The exporter belongs to T1; only check that the route reaches the service.
		if w.Code == http.StatusOK && !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
			t.Fatalf("Content-Disposition = %q", w.Header().Get("Content-Disposition"))
		}
		if w.Code == http.StatusNotFound || w.Code == http.StatusForbidden {
			t.Fatalf("status = %d", w.Code)
		}
	})
}

func TestRestDialogs(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ids := e.restIDs()
	for _, tt := range dialogCases(ids) {
		t.Run(tt.path, func(t *testing.T) {
			w := e.do(req{path: tt.path, role: domain.RoleModifier, htmx: true, target: "modal-root"})
			assertStatus(t, w.Code, http.StatusOK)
			body := w.Body.String()
			assertContains(t, body, append([]string{"<dialog"}, tt.frags...)...)
			if strings.Contains(body, "<html") {
				t.Fatal("htmx dialog rendered a full page")
			}
			w = e.do(req{path: tt.path, role: domain.RoleModifier})
			assertStatus(t, w.Code, http.StatusOK)
			assertContains(t, w.Body.String(), "<html", "modal-page")
		})
	}
	t.Run("reader cannot open modify dialogs", func(t *testing.T) {
		w := e.do(req{path: ids.MethodPath() + "/update", role: domain.RoleReader})
		assertStatus(t, w.Code, http.StatusForbidden)
	})
}

type dialogCase struct {
	path  string
	frags []string
}

func dialogCases(ids views.RestIDs) []dialogCase {
	return []dialogCase{
		{ids.ProjectPath() + "/update", []string{"Update project", `value="payments-gateway"`, "Stripe-like"}},
		{ids.ProjectPath() + "/delete", []string{"Delete project", `data-confirm-name="payments-gateway"`}},
		{ids.ProjectPath() + "/definition?type=RAML", []string{"Import RAML", "multipart/form-data", "Generate responses for each operation"}},
		{ids.ProjectPath() + "/definition?type=SWAGGER&source=url", []string{"Import Swagger V2", `name="url"`}},
		{ids.ProjectPath() + "/application/new", []string{"New application"}},
		{ids.ProjectPath() + "/application/status?ids=" + ids.App, []string{"Update status of applications", "payments-v1", "Record once"}},
		{ids.ProjectPath() + "/application/endpoint?ids=" + ids.App, []string{"Update forwarded endpoint", "forwardedEndpoint"}},
		{ids.ProjectPath() + "/application/delete?ids=" + ids.App, []string{"Delete applications", "payments-v1"}},
		{ids.ProjectPath() + "/application/delete", []string{"Select at least one row"}},
		{ids.AppPath() + "/update", []string{"Update application"}},
		{ids.AppPath() + "/resource/new", []string{"New resource", `name="uri"`}},
		{ids.AppPath() + "/resource/status?ids=" + ids.Resource, []string{"Update status of resources"}},
		{ids.ResourcePath() + "/update", []string{"Update resource", "/customers/{id}"}},
		{ids.ResourcePath() + "/method/new", []string{"New method", `<option value="PATCH"`}},
		{ids.ResourcePath() + "/method/endpoint?ids=" + ids.Method, []string{"Update forwarded endpoint", "GET Get customer"}},
		{ids.MethodPath() + "/update", []string{"Update method", `<option value="SEQUENCE" selected`, `value="250"`, "Automatic forward"}},
		{ids.MethodPath() + "/mockresponse/new", []string{"New mock response", `<option value="ENABLED" selected`}},
		{ids.MethodPath() + "/mockresponse/duplicate?ids=" + ids.Response, []string{"Duplicate mock responses", "customer (200)"}},
		{ids.ResponsePath() + "/delete", []string{"Delete mock response"}},
		{ids.ResponsePath() + "/validate", []string{"Validate expression", `hx-include="#mr-body"`}},
	}
}

func TestRestMutations(t *testing.T) {
	t.Parallel()
	post := func(e *testEnv, path string, form url.Values) *httptestResult {
		w := e.do(req{method: http.MethodPost, path: path, role: domain.RoleModifier, form: form, htmx: true, target: "modal-root"})
		return &httptestResult{code: w.Code, body: w.Body.String(), location: w.Header().Get("HX-Location"), cookie: w.Header().Get("Set-Cookie")}
	}
	ctx := t.Context()

	t.Run("create application, resource, method and response", func(t *testing.T) {
		e := newEnv(t)
		ids := e.restIDs()
		r := post(e, ids.ProjectPath()+"/application/new", url.Values{"name": {"orders"}})
		assertStatus(t, r.code, http.StatusOK)
		if !strings.HasPrefix(r.location, testCtx+ids.ProjectPath()+"/application/") || !strings.Contains(r.cookie, "mb_flash=") {
			t.Fatalf("location %q cookie %q", r.location, r.cookie)
		}
		appID := r.location[strings.LastIndex(r.location, "/")+1:]
		r = post(e, views.RestAppPath(ids.Project, appID)+"/resource/new", url.Values{"name": {"order"}, "uri": {"/orders/{orderId}"}})
		resID := r.location[strings.LastIndex(r.location, "/")+1:]
		r = post(e, views.RestResourcePath(ids.Project, appID, resID)+"/method/new", url.Values{"name": {"Get order"}, "httpMethod": {"GET"}})
		mID := r.location[strings.LastIndex(r.location, "/")+1:]
		r = post(e, views.RestMethodPath(ids.Project, appID, resID, mID)+"/mockresponse/new", url.Values{"name": {"ok"}, "status": {"DISABLED"}})
		mrID := r.location[strings.LastIndex(r.location, "/")+1:]
		mr, err := e.svc.RestMockResponse(ctx, mrID)
		if err != nil {
			t.Fatal(err)
		}
		if mr.Name != "ok" || mr.Status != domain.RestMockResponseDisabled || mr.HTTPStatusCode != 200 {
			t.Fatalf("mock response = %+v", mr)
		}
	})
	t.Run("validation error re-renders the dialog with 422", func(t *testing.T) {
		e := newEnv(t)
		r := post(e, e.restIDs().AppPath()+"/resource/new", url.Values{"name": {""}, "uri": {"/x"}})
		assertStatus(t, r.code, http.StatusUnprocessableEntity)
		assertContains(t, r.body, "<dialog", "alert--error", "New resource")
	})
	t.Run("update method", func(t *testing.T) {
		e := newEnv(t)
		ids := e.restIDs()
		r := post(e, ids.MethodPath()+"/update", url.Values{
			"name": {"Fetch customer"}, "httpMethod": {"PUT"}, "status": {"FORWARDED"}, "responseStrategy": {"QUERY_MATCH"},
			"forwardedEndpoint": {"https://up.example"}, "networkDelay": {"10"}, "automaticForward": {"true"}, "defaultMockResponseId": {""},
		})
		assertStatus(t, r.code, http.StatusOK)
		m, _ := e.svc.RestMethod(ctx, ids.Method)
		if m.Name != "Fetch customer" || m.HTTPMethod != domain.MethodPUT || m.Status != domain.RestMethodForwarded ||
			m.ResponseStrategy != domain.RestStrategyQueryMatch || m.ForwardedEndpoint != "https://up.example" ||
			domain.Deref(m.NetworkDelay) != 10 || !domain.Deref(m.AutomaticForward) || domain.Deref(m.SimulateNetworkDelay) || m.DefaultMockResponseID != "" {
			t.Fatalf("method = %+v", m)
		}
		r = post(e, ids.MethodPath()+"/update", url.Values{"name": {"x"}, "httpMethod": {"GET"}, "status": {"MOCKED"}, "responseStrategy": {"RANDOM"}, "networkDelay": {"-3"}})
		assertStatus(t, r.code, http.StatusUnprocessableEntity)
		assertContains(t, r.body, "network delay")
	})
	t.Run("save mock response replaces lists", func(t *testing.T) {
		e := newEnv(t)
		ids := e.restIDs()
		form := url.Values{
			"name": {"customer v2"}, "httpStatusCode": {"201"}, "body": {`{"ok":true}`}, "usingExpressions": {"true"},
			"h[0].name": {"Content-Type"}, "h[0].value": {"application/json"},
			"h[7].name": {"X-Mock"}, "h[7].value": {"yes"},
			"h[3].name": {""}, "h[3].value": {"dropped"},
			"pq[2].parameter": {"id"}, "pq[2].query": {"1"}, "pq[2].matchRegex": {"true"},
			"hq[0].header": {"X-Tier"}, "hq[0].query": {"vip"}, "hq[0].matchCase": {"true"},
			"jp[0].expression": {"$.id"}, "xp[0].expression": {"//id"}, "xp[1].expression": {"  "},
		}
		w := e.do(req{method: http.MethodPost, path: ids.ResponsePath(), role: domain.RoleModifier, form: form})
		assertStatus(t, w.Code, http.StatusSeeOther)
		mr, _ := e.svc.RestMockResponse(ctx, ids.Response)
		if mr.Name != "customer v2" || mr.HTTPStatusCode != 201 || mr.Body != `{"ok":true}` || mr.Status != domain.RestMockResponseDisabled || !domain.Deref(mr.UsingExpressions) {
			t.Fatalf("response = %+v", mr)
		}
		if len(mr.HTTPHeaders) != 2 || mr.HTTPHeaders[1].Name != "X-Mock" {
			t.Fatalf("headers = %+v", mr.HTTPHeaders)
		}
		if len(mr.ParameterQueries) != 1 || !mr.ParameterQueries[0].MatchRegex || mr.ParameterQueries[0].MatchCase {
			t.Fatalf("parameter queries = %+v", mr.ParameterQueries)
		}
		if len(mr.HeaderQueries) != 1 || !mr.HeaderQueries[0].MatchCase || len(mr.JSONPathExpressions) != 1 || len(mr.XPathExpressions) != 1 {
			t.Fatalf("matchers = %+v %+v %+v", mr.HeaderQueries, mr.JSONPathExpressions, mr.XPathExpressions)
		}
	})
	t.Run("save mock response error keeps the input", func(t *testing.T) {
		e := newEnv(t)
		ids := e.restIDs()
		w := e.do(req{method: http.MethodPost, path: ids.ResponsePath(), role: domain.RoleModifier, form: url.Values{"name": {""}, "body": {"draft body"}, "httpStatusCode": {"200"}}})
		assertStatus(t, w.Code, http.StatusUnprocessableEntity)
		assertContains(t, w.Body.String(), "draft body", "alert--error")
	})
	t.Run("bulk status, endpoint, duplicate and delete", func(t *testing.T) {
		e := newEnv(t)
		ids := e.restIDs()
		r := post(e, ids.ProjectPath()+"/application/status", url.Values{"ids": {ids.App, "unknown"}, "status": {"ECHO"}})
		assertStatus(t, r.code, http.StatusOK)
		m, _ := e.svc.RestMethod(ctx, ids.Method)
		if m.Status != domain.RestMethodEcho {
			t.Fatalf("status = %s", m.Status)
		}
		r = post(e, ids.ProjectPath()+"/application/status", url.Values{"ids": {ids.App}, "status": {"BOGUS"}})
		assertStatus(t, r.code, http.StatusUnprocessableEntity)
		post(e, ids.ResourcePath()+"/method/endpoint", url.Values{"ids": {ids.Method}, "forwardedEndpoint": {"http://fwd"}})
		if m, _ = e.svc.RestMethod(ctx, ids.Method); m.ForwardedEndpoint != "http://fwd" {
			t.Fatalf("endpoint = %q", m.ForwardedEndpoint)
		}
		post(e, ids.MethodPath()+"/mockresponse/duplicate", url.Values{"ids": {ids.Response}})
		post(e, ids.MethodPath()+"/mockresponse/status", url.Values{"ids": {e.seed.response2}, "status": {"DISABLED"}})
		m, _ = e.svc.RestMethod(ctx, ids.Method)
		if len(m.MockResponses) != 3 {
			t.Fatalf("responses after duplicate = %d", len(m.MockResponses))
		}
		r = post(e, ids.MethodPath()+"/mockresponse/delete", url.Values{"ids": {ids.Response}})
		assertStatus(t, r.code, http.StatusOK)
		if _, err := e.svc.RestMockResponse(ctx, ids.Response); err == nil {
			t.Fatal("response not deleted")
		}
	})
	t.Run("delete chain", func(t *testing.T) {
		e := newEnv(t)
		ids := e.restIDs()
		for _, tc := range []struct{ path, back string }{
			{ids.ResponsePath() + "/delete", ids.MethodPath()},
			{ids.MethodPath() + "/delete", ids.ResourcePath()},
			{ids.ResourcePath() + "/delete", ids.AppPath()},
			{ids.AppPath() + "/delete", ids.ProjectPath()},
			{ids.ProjectPath() + "/delete", "/web"},
		} {
			r := post(e, tc.path, url.Values{})
			if r.code != http.StatusOK || r.location != testCtx+tc.back {
				t.Fatalf("%s: %d %q", tc.path, r.code, r.location)
			}
		}
		if _, err := e.svc.RestProject(ctx, ids.Project); err == nil {
			t.Fatal("project not deleted")
		}
	})
	t.Run("update project name conflict", func(t *testing.T) {
		e := newEnv(t)
		ids := e.restIDs()
		post(e, "/web/project/new", url.Values{"name": {"other"}, "type": {"rest"}})
		r := post(e, ids.ProjectPath()+"/update", url.Values{"name": {"OTHER"}})
		assertStatus(t, r.code, http.StatusUnprocessableEntity)
		assertContains(t, r.body, "already exists")
	})
	t.Run("definition import needs a source", func(t *testing.T) {
		e := newEnv(t)
		r := post(e, e.restIDs().ProjectPath()+"/definition", url.Values{"definitionType": {"OPENAPI"}, "source": {"url"}, "url": {""}})
		assertStatus(t, r.code, http.StatusUnprocessableEntity)
		assertContains(t, r.body, "enter the URL")
	})
	t.Run("validate expression is allowed to readers", func(t *testing.T) {
		e := newEnv(t)
		w := e.do(req{method: http.MethodPost, path: e.restIDs().ResponsePath() + "/validate", role: domain.RoleReader, form: url.Values{"requestBody": {"{}"}, "body": {"plain"}}, htmx: true, target: "modal-root"})
		assertStatus(t, w.Code, http.StatusOK)
		assertContains(t, w.Body.String(), "<dialog")
	})
	t.Run("reader cannot mutate", func(t *testing.T) {
		e := newEnv(t)
		w := e.do(req{method: http.MethodPost, path: e.restIDs().ResponsePath(), role: domain.RoleReader, form: url.Values{"name": {"x"}}})
		assertStatus(t, w.Code, http.StatusForbidden)
	})
}

type httptestResult struct {
	code                   int
	body, location, cookie string
}
