package wsdl

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

type fakeRepo struct {
	storage.SoapRepository
	project  domain.SoapProject
	contents map[string]string
}

func newRepo() *fakeRepo {
	return &fakeRepo{project: domain.SoapProject{Project: domain.Project{ID: "P1", Name: "p"}}, contents: map[string]string{}}
}

func (f *fakeRepo) Update(_ context.Context, id string, fn func(*domain.SoapProject) error) (domain.SoapProject, error) {
	if id != f.project.ID {
		return domain.SoapProject{}, storage.ErrNotFound
	}
	if err := fn(&f.project); err != nil {
		return domain.SoapProject{}, err
	}
	for rid := range f.contents {
		if f.project.Resource(rid) == nil {
			delete(f.contents, rid)
		}
	}
	return f.project, nil
}

func (f *fakeRepo) PutResourceContent(_ context.Context, projectID, resourceID, content string) error {
	if f.project.Resource(resourceID) == nil {
		return storage.ErrNotFound
	}
	f.contents[resourceID] = content
	return nil
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func importFile(t *testing.T, repo *fakeRepo, name string, generate bool) {
	t.Helper()
	if err := Import(context.Background(), repo, "P1", Source{Filename: name, Data: fixture(t, name)}, Options{GenerateResponse: generate}); err != nil {
		t.Fatalf("Import(%s): %v", name, err)
	}
}

func findPort(t *testing.T, p domain.SoapProject, name string) domain.SoapPort {
	t.Helper()
	for _, port := range p.Ports {
		if port.Name == name {
			return port
		}
	}
	t.Fatalf("port %q not found in %+v", name, p.Ports)
	return domain.SoapPort{}
}

const exampleNamespace = "http://Services/ServiceExample/ServiceExample/1/Schema"

// verify mirrors SoapPortConverterTest.verify.
func verify(t *testing.T, port domain.SoapPort, name string, version domain.SoapVersion, namespace string) domain.SoapOperation {
	t.Helper()
	if port.Name != name+".Endpoint" || port.URI != name+".Endpoint" || port.ProjectID != "P1" || port.ID == "" {
		t.Errorf("port = %q %q %q", port.Name, port.URI, port.ProjectID)
	}
	if len(port.Operations) != 1 {
		t.Fatalf("operations = %d", len(port.Operations))
	}
	op := port.Operations[0]
	if op.Name != name || op.ResponseStrategy != domain.SoapStrategyRandom || op.Status != domain.SoapOperationMocked ||
		op.SoapVersion != version || op.HTTPMethod != domain.MethodPOST || op.IdentifyStrategy != domain.IdentifyByElementNamespace ||
		op.PortID != port.ID {
		t.Errorf("operation = %+v", op)
	}
	if op.OperationIdentifier != (domain.SoapOperationIdentifier{Name: "Request", Namespace: namespace}) {
		t.Errorf("identifier = %+v", op.OperationIdentifier)
	}
	if domain.Deref(op.SimulateNetworkDelay) || domain.Deref(op.MockOnFailure) || domain.Deref(op.AutomaticForward) ||
		op.SimulateNetworkDelay == nil || op.ForwardedEndpoint == "" || op.ForwardedEndpoint != op.OriginalEndpoint {
		t.Errorf("operation settings = %+v", op)
	}
	return op
}

func TestImportFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		file, port string
		version    domain.SoapVersion
		namespace  string
	}{
		{"ServiceExample1.wsdl", "ServiceExample1", domain.SOAP11, exampleNamespace},
		{"ServiceExample4.wsdl", "ServiceExample4", domain.SOAP11, exampleNamespace},
		{"ServiceExample5.wsdl", "ServiceExample5", domain.SOAP11, exampleNamespace},
		{"ServiceExampleSoap12.wsdl", "ServiceExampleSoap12", domain.SOAP12, exampleNamespace},
		{"ServiceExample7.wsdl", "ServiceExample7", domain.SOAP11, ""},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()
			repo := newRepo()
			importFile(t, repo, tt.file, false)
			if len(repo.project.Resources) != 1 {
				t.Fatalf("resources = %+v", repo.project.Resources)
			}
			res := repo.project.Resources[0]
			if res.Name != tt.file || res.Type != domain.SoapResourceWSDL || repo.contents[res.ID] != string(fixture(t, tt.file)) {
				t.Errorf("resource = %+v", res)
			}
			op := verify(t, findPort(t, repo.project, tt.port+".Endpoint"), tt.port, tt.version, tt.namespace)
			if len(op.MockResponses) != 0 {
				t.Errorf("responses generated without generateResponse")
			}
		})
	}
}

func TestImportGenerateResponse(t *testing.T) {
	t.Parallel()
	repo := newRepo()
	importFile(t, repo, "ServiceExample1.wsdl", true)
	op := findPort(t, repo.project, "ServiceExample1.Endpoint").Operations[0]
	want := `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:web="` + exampleNamespace + `">
   <soapenv:Header/>
   <soapenv:Body>
      <web:Response>?</web:Response>
   </soapenv:Body>
</soapenv:Envelope>`
	if op.DefaultBody != want {
		t.Errorf("default body = %q", op.DefaultBody)
	}
	if len(op.MockResponses) != 1 {
		t.Fatalf("responses = %+v", op.MockResponses)
	}
	r := op.MockResponses[0]
	if r.Name != "Auto-generated mocked response" || r.Body != want || r.HTTPStatusCode != 200 || r.Status != domain.SoapMockResponseEnabled ||
		domain.Deref(r.UsingExpressions) || r.OperationID != op.ID || r.ID == "" {
		t.Errorf("response = %+v", r)
	}
}

