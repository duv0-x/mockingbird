package service_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/config"
	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/service"
	"github.com/duv0-x/mockingbird/internal/service/servicetest"
	"github.com/duv0-x/mockingbird/internal/storage"
)

func newService(t *testing.T) (*service.Service, *servicetest.MemStore) {
	t.Helper()
	store := servicetest.NewMemStore()
	svc := service.New(service.Deps{
		Config:  config.Default(),
		Store:   store,
		Version: "test",
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return svc, store
}

func isValidation(err error) bool {
	var ve *service.ValidationError
	return errors.As(err, &ve)
}

// must panics on error; the test fails with the error message.
func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// restTree creates project → application → resource → method → 2 mock responses.
func restTree(t *testing.T, svc *service.Service) (domain.RestProject, domain.RestMethod, []domain.RestMockResponse) {
	t.Helper()
	ctx := context.Background()
	p := must(svc.CreateRestProject(ctx, service.ProjectInput{Name: "P"}))
	a := must(svc.CreateRestApplication(ctx, p.ID, "app"))
	r := must(svc.CreateRestResource(ctx, a.ID, "res", "/a/{x}/b/{y}"))
	m := must(svc.CreateRestMethod(ctx, r.ID, service.RestMethodCreate{Name: "get", HTTPMethod: "get"}))
	enc := []domain.ContentEncoding{domain.EncodingGZIP}
	r1 := must(svc.CreateRestMockResponse(ctx, m.ID, service.RestMockResponseInput{Name: "one", ContentEncodings: enc}))
	r2 := must(svc.CreateRestMockResponse(ctx, m.ID, service.RestMockResponseInput{Name: "two", HTTPStatusCode: domain.Ptr(404)}))
	return p, m, []domain.RestMockResponse{r1, r2}
}

func TestRestDefaultsAndComputedFields(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	p, m, rs := restTree(t, svc)
	if m.HTTPMethod != domain.MethodGET || m.Status != domain.RestMethodMocked || m.ResponseStrategy != domain.RestStrategyRandom || m.URI != "/a/{x}/b/{y}" {
		t.Fatalf("method defaults %+v", m)
	}
	if rs[0].HTTPStatusCode != 200 || rs[0].Status != domain.RestMockResponseEnabled || rs[0].UsingExpressions == nil || *rs[0].UsingExpressions {
		t.Fatalf("mock response defaults %+v", rs[0])
	}
	if rs[1].HTTPStatusCode != 404 {
		t.Fatalf("status code %d", rs[1].HTTPStatusCode)
	}
	got := must(svc.RestProject(ctx, p.ID))
	if n := got.Applications[0].StatusCount[domain.RestMethodMocked]; n != 1 || len(got.Applications[0].StatusCount) != 6 {
		t.Fatalf("statusCount %v", got.Applications[0].StatusCount)
	}
	params := must(svc.RestResourceParameters(ctx, m.ResourceID))
	if len(params) != 2 || params[0] != "x" || params[1] != "y" {
		t.Fatalf("parameters %v", params)
	}
}

func TestUpdateRestMethodUnsetsAbsentOptionals(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	_, m, rs := restTree(t, svc)
	full := service.RestMethodUpdate{Name: "n", HTTPMethod: "POST", Status: domain.RestMethodForwarded, ResponseStrategy: domain.RestStrategySequence,
		ForwardedEndpoint: domain.Ptr("http://x"), SimulateNetworkDelay: domain.Ptr(true), NetworkDelay: domain.Ptr(int64(10)),
		DefaultMockResponseID: domain.Ptr(rs[1].ID), AutomaticForward: domain.Ptr(true)}
	got := must(svc.UpdateRestMethod(ctx, m.ID, full))
	if got.DefaultResponseName != "two" || got.ForwardedEndpoint != "http://x" || !*got.SimulateNetworkDelay {
		t.Fatalf("update %+v", got)
	}
	got = must(svc.UpdateRestMethod(ctx, m.ID, service.RestMethodUpdate{Name: "n", HTTPMethod: "POST", Status: domain.RestMethodMocked, ResponseStrategy: domain.RestStrategyRandom}))
	if got.ForwardedEndpoint != "" || got.SimulateNetworkDelay != nil || got.NetworkDelay != nil || got.DefaultMockResponseID != "" || got.AutomaticForward != nil {
		t.Fatalf("absent optionals not unset: %+v", got)
	}
}

func TestUpdateRestMockResponseKeepsContentEncodings(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	_, _, rs := restTree(t, svc)
	got := must(svc.UpdateRestMockResponse(ctx, rs[0].ID, service.RestMockResponseInput{
		Name: "renamed", ContentEncodings: []domain.ContentEncoding{domain.EncodingDEFLATE},
		HTTPHeaders: []domain.HTTPHeader{{Name: "Content-Type", Value: "application/json"}},
	}))
	if got.Name != "renamed" || len(got.ContentEncodings) != 1 || got.ContentEncodings[0] != domain.EncodingGZIP || len(got.HTTPHeaders) != 1 {
		t.Fatalf("update %+v", got)
	}
	if got.HTTPStatusCode != 200 || got.Status != domain.RestMockResponseEnabled {
		t.Fatalf("absent status/code should keep stored values: %+v", got)
	}
}

func TestDuplicateRestMockResponses(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	_, m, rs := restTree(t, svc)
	if err := svc.DuplicateRestMockResponses(ctx, []string{rs[0].ID, "missing", rs[1].ID}); err != nil {
		t.Fatal(err)
	}
	got := must(svc.RestMethod(ctx, m.ID))
	if len(got.MockResponses) != 4 {
		t.Fatalf("got %d responses", len(got.MockResponses))
	}
	seen := map[string]bool{}
	for _, mr := range got.MockResponses {
		if seen[mr.ID] {
			t.Fatalf("duplicate id %s", mr.ID)
		}
		seen[mr.ID] = true
	}
	if got.MockResponses[2].Name != "one" || got.MockResponses[3].Name != "two" {
		t.Fatalf("names %q %q", got.MockResponses[2].Name, got.MockResponses[3].Name)
	}
}

func TestDeleteCascadesSequenceState(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name   string
		delete func(svc *service.Service, p domain.RestProject, m domain.RestMethod) error
	}{
		{"method", func(svc *service.Service, _ domain.RestProject, m domain.RestMethod) error {
			_, err := svc.DeleteRestMethod(ctx, m.ID)
			return err
		}},
		{"resource", func(svc *service.Service, _ domain.RestProject, m domain.RestMethod) error {
			_, err := svc.DeleteRestResource(ctx, m.ResourceID)
			return err
		}},
		{"application", func(svc *service.Service, p domain.RestProject, _ domain.RestMethod) error {
			_, err := svc.DeleteRestApplication(ctx, must(svc.RestProject(ctx, p.ID)).Applications[0].ID)
			return err
		}},
		{"project", func(svc *service.Service, p domain.RestProject, _ domain.RestMethod) error {
			_, err := svc.DeleteRestProject(ctx, p.ID)
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, store := newService(t)
			p, m, _ := restTree(t, svc)
			_ = store.State().SetSequenceIndex(ctx, m.ID, 3)
			if got := must(svc.RestMethod(ctx, m.ID)); got.CurrentResponseSequenceIndex != 3 {
				t.Fatalf("sequence index %d", got.CurrentResponseSequenceIndex)
			}
			if err := tt.delete(svc, p, m); err != nil {
				t.Fatal(err)
			}
			if _, ok := store.SequenceIndexes()[m.ID]; ok {
				t.Fatal("sequence index not deleted")
			}
			if _, err := svc.RestMethod(ctx, m.ID); !errors.Is(err, storage.ErrNotFound) {
				t.Fatalf("method still readable: %v", err)
			}
		})
	}
}

func TestValidationErrors(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	p, m, rs := restTree(t, svc)
	tests := []struct {
		name string
		err  error
	}{
		{"empty project name", func() error { _, err := svc.CreateRestProject(ctx, service.ProjectInput{}); return err }()},
		{"bad http method", func() error {
			_, err := svc.CreateRestMethod(ctx, m.ResourceID, service.RestMethodCreate{Name: "x", HTTPMethod: "BREW"})
			return err
		}()},
		{"bad status", func() error { return svc.SetRestMethodsStatus(ctx, []string{m.ID}, "SLEEPING") }()},
		{"bad mock status", func() error {
			return svc.SetRestMockResponsesStatus(ctx, []string{rs[0].ID}, "MAYBE")
		}()},
		{"bad definition type", func() error {
			return svc.ImportRestDefinition(ctx, p.ID, "PDF", service.Upload{Data: []byte("x")}, false)
		}()},
		{"unknown project file", func() error {
			_, err := svc.ImportRestProject(ctx, service.Upload{Data: []byte("hello")})
			return err
		}()},
		{"bad export format", func() error { _, err := svc.ExportRestProject(ctx, p.ID, "yaml"); return err }()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !isValidation(tt.err) {
				t.Fatalf("want ValidationError, got %v", tt.err)
			}
		})
	}
}

