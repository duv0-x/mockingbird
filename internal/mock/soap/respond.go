package soap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

const (
	defaultCharset = `; charset="utf-8"`
	// addressTag marks the address rewritten on ?wsdl (SoapUtility.getWsdlAddress).
	addressTag = `<soap:address location="`
)

// servePost ports AbstractSoapServiceController.process.
func (e *Engine) servePost(w http.ResponseWriter, r *http.Request, projectID, path string, start time.Time) {
	ctx := r.Context()
	req, err := parseRequest(r, projectID, path, e.deps.ContextPath)
	if err != nil {
		e.fail(w, r, path, err)
		return
	}
	project, err := e.deps.Store.Soap().Get(ctx, projectID)
	if err != nil {
		e.fail(w, r, path, fmt.Errorf("load project %s: %w", projectID, err))
		return
	}
	port, op, err := identify(&project, req)
	if err != nil {
		e.fail(w, r, path, err)
		return
	}
	t := target{projectID: projectID, port: port, operation: op, url: requestURL(r)}
	resp, err := e.process(ctx, req, t)
	if err == nil && resp == nil {
		err = fmt.Errorf("%w: unable to process request: %s", errNoResponse, req.URI)
	}
	if err == nil && http.StatusText(resp.HTTPStatusCode) == "" {
		// HttpStatus.valueOf rejects unknown codes.
		err = fmt.Errorf("unknown HTTP status code %d", resp.HTTPStatusCode)
	}
	if err != nil {
		e.recordEvent(req, t, resp, start)
		e.fail(w, r, path, err)
		return
	}
	if domain.Deref(op.SimulateNetworkDelay) && domain.Deref(op.NetworkDelay) >= 0 {
		select {
		case <-time.After(time.Duration(domain.Deref(op.NetworkDelay)) * time.Millisecond):
		case <-ctx.Done():
		}
	}
	// The event is created in a finally block after the delay.
	e.recordEvent(req, t, resp, start)
	writeResponse(w, req.SoapVersion, resp)
	e.log.DebugContext(ctx, "SOAP mock request", "path", path, "project", projectID, "port", port.ID,
		"operation", op.ID, "status", resp.HTTPStatusCode, "duration", time.Since(start))
}

func (e *Engine) fail(w http.ResponseWriter, r *http.Request, path string, err error) {
	e.log.WarnContext(r.Context(), "SOAP service exception", "path", path, "err", err)
	writeError(w, http.StatusInternalServerError, path)
}

// serveWSDL ports AbstractSoapServiceController.processGet: with a "wsdl"
// query parameter (any case) the project's first WSDL resource is served with
// its first soap:address pointing at the request URL.
func (e *Engine) serveWSDL(w http.ResponseWriter, r *http.Request, projectID, path string) {
	ctx := r.Context()
	wanted := false
	for name := range r.URL.Query() {
		if strings.EqualFold(name, "wsdl") {
			wanted = true
			break
		}
	}
	if !wanted {
		e.fail(w, r, path, errors.New("unable to parse incoming message"))
		return
	}
	project, err := e.deps.Store.Soap().Get(ctx, projectID)
	if errors.Is(err, storage.ErrNotFound) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		e.fail(w, r, path, fmt.Errorf("load project %s: %w", projectID, err))
		return
	}
	for _, res := range project.Resources {
		if res.Type != domain.SoapResourceWSDL {
			continue
		}
		content, err := e.deps.Store.Soap().ResourceContent(ctx, projectID, res.ID)
		if errors.Is(err, storage.ErrNotFound) {
			break
		}
		if err != nil {
			e.fail(w, r, path, fmt.Errorf("load resource %s: %w", res.ID, err))
			return
		}
		w.Header().Set("Content-Type", "text/xml"+defaultCharset)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(rewriteAddress(content, requestURL(r))))
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

// rewriteAddress ports SoapUtility.getWsdlAddress.
func rewriteAddress(wsdl, address string) string {
	i := strings.Index(wsdl, addressTag)
	if i < 0 {
		return wsdl
	}
	start := i + len(addressTag)
	end := strings.Index(wsdl[start:], `"`)
	if end < 0 {
		return wsdl
	}
	return wsdl[:start] + address + wsdl[start+end:]
}

// writeResponse ports getHttpHeaders: the SOAP version content type, then
// every response header overriding it, except Content-Encoding (D-7); the
// framing headers are recomputed for the body actually written.
func writeResponse(w http.ResponseWriter, version domain.SoapVersion, resp *domain.SoapResponse) {
	h := w.Header()
	h.Set("Content-Type", version.ContentType()+defaultCharset)
	for _, header := range resp.HTTPHeaders {
		switch strings.ToLower(header.Name) {
		case "content-encoding", "content-length", "transfer-encoding":
			continue
		}
		h.Set(header.Name, header.Value)
	}
	h.Set("Content-Length", strconv.Itoa(len(resp.Body)))
	w.WriteHeader(resp.HTTPStatusCode)
	_, _ = w.Write([]byte(resp.Body))
}

// errorBody is Spring Boot's default error JSON (SPEC.md §4.8).
type errorBody struct {
	Timestamp domain.Time `json:"timestamp"`
	Status    int         `json:"status"`
	Error     string      `json:"error"`
	Path      string      `json:"path"`
}

func writeError(w http.ResponseWriter, status int, path string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Timestamp: domain.Now(), Status: status, Error: http.StatusText(status), Path: path})
}

// recordEvent stores the SoapEvent asynchronously (CreateSoapEventInput via
// processAsync). resp is nil on errors.
func (e *Engine) recordEvent(req domain.SoapRequest, t target, resp *domain.SoapResponse, start time.Time) {
	if req.HTTPHeaders == nil {
		req.HTTPHeaders = []domain.HTTPHeader{}
	}
	event := domain.SoapEvent{
		Event: domain.Event{
			ID:           domain.NewID(),
			ResourceName: t.operation.Name,
			StartDate:    domain.Time{Time: start.UTC()},
			EndDate:      domain.Now(),
		},
		Request:     req,
		ProjectID:   t.projectID,
		PortID:      t.port.ID,
		OperationID: t.operation.ID,
	}
	if resp != nil {
		r := *resp
		if r.HTTPHeaders == nil {
			r.HTTPHeaders = []domain.HTTPHeader{}
		}
		if r.ContentEncodings == nil {
			r.ContentEncodings = []domain.ContentEncoding{}
		}
		event.Response = &r
	}
	e.async(func(ctx context.Context) {
		if err := e.deps.Store.Events().AddSoap(ctx, event); err != nil {
			e.log.Error("unable to save SOAP event", "event", event.ID, "err", err)
		}
	})
}
