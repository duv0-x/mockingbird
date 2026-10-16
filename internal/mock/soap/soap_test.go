package soap

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

const (
	projectID = "PRJ001"
	portURI   = "CalculatorSoap"
	mockURL   = "/castlemock/mock/soap/project/" + projectID + "/" + portURI
	tempuri   = "http://tempuri.org/"
)

func envelope(body string) string {
	return `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tem="` + tempuri + `">
   <soapenv:Header/>
   <soapenv:Body>` + body + `</soapenv:Body>
</soapenv:Envelope>`
}

var addRequest = envelope(`<tem:Add><tem:intA>1</tem:intA><tem:intB>2</tem:intB></tem:Add>`)

func mockResponse(id, name, body string, mutate ...func(*domain.SoapMockResponse)) domain.SoapMockResponse {
	r := domain.SoapMockResponse{ID: id, Name: name, Body: body, HTTPStatusCode: 200, Status: domain.SoapMockResponseEnabled}
	for _, m := range mutate {
		m(&r)
	}
	r.EnsureSlices()
	return r
}

func operation(mutate func(*domain.SoapOperation), responses ...domain.SoapMockResponse) domain.SoapOperation {
	op := domain.SoapOperation{
		ID: "OP1", Name: "Add", PortID: "PORT1",
		OperationIdentifier: domain.SoapOperationIdentifier{Name: "Add", Namespace: tempuri},
		ResponseStrategy:    domain.SoapStrategyRandom, Status: domain.SoapOperationMocked, HTTPMethod: domain.MethodPOST,
		SoapVersion: domain.SOAP11, IdentifyStrategy: domain.IdentifyByElementNamespace, MockResponses: responses,
	}
	if mutate != nil {
		mutate(&op)
	}
	return op
}

func project(ops ...domain.SoapOperation) domain.SoapProject {
	p := domain.SoapProject{
		Project: domain.Project{ID: projectID, Name: "Calculator"},
		Ports:   []domain.SoapPort{{ID: "PORT1", Name: portURI, URI: portURI, ProjectID: projectID, Operations: ops}},
	}
	p.EnsureSlices()
	return p
}

type call struct {
	method, path, body string
	headers            map[string]string
}

