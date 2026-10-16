package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/config"
	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/service"
	"github.com/duv0-x/mockingbird/internal/service/servicetest"
)

const testCtx = "/castlemock"

// testEnv is the UI on an in-memory store with one user per role.
type testEnv struct {
	t      testing.TB
	engine *gin.Engine
	store  *servicetest.MemStore
	svc    *service.Service
	auth   *auth.Service
	users  map[domain.Role]domain.User
	seed   seedIDs
}

func newEnv(t testing.TB) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store := servicetest.NewMemStore()
	cfg := config.Config{ContextPath: testCtx, UploadMaxBytes: 1 << 20}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.New(service.Deps{Config: cfg, Store: store, Version: "test", Logger: logger})
	authSvc, err := auth.New(store.Users(), auth.Options{KeyFile: filepath.Join(t.TempDir(), "jwt.key")})
	if err != nil {
		t.Fatal(err)
	}
	e := &testEnv{t: t, store: store, svc: svc, auth: authSvc, users: map[domain.Role]domain.User{}}
	for _, r := range []domain.Role{domain.RoleReader, domain.RoleModifier, domain.RoleAdmin} {
		hash, err := auth.HashPassword("secret")
		if err != nil {
			t.Fatal(err)
		}
		u := domain.User{ID: "u" + strings.ToLower(string(r)), Username: strings.ToLower(string(r)), Password: hash, FullName: "Test " + string(r), Role: r, Status: domain.UserActive, Created: domain.Now(), Updated: domain.Now()}
		if err := store.Users().Save(context.Background(), u); err != nil {
			t.Fatal(err)
		}
		e.users[r] = u
	}
	r := gin.New()
	g := r.Group(testCtx, authSvc.Middleware())
	Register(g, Deps{Config: cfg, Service: svc, Auth: authSvc, Version: "test", Logger: logger})
	e.engine = r
	e.seed = seed(t, svc, store)
	return e
}

type req struct {
	method string
	path   string // relative to the context path
	role   domain.Role
	form   url.Values
	htmx   bool
	target string
	origin string // "" → same origin; "-" → none
}

func (e *testEnv) do(r req) *httptest.ResponseRecorder {
	e.t.Helper()
	if r.method == "" {
		r.method = http.MethodGet
	}
	var body io.Reader
	if r.form != nil {
		body = strings.NewReader(r.form.Encode())
	}
	hr := httptest.NewRequest(r.method, testCtx+r.path, body)
	hr.Host = "mock.test"
	if r.form != nil {
		hr.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	switch r.origin {
	case "":
		hr.Header.Set("Origin", "http://mock.test")
	case "-":
	default:
		hr.Header.Set("Origin", r.origin)
	}
	if r.htmx {
		hr.Header.Set("HX-Request", "true")
		if r.target != "" {
			hr.Header.Set("HX-Target", r.target)
		}
	}
	if r.role != "" {
		tok, err := e.auth.Token(e.users[r.role].ID)
		if err != nil {
			e.t.Fatal(err)
		}
		hr.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok})
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, hr)
	return w
}

// seedIDs are the entities created by seed.
type seedIDs struct {
	rest, app, resource, method, response, response2 string
	soap, soapResponse                               string
}

// Fixed ids of the seeded SOAP project.
const (
	seedPort      = "port01"
	seedOperation = "oper01"
	seedResource  = "wsdl01"
)

const seedWSDL = `<?xml version="1.0"?><definitions name="Account" xmlns="http://schemas.xmlsoap.org/wsdl/"><service name="AccountService"><port name="AccountPort" binding="tns:AccountBinding"/></service></definitions>`