func TestProjectNameRules(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	a := must(svc.CreateRestProject(ctx, service.ProjectInput{Name: "Alpha"}))
	// REST creation does not check names (CastleMock); updates do.
	must(svc.CreateRestProject(ctx, service.ProjectInput{Name: "alpha"}))
	b := must(svc.CreateRestProject(ctx, service.ProjectInput{Name: "Beta"}))
	if _, err := svc.UpdateRestProject(ctx, b.ID, service.ProjectInput{Name: "ALPHA"}); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	got := must(svc.UpdateRestProject(ctx, a.ID, service.ProjectInput{Name: "Alpha", Description: "d"}))
	if got.Description != "d" || got.Updated.Before(a.Updated.Truncate(time.Millisecond)) {
		t.Fatalf("update %+v", got)
	}
	must(svc.CreateSoapProject(ctx, service.ProjectInput{Name: "S"}))
	if _, err := svc.CreateSoapProject(ctx, service.ProjectInput{Name: "s"}); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("soap create: want conflict, got %v", err)
	}
	projects := must(svc.Projects(ctx))
	if len(projects) != 4 || projects[3].Type != domain.ProjectTypeSOAP || projects[0].Type != domain.ProjectTypeREST {
		t.Fatalf("projects %+v", projects)
	}
}

func soapTree(t *testing.T, svc *service.Service, store *servicetest.MemStore) (domain.SoapProject, string) {
	t.Helper()
	ctx := context.Background()
	p := must(svc.CreateSoapProject(ctx, service.ProjectInput{Name: "Calc"}))
	_, err := store.Soap().Update(ctx, p.ID, func(sp *domain.SoapProject) error {
		sp.Ports = []domain.SoapPort{{ID: "Po0001", Name: "Port", URI: "Port", Operations: []domain.SoapOperation{{
			ID: "Op0001", Name: "Add", Status: domain.SoapOperationMocked, ResponseStrategy: domain.SoapStrategyRandom,
			IdentifyStrategy: domain.IdentifyByElementNamespace, HTTPMethod: domain.MethodPOST, SoapVersion: domain.SOAP11,
		}}}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return p, "Op0001"
}

func TestSoapOperationAndMockResponses(t *testing.T) {
	svc, store := newService(t)
	ctx := context.Background()
	_, opID := soapTree(t, svc, store)
	mr := must(svc.CreateSoapMockResponse(ctx, opID, service.SoapMockResponseInput{Name: "r", Body: "<b/>"}))
	if mr.Status != domain.SoapMockResponseEnabled || mr.HTTPStatusCode != 200 || len(mr.ContentEncodings) != 0 {
		t.Fatalf("defaults %+v", mr)
	}
	o := must(svc.UpdateSoapOperation(ctx, opID, service.SoapOperationUpdate{
		Status: domain.SoapOperationForwarded, ResponseStrategy: domain.SoapStrategyXPathInput, IdentifyStrategy: domain.IdentifyByElement,
		DefaultMockResponseID: domain.Ptr(mr.ID), NetworkDelay: domain.Ptr(int64(5)),
	}))
	if o.SimulateNetworkDelay == nil || *o.SimulateNetworkDelay || o.AutomaticForward == nil || o.MockOnFailure == nil {
		t.Fatalf("absent booleans must become false: %+v", o)
	}
	if o.DefaultResponseName != "r" || *o.NetworkDelay != 5 {
		t.Fatalf("operation %+v", o)
	}
	if _, err := svc.UpdateSoapOperation(ctx, opID, service.SoapOperationUpdate{Status: "MOCKED", ResponseStrategy: "QUERY_MATCH", IdentifyStrategy: "ELEMENT"}); !isValidation(err) {
		t.Fatalf("REST-only strategy accepted: %v", err)
	}
	if err := svc.DuplicateSoapMockResponses(ctx, []string{mr.ID}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetSoapMockResponsesStatus(ctx, []string{mr.ID}, domain.SoapMockResponseDisabled); err != nil {
		t.Fatal(err)
	}
	got := must(svc.SoapOperation(ctx, opID))
	if len(got.MockResponses) != 2 || got.MockResponses[0].Status != domain.SoapMockResponseDisabled || got.MockResponses[1].Status != domain.SoapMockResponseEnabled {
		t.Fatalf("responses %+v", got.MockResponses)
	}
	port := must(svc.SoapPort(ctx, "Po0001"))
	if port.StatusCount[domain.SoapOperationForwarded] != 1 || len(port.Operations[0].MockResponses) != 0 {
		t.Fatalf("port view %+v", port)
	}
	_ = store.State().SetSequenceIndex(ctx, opID, 2)
	must(svc.DeleteSoapPort(ctx, "Po0001"))
	if _, ok := store.SequenceIndexes()[opID]; ok {
		t.Fatal("sequence index not deleted with the port")
	}
}

func TestImportSoapResourceReplacesWSDL(t *testing.T) {
	svc, store := newService(t)
	ctx := context.Background()
	p, _ := soapTree(t, svc, store)
	first := must(svc.ImportSoapResource(ctx, p.ID, domain.SoapResourceWSDL, service.Upload{Filename: "a.wsdl", Data: []byte("<a/>")}))
	schema := must(svc.ImportSoapResource(ctx, p.ID, domain.SoapResourceSchema, service.Upload{Filename: "a.xsd", Data: []byte("<s/>")}))
	second := must(svc.ImportSoapResource(ctx, p.ID, domain.SoapResourceWSDL, service.Upload{Filename: "b.wsdl", Data: []byte("<b/>")}))
	got := must(svc.SoapProject(ctx, p.ID))
	if len(got.Resources) != 2 || got.Resources[0].ID != schema.ID || got.Resources[1].ID != second.ID {
		t.Fatalf("resources %+v", got.Resources)
	}
	if _, err := svc.SoapResource(ctx, first.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("old wsdl still present: %v", err)
	}
	if c := must(svc.SoapResourceContent(ctx, second.ID)); c != "<b/>" {
		t.Fatalf("content %q", c)
	}
	if _, err := svc.ImportSoapResource(ctx, p.ID, "PDF", service.Upload{Data: []byte("x")}); !isValidation(err) {
		t.Fatalf("bad type accepted: %v", err)
	}
}

func TestBulkSkipsUnknownIDs(t *testing.T) {
	svc, store := newService(t)
	ctx := context.Background()
	_, m, _ := restTree(t, svc)
	soapTree(t, svc, store)
	if err := svc.SetRestMethodsForwardedEndpoint(ctx, []string{"nope", m.ID}, "http://e"); err != nil {
		t.Fatal(err)
	}
	if got := must(svc.RestMethod(ctx, m.ID)); got.ForwardedEndpoint != "http://e" {
		t.Fatalf("endpoint %q", got.ForwardedEndpoint)
	}
	if err := svc.SetSoapPortsStatus(ctx, []string{"nope"}, domain.SoapOperationEcho); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetSoapOperationsForwardedEndpoint(ctx, []string{"Op0001"}, "http://s"); err != nil {
		t.Fatal(err)
	}
	if got := must(svc.SoapOperation(ctx, "Op0001")); got.ForwardedEndpoint != "http://s" {
		t.Fatalf("soap endpoint %q", got.ForwardedEndpoint)
	}
}

func TestEvents(t *testing.T) {
	svc, store := newService(t)
	ctx := context.Background()
	_ = store.Events().AddRest(ctx, domain.RestEvent{Event: domain.Event{ID: "e1", ResourceName: "get"}})
	_ = store.Events().AddSoap(ctx, domain.SoapEvent{Event: domain.Event{ID: "e2", ResourceName: "Add"}})
	ev := must(svc.Events(ctx))
	if len(ev) != 2 || ev[0].Type != domain.ProjectTypeREST || ev[1].Type != domain.ProjectTypeSOAP {
		t.Fatalf("events %+v", ev)
	}
	re := must(svc.RestEvent(ctx, "e1"))
	if re.Request.HTTPHeaders == nil || re.Request.HTTPParameters == nil {
		t.Fatal("event lists must not be nil")
	}
	if err := svc.ClearEvents(ctx); err != nil {
		t.Fatal(err)
	}
	if ev := must(svc.Events(ctx)); len(ev) != 0 {
		t.Fatalf("events after clear %d", len(ev))
	}
	if _, err := svc.SoapEvent(ctx, "e2"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestUserRules(t *testing.T) {
	svc, store := newService(t)
	ctx := context.Background()
	admin := must(svc.CreateUser(ctx, service.UserInput{Username: "admin", Password: "pw", Status: domain.UserActive, Role: domain.RoleAdmin}))
	reader := must(svc.CreateUser(ctx, service.UserInput{Username: "reader", Password: "pw", Status: domain.UserActive, Role: domain.RoleReader}))
	if admin.Password != "" {
		t.Fatal("hash returned")
	}
	tests := []struct {
		name  string
		err   error
		check func(error) bool
	}{
		{"duplicate username", func() error {
			_, err := svc.CreateUser(ctx, service.UserInput{Username: "ADMIN", Password: "x", Status: domain.UserActive, Role: domain.RoleReader})
			return err
		}(), func(err error) bool { return errors.Is(err, storage.ErrConflict) }},
		{"empty password", func() error {
			_, err := svc.CreateUser(ctx, service.UserInput{Username: "new", Status: domain.UserActive, Role: domain.RoleReader})
			return err
		}(), isValidation},
		{"bad role", func() error {
			_, err := svc.CreateUser(ctx, service.UserInput{Username: "new", Password: "x", Status: domain.UserActive, Role: "ROOT"})
			return err
		}(), isValidation},
		{"last admin demoted", func() error {
			_, err := svc.UpdateUser(ctx, admin.ID, service.UserInput{Username: "admin", Status: domain.UserActive, Role: domain.RoleModifier})
			return err
		}(), isValidation},
		{"last admin locked", func() error {
			_, err := svc.UpdateUser(ctx, admin.ID, service.UserInput{Username: "admin", Status: domain.UserLocked, Role: domain.RoleAdmin})
			return err
		}(), isValidation},
		{"last admin deleted", svc.DeleteUser(ctx, admin.ID), isValidation},
		{"rename to taken username", func() error {
			_, err := svc.UpdateUser(ctx, reader.ID, service.UserInput{Username: "Admin", Status: domain.UserActive, Role: domain.RoleReader})
			return err
		}(), func(err error) bool { return errors.Is(err, storage.ErrConflict) }},
		{"unknown user", svc.DeleteUser(ctx, "nope"), func(err error) bool { return errors.Is(err, storage.ErrNotFound) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check(tt.err) {
				t.Fatalf("unexpected error %v", tt.err)
			}
		})
	}

	t.Run("update without password keeps the hash (D-2)", func(t *testing.T) {
		before := must(store.Users().Get(ctx, reader.ID)).Password
		must(svc.UpdateUser(ctx, reader.ID, service.UserInput{Username: "reader", Email: "r@x", Status: domain.UserActive, Role: domain.RoleModifier}))
		after := must(store.Users().Get(ctx, reader.ID))
		if after.Password != before || after.Role != domain.RoleModifier || after.Email != "r@x" {
			t.Fatalf("user %+v", after)
		}
		must(svc.UpdateProfile(ctx, reader.ID, service.ProfileInput{Username: "reader2", Password: "new"}))
		after = must(store.Users().Get(ctx, reader.ID))
		if !auth.CheckPassword(after.Password, "new") || after.Username != "reader2" || after.Role != domain.RoleModifier {
			t.Fatalf("profile update %+v", after)
		}
	})

	t.Run("second admin allows demoting the first", func(t *testing.T) {
		must(svc.CreateUser(ctx, service.UserInput{Username: "boss", Password: "pw", Status: domain.UserActive, Role: domain.RoleAdmin}))
		must(svc.UpdateUser(ctx, admin.ID, service.UserInput{Username: "admin", Status: domain.UserActive, Role: domain.RoleReader}))
	})
}

func TestSystemInformation(t *testing.T) {
	svc, _ := newService(t)
	info := must(svc.SystemInformation(context.Background()))
	if info.JavaVendor != "Go" || info.TomcatInfo != "mockingbird" || info.TomcatVersion != "test" || info.TotalMemory <= 0 || info.MaxMemory < info.TotalMemory && info.MaxMemory <= 0 {
		t.Fatalf("system %+v", info)
	}
}