func do(t *testing.T, e *Engine, c call) *httptest.ResponseRecorder {
	t.Helper()
	if c.method == "" {
		c.method = http.MethodPost
	}
	if c.path == "" {
		c.path = mockURL
	}
	req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
	if _, ok := c.headers["Content-Type"]; !ok && c.method == http.MethodPost {
		req.Header.Set("Content-Type", "text/xml;charset=UTF-8")
	}
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

func TestMocked(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		op         domain.SoapOperation
		call       call
		wantStatus int
		wantBody   string
		wantHeader map[string]string
	}{
		{
			name:       "random",
			op:         operation(nil, mockResponse("R1", "Response", "<ok/>")),
			call:       call{body: addRequest},
			wantStatus: 200, wantBody: "<ok/>", wantHeader: map[string]string{"Content-Type": `text/xml; charset="utf-8"`},
		},
		{
			name: "soap 1.2 content type and header override without content encoding",
			op: operation(func(o *domain.SoapOperation) { o.SoapVersion = domain.SOAP12 }, mockResponse("R1", "Response", "<ok/>", func(r *domain.SoapMockResponse) {
				r.HTTPStatusCode = 202
				r.HTTPHeaders = []domain.HTTPHeader{{Name: "X-A", Value: "1"}, {Name: "Content-Encoding", Value: "gzip"}}
			})),
			call:       call{body: addRequest, headers: map[string]string{"Content-Type": `application/soap+xml;charset=UTF-8;action="Add"`}},
			wantStatus: 202, wantBody: "<ok/>",
			wantHeader: map[string]string{"Content-Type": `application/soap+xml; charset="utf-8"`, "X-A": "1", "Content-Encoding": ""},
		},
		{
			name: "response content type overrides",
			op: operation(nil, mockResponse("R1", "Response", "<ok/>", func(r *domain.SoapMockResponse) {
				r.HTTPHeaders = []domain.HTTPHeader{{Name: "Content-Type", Value: "application/xml"}}
			})),
			call:       call{body: addRequest},
			wantStatus: 200, wantHeader: map[string]string{"Content-Type": "application/xml"}, wantBody: "<ok/>",
		},
		{
			name: "xpath on the envelope",
			op: operation(func(o *domain.SoapOperation) { o.ResponseStrategy = domain.SoapStrategyXPathInput },
				mockResponse("R1", "A", "one", func(r *domain.SoapMockResponse) {
					r.XPathExpressions = []domain.SoapXPathExpression{{Expression: "//Add/intA[text()='5']"}}
				}),
				mockResponse("R2", "B", "two", func(r *domain.SoapMockResponse) {
					r.XPathExpressions = []domain.SoapXPathExpression{{Expression: "//Add/intA[text()='1']"}}
				})),
			call: call{body: addRequest}, wantStatus: 200, wantBody: "two",
		},
		{
			name: "xpath default response",
			op: operation(func(o *domain.SoapOperation) {
				o.ResponseStrategy = domain.SoapStrategyXPathInput
				o.DefaultMockResponseID = "R2"
			}, mockResponse("R1", "A", "one", func(r *domain.SoapMockResponse) {
				r.XPathExpressions = []domain.SoapXPathExpression{{Expression: "//Add/intA[text()='5']"}}
			}), mockResponse("R2", "B", "default")),
			call: call{body: addRequest}, wantStatus: 200, wantBody: "default",
		},
		{
			name: "xpath without match nor default",
			op: operation(func(o *domain.SoapOperation) { o.ResponseStrategy = domain.SoapStrategyXPathInput },
				mockResponse("R1", "A", "one")),
			call: call{body: addRequest}, wantStatus: 500,
		},
		{
			name: "expressions get the request body and URL",
			op: operation(nil, mockResponse("R1", "A", `${BODY_XPATH(expression="//intB/text()")}@${URL_HOST()}`, func(r *domain.SoapMockResponse) {
				r.UsingExpressions = domain.Ptr(true)
			})),
			call: call{body: addRequest}, wantStatus: 200, wantBody: "2@example.com",
		},
		{
			name:       "echo",
			op:         operation(func(o *domain.SoapOperation) { o.Status = domain.SoapOperationEcho }),
			call:       call{body: addRequest + "\r\n"},
			wantStatus: 200, wantBody: addRequest, wantHeader: map[string]string{"Content-Type": "text/xml;charset=UTF-8"},
		},
		{name: "disabled", op: operation(func(o *domain.SoapOperation) { o.Status = domain.SoapOperationDisabled }, mockResponse("R1", "A", "x")), call: call{body: addRequest}, wantStatus: 500},
		{name: "no enabled response", op: operation(nil, mockResponse("R1", "A", "x", func(r *domain.SoapMockResponse) { r.Status = domain.SoapMockResponseDisabled })), call: call{body: addRequest}, wantStatus: 500},
		{name: "unknown status code", op: operation(nil, mockResponse("R1", "A", "x", func(r *domain.SoapMockResponse) { r.HTTPStatusCode = 299 })), call: call{body: addRequest}, wantStatus: 500},
		{name: "unknown port", op: operation(nil, mockResponse("R1", "A", "x")), call: call{body: addRequest, path: mockURL + "x"}, wantStatus: 500},
		{name: "no port in path", op: operation(nil, mockResponse("R1", "A", "x")), call: call{body: addRequest, path: "/castlemock/mock/soap/project/" + projectID}, wantStatus: 500},
		{name: "unknown project", op: operation(nil, mockResponse("R1", "A", "x")), call: call{body: addRequest, path: "/castlemock/mock/soap/project/NOPE/" + portURI}, wantStatus: 500},
		{name: "invalid envelope", op: operation(nil, mockResponse("R1", "A", "x")), call: call{body: "not xml"}, wantStatus: 500},
		{name: "wrong soap version", op: operation(nil, mockResponse("R1", "A", "x")), call: call{body: addRequest, headers: map[string]string{"Content-Type": "application/soap+xml"}}, wantStatus: 500},
		{name: "unsupported method", op: operation(nil), call: call{method: http.MethodPut, body: addRequest}, wantStatus: 405},
		{name: "no project id", op: operation(nil), call: call{path: "/castlemock/mock/soap/project/", body: addRequest}, wantStatus: 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := do(t, newEngine(newFakeStore(project(tt.op))), tt.call)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus >= 400 {
				var e errorBody
				if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Status != tt.wantStatus || e.Path == "" {
					t.Errorf("error body = %s", rec.Body)
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

func TestIdentification(t *testing.T) {
	t.Parallel()
	other := "http://other.org/"
	tests := []struct {
		name string
		op   func(*domain.SoapOperation)
		body string
		want bool
	}{
		{"same namespace", nil, addRequest, true},
		{"name ignores case", func(o *domain.SoapOperation) { o.OperationIdentifier.Name = "ADD" }, addRequest, true},
		{"other namespace", nil, envelope(`<o:Add xmlns:o="` + other + `"/>`), false},
		{"namespace ignores case", func(o *domain.SoapOperation) { o.OperationIdentifier.Namespace = strings.ToUpper(tempuri) }, addRequest, true},
		{"identify by element", func(o *domain.SoapOperation) { o.IdentifyStrategy = domain.IdentifyByElement }, envelope(`<o:Add xmlns:o="` + other + `"/>`), true},
		{"operation without namespace", func(o *domain.SoapOperation) { o.OperationIdentifier.Namespace = "" }, envelope(`<Add/>`), true},
		{"request without namespace", nil, envelope(`<Add/>`), false},
		{"default namespace", nil, envelope(`<Add xmlns="` + tempuri + `"/>`), true},
		// D-10: the innermost declaration of the prefix wins.
		{"inner prefix declaration wins", nil, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" xmlns:t="` + other + `"><s:Body><t:Add xmlns:t="` + tempuri + `"/></s:Body></s:Envelope>`, true},
		{"other method", func(o *domain.SoapOperation) { o.HTTPMethod = domain.MethodGET }, addRequest, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := do(t, newEngine(newFakeStore(project(operation(tt.op, mockResponse("R1", "A", "ok"))))), call{body: tt.body})
			if got := rec.Code == 200; got != tt.want {
				t.Errorf("identified = %v (status %d), want %v", got, rec.Code, tt.want)
			}
		})
	}
}

// Cases from SoapUtilityTest.extractSoapRequestName.
func TestOperationIdentifier(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, body, wantName, wantNS string
		wantErr                      bool
	}{
		{"prefixed", `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope/" soap:encodingStyle="http://www.w3.org/2003/05/soap-encoding">
<soap:Body>
  <m:GetPrice xmlns:m="http://www.w3schools.com/prices"><m:Item>Apples</m:Item></m:GetPrice>
</soap:Body>
</soap:Envelope> `, "GetPrice", "http://www.w3schools.com/prices", false},
		{"default namespace", `<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope/"><soap:Body><GetPrice xmlns="http://www.w3schools.com/prices"><Item>Apples</Item></GetPrice></soap:Body></soap:Envelope>`,
			"GetPrice", "http://www.w3schools.com/prices", false},
		{"href", `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
	<soap:Body soap:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/" xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
		<ns3:GetPrice xmlns:ns3="http://www.w3schools.com/prices"><in href="#id1"/></ns3:GetPrice>
		<ns3:GetPriceIn id="id1" xmlns:ns3="http://www.w3schools.com/prices" xsi:type="ns3:GetPriceIn"></ns3:GetPriceIn>
	</soap:Body>
</soapenv:Envelope>`, "GetPrice", "http://www.w3schools.com/prices", false},
		{"no namespace", `<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope/"><soap:Body><GetPrice><Item>Apples</Item></GetPrice></soap:Body></soap:Envelope>`, "GetPrice", "", false},
		{"prefix declaration named like the element", `<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope/"><soap:Body><GetPrice xmlns:GetPrice="http://www.w3schools.com/prices"><m:Item>Apples</m:Item></GetPrice></soap:Body></soap:Envelope>`, "GetPrice", "", false},
		{"unprefixed body", `<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope/"><Body><GetPrice><Item>Apples</Item></GetPrice></Body></soap:Envelope>`, "GetPrice", "", false},
		{"body namespace on body", `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><GetPrice><Item>Apples</Item></GetPrice></soap:Body></soapenv:Envelope>`, "GetPrice", "", false},
		{"no soap namespaces", `<Envelope><Body><GetPrice><Item>Apples</Item></GetPrice></Body></Envelope>`, "GetPrice", "", false},
		{"empty", "", "", "", true},
		{"no body", `<Envelope><Header/></Envelope>`, "", "", true},
		{"empty body", `<Envelope><Body>  </Body></Envelope>`, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := operationIdentifier(tt.body)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (got.Name != tt.wantName || got.Namespace != tt.wantNS) {
				t.Errorf("identifier = %+v, want %s %s", got, tt.wantName, tt.wantNS)
			}
		})
	}
}

const mtomBody = `------=_Part_64_1526053806.1517665317492
Content-Type: text/xml; charset=UTF-8
Content-Transfer-Encoding: 8bit
Content-ID: <test@castlemock.org>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cas="http://castlemock.com/">
   <soapenv:Header/>
   <soapenv:Body>
      <cas:TestService>
         <Variable1>?</Variable1>
      </cas:TestService>
   </soapenv:Body>
</soapenv:Envelope>
------=_Part_64_1526053806.1517665317492
Content-Type: text/plain; charset=us-ascii; name="example"
Content-ID: <example>
Content-Disposition: attachment; name="example.txt"; filename="example.txt"
This is an example
------=_Part_24_1742827313.1517654770545--`

const mtomContentType = `multipart/related; type="text/xml"; start="<test@castlemock.org>"; boundary="----=_Part_64_1526053806.1517665317492"`

// MtomUtilityTest plus a standard CRLF message whose root is not first.
func TestMTOMEnvelope(t *testing.T) {
	t.Parallel()
	got, err := mtomEnvelope(mtomBody, mtomContentType)
	if err != nil {
		t.Fatal(err)
	}
	want := `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cas="http://castlemock.com/">
   <soapenv:Header/>
   <soapenv:Body>
      <cas:TestService>
         <Variable1>?</Variable1>
      </cas:TestService>
   </soapenv:Body>
</soapenv:Envelope>
`
	if got != want {
		t.Errorf("envelope = %q", got)
	}
	std := "--b\r\nContent-ID: <att>\r\n\r\nattachment\r\n--b\r\nContent-Type: application/xop+xml\r\nContent-ID: <root>\r\n\r\n<Envelope/>\r\n--b--\r\n"
	if got, err := mtomEnvelope(std, `multipart/related; boundary=b; start="<root>"`); err != nil || strings.TrimSpace(got) != "<Envelope/>" {
		t.Errorf("standard = %q, %v", got, err)
	}
	if got, err := mtomEnvelope(std, `multipart/related; boundary=b`); err != nil || strings.TrimSpace(got) != "attachment" {
		t.Errorf("without start = %q, %v", got, err)
	}
	if _, err := mtomEnvelope(std, `multipart/related`); err == nil {
		t.Error("missing boundary accepted")
	}
}

// SoapServiceControllerTest.testMTOM: a multipart request is identified by its root part.
func TestMTOMRequest(t *testing.T) {
	t.Parallel()
	op := operation(func(o *domain.SoapOperation) {
		o.Name = "TestService"
		o.OperationIdentifier = domain.SoapOperationIdentifier{Name: "TestService", Namespace: "http://castlemock.com/"}
		o.ResponseStrategy = domain.SoapStrategyXPathInput
	}, mockResponse("R1", "A", "matched", func(r *domain.SoapMockResponse) {
		r.XPathExpressions = []domain.SoapXPathExpression{{Expression: "//TestService/Variable1[text()='?']"}}
	}))
	store := newFakeStore(project(op))
	rec := do(t, newEngine(store), call{body: mtomBody, headers: map[string]string{"Content-Type": mtomContentType}})
	if rec.Code != 200 || rec.Body.String() != "matched" {
		t.Fatalf("MTOM = %d %s", rec.Code, rec.Body)
	}
	ev := store.soapEvents()[0]
	if !strings.HasPrefix(ev.Request.Envelope, "<soapenv:Envelope") || !strings.HasPrefix(ev.Request.Body, "------=_Part") {
		t.Errorf("event request = %+v", ev.Request)
	}
}

func TestSequence(t *testing.T) {
	t.Parallel()
	op := operation(func(o *domain.SoapOperation) { o.ResponseStrategy = domain.SoapStrategySequence },
		mockResponse("R3", "Response 10", "third"), mockResponse("R2", "Response 2", "second"), mockResponse("R1", "Response 1", "first"))
	e := newEngine(newFakeStore(project(op)))
	var got []string
	for range 4 {
		got = append(got, do(t, e, call{body: addRequest}).Body.String())
	}
	if strings.Join(got, ",") != "first,second,third,first" {
		t.Errorf("sequence = %v", got)
	}
}

func TestForwarding(t *testing.T) {
	t.Parallel()
	var seen struct{ path, body, action string }
	backendStatus := 200
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen.path, seen.body, seen.action = r.URL.RequestURI(), string(b), r.Header.Get("SOAPAction")
		w.Header().Set("Content-Type", "text/xml")
		w.Header().Set("X-Backend", "yes")
		w.WriteHeader(backendStatus)
		_, _ = w.Write([]byte("<forwarded/>"))
	}))
	t.Cleanup(backend.Close)
	fwd := func(status domain.SoapOperationStatus, mutate ...func(*domain.SoapOperation)) domain.SoapOperation {
		return operation(func(o *domain.SoapOperation) {
			o.Status = status
			o.ForwardedEndpoint = backend.URL + "/calculator.asmx"
			for _, m := range mutate {
				m(o)
			}
		}, mockResponse("R1", "Mocked", "<mocked/>"))
	}
	mockOnFailure := func(o *domain.SoapOperation) { o.MockOnFailure = domain.Ptr(true) }

	t.Run("forwarded", func(t *testing.T) {
		rec := do(t, newEngine(newFakeStore(project(fwd(domain.SoapOperationForwarded)))),
			call{body: addRequest, path: mockURL + "?x=1", headers: map[string]string{"SOAPAction": "Add"}})
		if rec.Code != 200 || rec.Body.String() != "<forwarded/>" || rec.Header().Get("X-Backend") != "yes" {
			t.Errorf("response = %d %s", rec.Code, rec.Body)
		}
		if seen.path != "/calculator.asmx" || seen.body != addRequest || seen.action != "Add" {
			t.Errorf("backend saw %+v", seen)
		}
	})

	backendStatus = 500
	t.Run("forwarded failure with mock on failure", func(t *testing.T) {
		rec := do(t, newEngine(newFakeStore(project(fwd(domain.SoapOperationForwarded, mockOnFailure)))), call{body: addRequest})
		if rec.Code != 200 || rec.Body.String() != "<mocked/>" {
			t.Errorf("response = %d %s", rec.Code, rec.Body)
		}
	})
	t.Run("forwarded failure without mock on failure", func(t *testing.T) {
		rec := do(t, newEngine(newFakeStore(project(fwd(domain.SoapOperationForwarded)))), call{body: addRequest})
		if rec.Code != 500 || rec.Body.String() != "<forwarded/>" {
			t.Errorf("response = %d %s", rec.Code, rec.Body)
		}
	})
	t.Run("recording failure with mock on failure does not record", func(t *testing.T) {
		store := newFakeStore(project(fwd(domain.SoapOperationRecording, mockOnFailure)))
		rec := do(t, newEngine(store), call{body: addRequest})
		if rec.Body.String() != "<mocked/>" || len(store.project(projectID).Ports[0].Operations[0].MockResponses) != 1 {
			t.Errorf("response = %s", rec.Body)
		}
	})

	backendStatus = 200
	t.Run("recording", func(t *testing.T) {
		store := newFakeStore(project(fwd(domain.SoapOperationRecording)))
		do(t, newEngine(store), call{body: addRequest})
		op := store.project(projectID).Ports[0].Operations[0]
		if len(op.MockResponses) != 2 || op.Status != domain.SoapOperationRecording {
			t.Fatalf("operation = %+v", op)
		}
		r := op.MockResponses[1]
		if !strings.HasPrefix(r.Name, "Recorded response ") || r.Body != "<forwarded/>" || r.HTTPStatusCode != 200 ||
			r.Status != domain.SoapMockResponseEnabled || domain.Deref(r.UsingExpressions) || r.OperationID != "OP1" {
			t.Errorf("recorded = %+v", r)
		}
	})
	t.Run("record once", func(t *testing.T) {
		store := newFakeStore(project(fwd(domain.SoapOperationRecordOnce)))
		do(t, newEngine(store), call{body: addRequest})
		op := store.project(projectID).Ports[0].Operations[0]
		if len(op.MockResponses) != 2 || op.Status != domain.SoapOperationMocked {
			t.Errorf("operation = %+v", op)
		}
	})
	t.Run("automatic forward", func(t *testing.T) {
		op := fwd(domain.SoapOperationMocked, func(o *domain.SoapOperation) {
			o.AutomaticForward = domain.Ptr(true)
			o.MockResponses = nil
		})
		rec := do(t, newEngine(newFakeStore(project(op))), call{body: addRequest})
		if rec.Code != 200 || rec.Body.String() != "<forwarded/>" {
			t.Errorf("response = %d %s", rec.Code, rec.Body)
		}
	})
	t.Run("unreachable endpoint", func(t *testing.T) {
		op := fwd(domain.SoapOperationForwarded, func(o *domain.SoapOperation) { o.ForwardedEndpoint = "http://127.0.0.1:1" })
		if rec := do(t, newEngine(newFakeStore(project(op))), call{body: addRequest}); rec.Code != 500 {
			t.Errorf("status = %d", rec.Code)
		}
	})
}