// seed builds a small REST project (through the service) and a SOAP project.
func seed(t testing.TB, svc *service.Service, store *servicetest.MemStore) seedIDs {
	t.Helper()
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var ids seedIDs
	p, err := svc.CreateRestProject(ctx, service.ProjectInput{Name: "payments-gateway", Description: "Stripe-like payments API"})
	must(err)
	ids.rest = p.ID
	app, err := svc.CreateRestApplication(ctx, p.ID, "payments-v1")
	must(err)
	ids.app = app.ID
	_, err = svc.CreateRestApplication(ctx, p.ID, "webhooks")
	must(err)
	res, err := svc.CreateRestResource(ctx, app.ID, "customer", "/customers/{id}")
	must(err)
	ids.resource = res.ID
	res2, err := svc.CreateRestResource(ctx, app.ID, "charges", "/charges")
	must(err)
	m, err := svc.CreateRestMethod(ctx, res.ID, service.RestMethodCreate{Name: "Get customer", HTTPMethod: domain.MethodGET})
	must(err)
	ids.method = m.ID
	_, err = svc.CreateRestMethod(ctx, res.ID, service.RestMethodCreate{Name: "Delete customer", HTTPMethod: domain.MethodDELETE})
	must(err)
	post, err := svc.CreateRestMethod(ctx, res2.ID, service.RestMethodCreate{Name: "Create charge", HTTPMethod: domain.MethodPOST})
	must(err)
	_, err = svc.UpdateRestMethod(ctx, post.ID, service.RestMethodUpdate{Name: "Create charge", HTTPMethod: domain.MethodPOST, Status: domain.RestMethodForwarded, ResponseStrategy: domain.RestStrategyRandom, ForwardedEndpoint: domain.Ptr("https://api.example.com")})
	must(err)
	body := "{\n  \"id\": \"${PATH_PARAMETER(parameter=\\\"id\\\")}\",\n  \"name\": \"Ada Lovelace\",\n  \"tier\": \"vip\"\n}"
	mr, err := svc.CreateRestMockResponse(ctx, m.ID, service.RestMockResponseInput{
		Name: "customer", Body: &body, HTTPStatusCode: domain.Ptr(200), UsingExpressions: domain.Ptr(true),
		HTTPHeaders:      []domain.HTTPHeader{{Name: "Content-Type", Value: "application/json"}},
		ParameterQueries: []domain.RestParameterQuery{{Parameter: "id", Query: "42", MatchCase: true}},
	})
	must(err)
	ids.response = mr.ID
	mr2, err := svc.CreateRestMockResponse(ctx, m.ID, service.RestMockResponseInput{Name: "not found", HTTPStatusCode: domain.Ptr(404)})
	must(err)
	ids.response2 = mr2.ID
	_, err = svc.UpdateRestMethod(ctx, m.ID, service.RestMethodUpdate{Name: "Get customer", HTTPMethod: domain.MethodGET, Status: domain.RestMethodMocked, ResponseStrategy: domain.RestStrategySequence, DefaultMockResponseID: domain.Ptr(mr.ID), SimulateNetworkDelay: domain.Ptr(true), NetworkDelay: domain.Ptr(int64(250))})
	must(err)

	sp, err := svc.CreateSoapProject(ctx, service.ProjectInput{Name: "core-banking", Description: "Core banking SOAP services"})
	must(err)
	ids.soap = sp.ID
	_, err = store.Soap().Update(ctx, sp.ID, func(p *domain.SoapProject) error {
		p.Ports = append(p.Ports, domain.SoapPort{ID: seedPort, Name: "AccountPort", URI: "AccountPort", ProjectID: sp.ID,
			Operations: []domain.SoapOperation{{ID: seedOperation, Name: "GetAccount", PortID: seedPort, Status: domain.SoapOperationForwarded,
				ResponseStrategy: domain.SoapStrategyXPathInput, HTTPMethod: domain.MethodPOST, SoapVersion: domain.SOAP11,
				IdentifyStrategy: domain.IdentifyByElementNamespace, ForwardedEndpoint: "https://core.example/account",
				OriginalEndpoint:    "http://bank.example/AccountService",
				OperationIdentifier: domain.SoapOperationIdentifier{Name: "GetAccountRequest", Namespace: "http://bank.example/account"},
				DefaultBody:         "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body><GetAccountResponse/></soapenv:Body></soapenv:Envelope>"}}})
		p.Resources = append(p.Resources, domain.SoapResource{ID: seedResource, Name: "AccountService.wsdl", ProjectID: sp.ID, Type: domain.SoapResourceWSDL})
		return nil
	})
	must(err)
	must(store.Soap().PutResourceContent(ctx, sp.ID, seedResource, seedWSDL))
	smr, err := svc.CreateSoapMockResponse(ctx, seedOperation, service.SoapMockResponseInput{Name: "account-ok", Body: "<ok/>", HTTPStatusCode: 200,
		XPathExpressions: []domain.SoapXPathExpression{{Expression: "//accountId[text()='1001']"}}})
	must(err)
	ids.soapResponse = smr.ID
	now := time.Now().UTC()
	must(store.Events().AddRest(ctx, domain.RestEvent{Event: domain.Event{ID: "evt001", ResourceName: "Get customer", StartDate: domain.Time{Time: now.Add(-time.Minute)}, EndDate: domain.Time{Time: now}},
		Request:   domain.RestRequest{URI: "/customers/42", HTTPMethod: domain.MethodGET, HTTPParameters: []domain.HTTPParameter{{Name: "expand", Value: "true"}}},
		Response:  &domain.RestResponse{Body: `{"id":42}`, MockResponseName: "customer", HTTPStatusCode: 200, ContentType: "application/json"},
		ProjectID: p.ID, ApplicationID: app.ID, ResourceID: res.ID, MethodID: m.ID}))
	must(store.Events().AddSoap(ctx, domain.SoapEvent{Event: domain.Event{ID: "evt002", ResourceName: "GetAccount", StartDate: domain.Time{Time: now.Add(-2 * time.Minute)}, EndDate: domain.Time{Time: now.Add(-2*time.Minute + 14*time.Millisecond)}},
		Request: domain.SoapRequest{Body: "<GetAccountRequest/>", URI: "/castlemock/mock/soap/project/" + sp.ID + "/AccountPort", HTTPMethod: domain.MethodPOST, OperationName: "GetAccount", SoapVersion: domain.SOAP11,
			OperationIdentifier: domain.SoapOperationIdentifier{Name: "GetAccountRequest", Namespace: "http://bank.example/account"}},
		ProjectID: sp.ID, PortID: seedPort, OperationID: seedOperation}))
	return ids
}

// TestDevServer serves the UI on a seeded in-memory store for manual and
// screenshot checks: MOCKINGBIRD_UI_DEV=127.0.0.1:8089 go test ./internal/web -run TestDevServer
// Sign in as admin/modifier/reader with password "secret".
func TestDevServer(t *testing.T) {
	addr := os.Getenv("MOCKINGBIRD_UI_DEV")
	if addr == "" {
		t.Skip("set MOCKINGBIRD_UI_DEV=host:port to run the UI dev server")
	}
	e := newEnv(t)
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Screenshots without a browser session: ?as=<role> signs in.
		if as := r.URL.Query().Get("as"); as != "" {
			if u, ok := e.users[domain.Role(strings.ToUpper(as))]; ok {
				tok, _ := e.auth.Token(u.ID)
				r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok})
			}
		}
		e.engine.ServeHTTP(w, r)
	})
	t.Logf("UI on http://%s%s/web (rest project %s)", addr, testCtx, e.seed.rest)
	if err := http.ListenAndServe(addr, root); err != nil {
		t.Fatal(err)
	}
}