func TestImportAnyWSDL(t *testing.T) {
	t.Parallel()
	repo := newRepo()
	importFile(t, repo, "wsdl.wsdl", true)
	if len(repo.project.Ports) == 0 {
		t.Fatal("no ports imported from wsdl.wsdl")
	}
	for _, port := range repo.project.Ports {
		if len(port.Operations) == 0 {
			t.Errorf("port %s without operations", port.Name)
		}
	}
}

func TestImportLinkWithImports(t *testing.T) {
	t.Parallel()
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		b, err := os.ReadFile("testdata" + r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)

	repo := newRepo()
	repo.project.Resources = []domain.SoapResource{
		{ID: "OLD1", Name: "old.wsdl", Type: domain.SoapResourceWSDL},
		{ID: "OLD2", Name: "old.xsd", Type: domain.SoapResourceSchema},
	}
	repo.contents["OLD1"], repo.contents["OLD2"] = "old", "schema"
	err := Import(context.Background(), repo, "P1", Source{URL: srv.URL + "/ServiceExample1.wsdl"}, Options{IncludeImports: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(hits, ",") != "/ServiceExample1.wsdl,/ServiceExample2.wsdl,/ServiceExample3.wsdl" {
		t.Errorf("downloads = %v", hits)
	}
	types := map[string]domain.SoapResourceType{}
	for _, r := range repo.project.Resources {
		types[r.Name] = r.Type
	}
	want := map[string]domain.SoapResourceType{
		"old.xsd": domain.SoapResourceSchema, "ServiceExample1.wsdl": domain.SoapResourceWSDL,
		"ServiceExample2.wsdl": domain.SoapResourceWSDLImport, "ServiceExample3.wsdl": domain.SoapResourceWSDLImport,
	}
	if len(types) != len(want) {
		t.Fatalf("resources = %v", types)
	}
	for k, v := range want {
		if types[k] != v {
			t.Errorf("resource %s = %s, want %s", k, types[k], v)
		}
	}
	if _, ok := repo.contents["OLD1"]; ok {
		t.Error("old WSDL content not removed")
	}
	for _, name := range []string{"ServiceExample1", "ServiceExample2", "ServiceExample3"} {
		verify(t, findPort(t, repo.project, name+".Endpoint"), name, domain.SOAP11, exampleNamespace)
	}

	hits = nil
	repo = newRepo()
	if err := Import(context.Background(), repo, "P1", Source{URL: srv.URL + "/ServiceExample1.wsdl"}, Options{}); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || len(repo.project.Ports) != 1 {
		t.Errorf("without includeImports: downloads %v, ports %d", hits, len(repo.project.Ports))
	}
}

func TestImportMerge(t *testing.T) {
	t.Parallel()
	repo := newRepo()
	repo.project.Ports = []domain.SoapPort{{
		ID: "PORT1", Name: "ServiceExample1.Endpoint", URI: "custom", ProjectID: "P1",
		Operations: []domain.SoapOperation{{
			ID: "OP1", Name: "ServiceExample1", PortID: "PORT1", SoapVersion: domain.SOAP12,
			OriginalEndpoint: "http://old", ForwardedEndpoint: "http://mine", Status: domain.SoapOperationForwarded,
			MockResponses: []domain.SoapMockResponse{{ID: "MR1", Name: "kept"}},
		}},
	}}
	importFile(t, repo, "ServiceExample1.wsdl", true)
	if len(repo.project.Ports) != 1 {
		t.Fatalf("ports = %d", len(repo.project.Ports))
	}
	port := repo.project.Ports[0]
	op := port.Operations[0]
	if port.URI != "custom" || len(port.Operations) != 1 || op.ID != "OP1" || op.SoapVersion != domain.SOAP11 ||
		op.OriginalEndpoint != "https://localhost:8443/ESB/Services/ServiceExample" || op.ForwardedEndpoint != "http://mine" ||
		op.Status != domain.SoapOperationForwarded || len(op.MockResponses) != 1 {
		t.Errorf("merged = %+v", port)
	}
	importFile(t, repo, "ServiceExample4.wsdl", false)
	if len(repo.project.Ports) != 2 {
		t.Errorf("new port not added: %d", len(repo.project.Ports))
	}
}

func TestImportErrors(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"not xml":         "nope",
		"missing binding": `<wsdl:definitions xmlns:wsdl="http://schemas.xmlsoap.org/wsdl/" xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"><wsdl:service name="S"><wsdl:port name="P" binding="tns:B"><soap:address location="http://x"/></wsdl:port></wsdl:service></wsdl:definitions>`,
	}
	for name, data := range tests {
		err := Import(context.Background(), newRepo(), "P1", Source{Filename: "x.wsdl", Data: []byte(data)}, Options{})
		if !errors.Is(err, ErrInvalidWSDL) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if err := Import(context.Background(), newRepo(), "P1", Source{}, Options{}); err == nil {
		t.Error("empty source accepted")
	}
	// A port without SOAP address is skipped.
	noAddress := `<wsdl:definitions xmlns:wsdl="http://schemas.xmlsoap.org/wsdl/"><wsdl:service name="S"><wsdl:port name="P" binding="tns:B"/></wsdl:service></wsdl:definitions>`
	repo := newRepo()
	if err := Import(context.Background(), repo, "P1", Source{Filename: "x.wsdl", Data: []byte(noAddress)}, Options{}); err != nil || len(repo.project.Ports) != 0 {
		t.Errorf("no address: err %v, ports %d", err, len(repo.project.Ports))
	}
}