func TestEvents(t *testing.T) {
	t.Parallel()
	store := newFakeStore(project(operation(nil, mockResponse("R1", "Mocked", "<ok/>"))))
	e := newEngine(store)
	do(t, e, call{body: addRequest, headers: map[string]string{"SOAPAction": "Add"}})
	do(t, e, call{body: envelope(`<tem:Unknown/>`)})
	events := store.soapEvents()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1 (none before identification)", len(events))
	}
	ev := events[0]
	if ev.ResourceName != "Add" || ev.ProjectID != projectID || ev.PortID != "PORT1" || ev.OperationID != "OP1" {
		t.Errorf("event = %+v", ev)
	}
	req := ev.Request
	if req.URI != portURI || req.HTTPMethod != domain.MethodPOST || req.SoapVersion != domain.SOAP11 || req.Body != addRequest ||
		req.OperationIdentifier != (domain.SoapOperationIdentifier{Name: "Add", Namespace: tempuri}) || req.ContentType != "text/xml;charset=UTF-8" {
		t.Errorf("event request = %+v", req)
	}
	if ev.Response == nil || ev.Response.Body != "<ok/>" || ev.Response.MockResponseName != "Mocked" {
		t.Errorf("event response = %+v", ev.Response)
	}

	store = newFakeStore(project(operation(func(o *domain.SoapOperation) { o.Status = domain.SoapOperationDisabled })))
	do(t, newEngine(store), call{body: addRequest})
	if events := store.soapEvents(); len(events) != 1 || events[0].Response != nil {
		t.Errorf("failed request events = %+v", events)
	}
}

