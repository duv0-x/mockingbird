package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/web/views"
)

func (e *testEnv) soapIDs() views.SoapIDs {
	return views.SoapIDs{Project: e.seed.soap, Port: seedPort, Operation: seedOperation, Response: e.seed.soapResponse, Resource: seedResource}
}

func TestSoapPages(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ids := e.soapIDs()
	tests := []struct {
		name  string
		path  string
		frags []string
	}{
		{"project", ids.ProjectPath(), []string{"core-banking", "AccountPort", "AccountService.wsdl", "http://mock.test/castlemock/mock/soap/project/" + ids.Project, "1 forwarded"}},
		{"port", ids.PortPath(), []string{"GetAccount", "SOAP 1.1", "Xpath input", "http://mock.test/castlemock/mock/soap/project/" + ids.Project + "/AccountPort?wsdl"}},
		{"operation", ids.OperationPath(), []string{"GetAccountRequest", "http://bank.example/account", "account-ok", "//accountId[text()=&#39;1001&#39;]", "Default response body", "https://core.example/account"}},
		{"mock response", ids.ResponsePath(), []string{`name="body"`, "&lt;ok/&gt;", `name="xp[0].expression"`, "Use expressions"}},
		{"resource", ids.ResourcePath(), []string{"AccountService.wsdl", "WSDL", "&lt;service name=&#34;AccountService&#34;&gt;", "/castlemock/resource/soap/project/" + ids.Project + "/resource/" + ids.Resource}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := e.do(req{path: tt.path, role: domain.RoleReader})
			assertStatus(t, w.Code, http.StatusOK)
			body := w.Body.String()
			assertContains(t, body, tt.frags...)
			for _, hidden := range []string{"/update\"", "/delete\"", "Save response", "Import WSDL"} {
				if strings.Contains(body, hidden) {
					t.Fatalf("reader sees %q", hidden)
				}
			}
		})
	}
	t.Run("tree fragment", func(t *testing.T) {
		w := e.do(req{path: ids.ProjectPath() + "/tree/port/" + ids.Port, role: domain.RoleReader, htmx: true})
		assertStatus(t, w.Code, http.StatusOK)
		assertContains(t, w.Body.String(), "GetAccount", "dot--forwarded")
	})
	t.Run("unknown port is 404", func(t *testing.T) {
		w := e.do(req{path: views.SoapPortPath(ids.Project, "nope00"), role: domain.RoleReader})
		assertStatus(t, w.Code, http.StatusNotFound)
	})
	t.Run("export reaches the service", func(t *testing.T) {
		w := e.do(req{path: ids.ProjectPath() + "/export", role: domain.RoleReader})
		if w.Code == http.StatusNotFound || w.Code == http.StatusForbidden {
			t.Fatalf("status = %d", w.Code)
		}
	})
}

func TestSoapDialogs(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ids := e.soapIDs()
	tests := []dialogCase{
		{ids.ProjectPath() + "/update", []string{"Update project", `value="core-banking"`}},
		{ids.ProjectPath() + "/delete", []string{"Delete project", `data-confirm-name="core-banking"`}},
		{ids.ProjectPath() + "/wsdl", []string{"Import WSDL", "multipart/form-data", "Generate responses for each operation"}},
		{ids.ProjectPath() + "/wsdl?source=url", []string{"Import linked WSDLs", `name="url"`}},
		{ids.ProjectPath() + "/port/status?ids=" + ids.Port, []string{"Update status of ports", "AccountPort"}},
		{ids.ProjectPath() + "/port/endpoint?ids=" + ids.Port, []string{"Update forwarded endpoint"}},
		{ids.ProjectPath() + "/port/delete?ids=" + ids.Port, []string{"Delete ports", "AccountPort"}},
		{ids.PortPath() + "/update", []string{"Update port", `value="AccountPort"`}},
		{ids.PortPath() + "/delete", []string{"Delete port"}},
		{ids.PortPath() + "/operation/status?ids=" + ids.Operation, []string{"Update status of operations", "GetAccount"}},
		{ids.OperationPath() + "/update", []string{"Update operation", `<option value="XPATH_INPUT" selected`, `<option value="ELEMENT_NAMESPACE" selected`, "Mock on failure"}},
		{ids.OperationPath() + "/mockresponse/new", []string{"New mock response", "default response body"}},
		{ids.OperationPath() + "/mockresponse/duplicate?ids=" + ids.Response, []string{"Duplicate mock responses", "account-ok (200)"}},
		{ids.ResponsePath() + "/delete", []string{"Delete mock response"}},
		{ids.ResponsePath() + "/validate", []string{"Validate expression"}},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := e.do(req{path: tt.path, role: domain.RoleModifier, htmx: true, target: "modal-root"})
			assertStatus(t, w.Code, http.StatusOK)
			assertContains(t, w.Body.String(), append([]string{"<dialog"}, tt.frags...)...)
			w = e.do(req{path: tt.path, role: domain.RoleModifier})
			assertStatus(t, w.Code, http.StatusOK)
			assertContains(t, w.Body.String(), "<html", "modal-page")
		})
	}
	t.Run("operations have no bulk delete", func(t *testing.T) {
		w := e.do(req{path: ids.PortPath() + "/operation/delete?ids=" + ids.Operation, role: domain.RoleModifier})
		assertStatus(t, w.Code, http.StatusNotFound)
	})
}