func TestNetworkDelay(t *testing.T) {
	t.Parallel()
	op := operation(func(o *domain.SoapOperation) {
		o.SimulateNetworkDelay = domain.Ptr(true)
		o.NetworkDelay = domain.Ptr(int64(30))
	}, mockResponse("R1", "A", "late"))
	start := time.Now()
	if rec := do(t, newEngine(newFakeStore(project(op))), call{body: addRequest}); rec.Body.String() != "late" || time.Since(start) < 30*time.Millisecond {
		t.Errorf("delay not applied")
	}
}

const wsdlContent = `<wsdl:definitions>
  <wsdl:service name="Service">
    <wsdl:port name="ServiceHttpPost" binding="tns:ServiceHttpPost">
      <soap:address location="http://www.castlemock.com/services/myservice"/>
    </wsdl:port>
    <wsdl:port name="Second"><soap:address location="http://second"/></wsdl:port>
  </wsdl:service>
</wsdl:definitions>`

func TestWSDL(t *testing.T) {
	t.Parallel()
	p := project(operation(nil))
	p.Resources = []domain.SoapResource{
		{ID: "XSD", Name: "types.xsd", Type: domain.SoapResourceSchema},
		{ID: "W1", Name: "service.wsdl", Type: domain.SoapResourceWSDL},
		{ID: "W2", Name: "other.wsdl", Type: domain.SoapResourceWSDL},
	}
	store := newFakeStore(p)
	store.contents["W1"], store.contents["W2"], store.contents["XSD"] = wsdlContent, "<other/>", "<xsd/>"
	e := newEngine(store)
	base := "/castlemock/mock/soap/project/" + projectID
	want := strings.Replace(wsdlContent, "http://www.castlemock.com/services/myservice", "http://example.com"+base+"/any/path", 1)

	rec := do(t, e, call{method: http.MethodGet, path: base + "/any/path?WSDL"})
	if rec.Code != 200 || rec.Body.String() != want || rec.Header().Get("Content-Type") != `text/xml; charset="utf-8"` {
		t.Errorf("wsdl = %d %q %v", rec.Code, rec.Body, rec.Header())
	}
	if rec := do(t, e, call{method: http.MethodGet, path: base + "?wsdl=x"}); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"http://example.com`+base+`"`) {
		t.Errorf("wsdl at project root = %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, e, call{method: http.MethodGet, path: base}); rec.Code != 500 {
		t.Errorf("without wsdl parameter = %d", rec.Code)
	}
	if rec := do(t, newEngine(newFakeStore(project(operation(nil)))), call{method: http.MethodGet, path: base + "?wsdl"}); rec.Code != 404 || rec.Body.Len() != 0 {
		t.Errorf("no wsdl resource = %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, e, call{method: http.MethodGet, path: "/castlemock/mock/soap/project/NOPE?wsdl"}); rec.Code != 404 {
		t.Errorf("unknown project = %d", rec.Code)
	}
}

// SoapUtilityTest (config): only the first soap:address is rewritten.
func TestRewriteAddress(t *testing.T) {
	t.Parallel()
	got := rewriteAddress(wsdlContent, "http://localhost:8080/other-path")
	if !strings.Contains(got, `"http://localhost:8080/other-path"`) || !strings.Contains(got, `"http://second"`) {
		t.Errorf("rewrite = %s", got)
	}
	without := "<wsdl:definitions><wsdl:port/></wsdl:definitions>"
	if rewriteAddress(without, "x") != without {
		t.Error("document without address changed")
	}
}