func TestSoapMutations(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	post := func(e *testEnv, path string, form url.Values) *httptestResult {
		w := e.do(req{method: http.MethodPost, path: path, role: domain.RoleModifier, form: form, htmx: true, target: "modal-root"})
		return &httptestResult{code: w.Code, body: w.Body.String(), location: w.Header().Get("HX-Location")}
	}

	t.Run("update operation", func(t *testing.T) {
		e := newEnv(t)
		ids := e.soapIDs()
		r := post(e, ids.OperationPath()+"/update", url.Values{
			"status": {"MOCKED"}, "responseStrategy": {"SEQUENCE"}, "identifyStrategy": {"ELEMENT"},
			"forwardedEndpoint": {""}, "automaticForward": {"true"}, "mockOnFailure": {"true"}, "networkDelay": {"30"},
			"simulateNetworkDelay": {"true"}, "defaultMockResponseId": {ids.Response},
		})
		assertStatus(t, r.code, http.StatusOK)
		o, _ := e.svc.SoapOperation(ctx, ids.Operation)
		if o.Status != domain.SoapOperationMocked || o.ResponseStrategy != domain.SoapStrategySequence || o.IdentifyStrategy != domain.IdentifyByElement ||
			o.ForwardedEndpoint != "" || domain.Deref(o.AutomaticForward) || !domain.Deref(o.MockOnFailure) || domain.Deref(o.NetworkDelay) != 30 ||
			o.DefaultMockResponseID != ids.Response {
			t.Fatalf("operation = %+v", o)
		}
		r = post(e, ids.OperationPath()+"/update", url.Values{"status": {"NOPE"}, "responseStrategy": {"RANDOM"}, "identifyStrategy": {"ELEMENT"}})
		assertStatus(t, r.code, http.StatusUnprocessableEntity)
		assertContains(t, r.body, "alert--error")
	})
	t.Run("create response starts with the default body", func(t *testing.T) {
		e := newEnv(t)
		ids := e.soapIDs()
		r := post(e, ids.OperationPath()+"/mockresponse/new", url.Values{"name": {"fault"}, "status": {"DISABLED"}})
		assertStatus(t, r.code, http.StatusOK)
		mrID := r.location[strings.LastIndex(r.location, "/")+1:]
		mr, err := e.svc.SoapMockResponse(ctx, mrID)
		if err != nil {
			t.Fatal(err)
		}
		if mr.Status != domain.SoapMockResponseDisabled || mr.HTTPStatusCode != 200 || !strings.Contains(mr.Body, "GetAccountResponse") {
			t.Fatalf("response = %+v", mr)
		}
	})
	t.Run("save response", func(t *testing.T) {
		e := newEnv(t)
		ids := e.soapIDs()
		w := e.do(req{method: http.MethodPost, path: ids.ResponsePath(), role: domain.RoleModifier, form: url.Values{
			"name": {"account-500"}, "httpStatusCode": {"500"}, "body": {"<fault/>"}, "enabled": {"true"},
			"h[0].name": {"X-Mock"}, "h[0].value": {"1"}, "xp[4].expression": {"//id"},
		}})
		assertStatus(t, w.Code, http.StatusSeeOther)
		mr, _ := e.svc.SoapMockResponse(ctx, ids.Response)
		if mr.Name != "account-500" || mr.HTTPStatusCode != 500 || mr.Body != "<fault/>" || mr.Status != domain.SoapMockResponseEnabled ||
			len(mr.HTTPHeaders) != 1 || len(mr.XPathExpressions) != 1 || mr.XPathExpressions[0].Expression != "//id" {
			t.Fatalf("response = %+v", mr)
		}
		w = e.do(req{method: http.MethodPost, path: ids.ResponsePath(), role: domain.RoleModifier, form: url.Values{"name": {""}, "body": {"<draft/>"}}})
		assertStatus(t, w.Code, http.StatusUnprocessableEntity)
		assertContains(t, w.Body.String(), "&lt;draft/&gt;", "alert--error")
	})
	t.Run("update port uri", func(t *testing.T) {
		e := newEnv(t)
		ids := e.soapIDs()
		r := post(e, ids.PortPath()+"/update", url.Values{"uri": {"AccountPortV2"}})
		assertStatus(t, r.code, http.StatusOK)
		po, _ := e.svc.SoapPort(ctx, ids.Port)
		if po.URI != "AccountPortV2" {
			t.Fatalf("uri = %q", po.URI)
		}
	})
	t.Run("bulk on ports, operations and responses", func(t *testing.T) {
		e := newEnv(t)
		ids := e.soapIDs()
		post(e, ids.ProjectPath()+"/port/status", url.Values{"ids": {ids.Port}, "status": {"ECHO"}})
		if o, _ := e.svc.SoapOperation(ctx, ids.Operation); o.Status != domain.SoapOperationEcho {
			t.Fatalf("status = %s", o.Status)
		}
		post(e, ids.PortPath()+"/operation/endpoint", url.Values{"ids": {ids.Operation}, "forwardedEndpoint": {"http://fwd"}})
		if o, _ := e.svc.SoapOperation(ctx, ids.Operation); o.ForwardedEndpoint != "http://fwd" {
			t.Fatalf("endpoint = %q", o.ForwardedEndpoint)
		}
		post(e, ids.OperationPath()+"/mockresponse/duplicate", url.Values{"ids": {ids.Response}})
		if o, _ := e.svc.SoapOperation(ctx, ids.Operation); len(o.MockResponses) != 2 {
			t.Fatalf("responses = %d", len(o.MockResponses))
		}
		r := post(e, ids.ProjectPath()+"/port/delete", url.Values{"ids": {ids.Port}})
		assertStatus(t, r.code, http.StatusOK)
		if _, err := e.svc.SoapPort(ctx, ids.Port); err == nil {
			t.Fatal("port not deleted")
		}
	})
	t.Run("wsdl import needs a source", func(t *testing.T) {
		e := newEnv(t)
		r := post(e, e.soapIDs().ProjectPath()+"/wsdl", url.Values{"source": {"url"}, "url": {""}})
		assertStatus(t, r.code, http.StatusUnprocessableEntity)
		assertContains(t, r.body, "enter the URL")
	})
	t.Run("delete chain", func(t *testing.T) {
		e := newEnv(t)
		ids := e.soapIDs()
		for _, tc := range []struct{ path, back string }{
			{ids.ResponsePath() + "/delete", ids.OperationPath()},
			{ids.PortPath() + "/delete", ids.ProjectPath()},
			{ids.ProjectPath() + "/delete", "/web"},
		} {
			r := post(e, tc.path, url.Values{})
			if r.code != http.StatusOK || r.location != testCtx+tc.back {
				t.Fatalf("%s: %d %q", tc.path, r.code, r.location)
			}
		}
	})
	t.Run("reader cannot mutate", func(t *testing.T) {
		e := newEnv(t)
		w := e.do(req{method: http.MethodPost, path: e.soapIDs().OperationPath() + "/update", role: domain.RoleReader, form: url.Values{}})
		assertStatus(t, w.Code, http.StatusForbidden)
	})
}

func TestPrettyXML(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"<a><b>x</b></a>", "<a>\n  <b>x</b>\n</a>"},
		{"not xml <", "not xml <"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := prettyXML(tt.in); got != tt.want {
			t.Errorf("prettyXML(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
