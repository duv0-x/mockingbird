// Package rest serves REST mocks under {context}/mock/rest/project/{projectId}/application/{applicationId}/**
// (SPEC.md §4.1–4.3).
//
// Owner: T2.
package rest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/mock/expression"
	"github.com/duv0-x/mockingbird/internal/mock/forward"
	"github.com/duv0-x/mockingbird/internal/storage"
)

type Deps struct {
	Store       storage.Store
	Forward     *forward.Client
	ContextPath string
	Logger      *slog.Logger
}

type Engine struct {
	deps Deps
	log  *slog.Logger
	// background tracks the asynchronous event and recording writes.
	background sync.WaitGroup
}

func NewEngine(deps Deps) *Engine {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Engine{deps: deps, log: log}
}

// Wait blocks until the asynchronous writes started by previous requests
// (events, recorded responses) have finished. Call it on shutdown before
// closing the store.
func (e *Engine) Wait() { e.background.Wait() }

const (
	recordedResponseName  = "Recorded response"
	forwardedResponseName = "Forwarded response"
	recordedDateLayout    = "2006/01/02 15:04:05"
)

// errNoResponse covers the cases AbstractRestServiceController reports with a
// RestException; they all end as HTTP 500.
var errNoResponse = errors.New("no response")

// ServeHTTP receives the full request path (context path included).
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()
	req, err := parseRequest(r, e.deps.ContextPath)
	if err != nil {
		status := http.StatusNotFound
		if _, perr := domain.ParseHTTPMethod(r.Method); perr != nil {
			status = http.StatusMethodNotAllowed
		}
		e.log.WarnContext(ctx, "REST mock request rejected", "path", rawRequestPath(r), "err", err)
		writeError(w, status, rawRequestPath(r))
		return
	}
	id, err := identify(ctx, e.deps.Store.Rest(), req)
	if err != nil {
		e.log.WarnContext(ctx, "REST service exception", "path", req.path, "err", err)
		writeError(w, http.StatusInternalServerError, req.path)
		return
	}

	resp, err := e.process(ctx, req, id)
	if err == nil && http.StatusText(resp.HTTPStatusCode) == "" {
		// HttpStatus.valueOf rejects unknown codes after the response was built.
		err = fmt.Errorf("unknown HTTP status code %d", resp.HTTPStatusCode)
	}
	e.recordEvent(req, id, resp, start)
	if err != nil {
		e.log.WarnContext(ctx, "REST service exception", "path", req.path, "method", id.method.ID, "err", err)
		writeError(w, http.StatusInternalServerError, req.path)
		return
	}
	if domain.Deref(id.method.SimulateNetworkDelay) && id.method.NetworkDelay != nil {
		select {
		case <-time.After(time.Duration(*id.method.NetworkDelay) * time.Millisecond):
		case <-ctx.Done():
		}
	}
	accept, _ := headerValue(req.headers, "Accept")
	writeResponse(w, resp, accept)
	e.log.DebugContext(ctx, "REST mock request", "method", req.method, "path", req.path, "project", id.projectID,
		"restMethod", id.method.ID, "status", resp.HTTPStatusCode, "duration", time.Since(start))
}

// process dispatches on the method status (AbstractRestServiceController.process).
func (e *Engine) process(ctx context.Context, req *incoming, id *identified) (*domain.RestResponse, error) {
	switch id.method.Status {
	case domain.RestMethodDisabled:
		return nil, fmt.Errorf("%w: the requested REST method, %s, is disabled", errNoResponse, id.method.Name)
	case domain.RestMethodForwarded:
		return e.forward(ctx, req, id.method)
	case domain.RestMethodRecording:
		return e.forwardAndRecord(ctx, req, id)
	case domain.RestMethodRecordOnce:
		resp, err := e.forwardAndRecord(ctx, req, id)
		if err != nil {
			return nil, err
		}
		if err := e.setStatus(ctx, id, domain.RestMethodMocked); err != nil {
			return nil, err
		}
		return resp, nil
	case domain.RestMethodEcho:
		return echo(req), nil
	default:
		return e.mock(ctx, req, id)
	}
}

func echo(req *incoming) *domain.RestResponse {
	headers := []domain.HTTPHeader{}
	if req.contentType != "" {
		headers = append(headers, domain.HTTPHeader{Name: "Content-Type", Value: req.contentType})
	}
	return &domain.RestResponse{
		Body:             req.body,
		ContentType:      req.contentType,
		HTTPStatusCode:   http.StatusOK,
		HTTPHeaders:      headers,
		ContentEncodings: []domain.ContentEncoding{},
	}
}

// forward mirrors RestClient.getResponse: forwardedEndpoint + resource URI +
// "?name=value&..." (values not encoded), body only for POST, PUT and DELETE.
func (e *Engine) forward(ctx context.Context, req *incoming, method domain.RestMethod) (*domain.RestResponse, error) {
	if method.ForwardedEndpoint == "" {
		return nil, fmt.Errorf("%w: missing forwarded endpoint for the REST method %s", errNoResponse, method.ID)
	}
	target := method.ForwardedEndpoint + req.uri
	for i, p := range req.params {
		sep := "&"
		if i == 0 {
			sep = "?"
		}
		target += sep + p.Name + "=" + p.Value
	}
	out := forward.Request{Method: req.method, URL: target, Headers: req.headers}
	switch req.method {
	case domain.MethodPOST, domain.MethodPUT, domain.MethodDELETE:
		body := req.body
		out.Body = &body
	}
	resp, err := e.deps.Forward.Do(ctx, out)
	if err != nil {
		return nil, fmt.Errorf("%w: unable to forward request to configured endpoint: %w", errNoResponse, err)
	}
	return &domain.RestResponse{
		Body:             resp.Body,
		MockResponseName: forwardedResponseName,
		HTTPStatusCode:   resp.StatusCode,
		HTTPHeaders:      resp.Headers,
		ContentEncodings: resp.Encodings,
	}, nil
}

// forwardAndRecord forwards and stores the answer as a new mock response in
// the background, like the asynchronous CreateRestMockResponseInput.
func (e *Engine) forwardAndRecord(ctx context.Context, req *incoming, id *identified) (*domain.RestResponse, error) {
	resp, err := e.forward(ctx, req, id.method)
	if err != nil {
		return nil, err
	}
	recorded := domain.RestMockResponse{
		ID:               domain.NewID(),
		Name:             recordedResponseName + " " + time.Now().Format(recordedDateLayout),
		Body:             resp.Body,
		MethodID:         id.method.ID,
		HTTPStatusCode:   resp.HTTPStatusCode,
		Status:           domain.RestMockResponseEnabled,
		UsingExpressions: domain.Ptr(false),
		HTTPHeaders:      append([]domain.HTTPHeader{}, resp.HTTPHeaders...),
	}
	recorded.EnsureSlices()
	e.async(func(ctx context.Context) {
		_, err := e.deps.Store.Rest().Update(ctx, id.projectID, func(p *domain.RestProject) error {
			m := findMethod(p, id.resource.ID, id.method.ID)
			if m == nil {
				return fmt.Errorf("method %s not found", id.method.ID)
			}
			m.MockResponses = append(m.MockResponses, recorded)
			return nil
		})
		if err != nil {
			e.log.Error("unable to record REST response", "project", id.projectID, "method", id.method.ID, "err", err)
		}
	})
	return resp, nil
}

func (e *Engine) setStatus(ctx context.Context, id *identified, status domain.RestMethodStatus) error {
	_, err := e.deps.Store.Rest().Update(ctx, id.projectID, func(p *domain.RestProject) error {
		m := findMethod(p, id.resource.ID, id.method.ID)
		if m == nil {
			return fmt.Errorf("method %s not found", id.method.ID)
		}
		m.Status = status
		return nil
	})
	if err != nil {
		return fmt.Errorf("update status of REST method %s: %w", id.method.ID, err)
	}
	return nil
}

func findMethod(p *domain.RestProject, resourceID, methodID string) *domain.RestMethod {
	for i := range p.Applications {
		if res := p.Applications[i].Resource(resourceID); res != nil {
			return res.Method(methodID)
		}
	}
	return nil
}

// mock selects a mock response (AbstractRestServiceController.mockResponse).
func (e *Engine) mock(ctx context.Context, req *incoming, id *identified) (*domain.RestResponse, error) {
	method := id.method
	list := candidates(method, req.headers)
	if len(list) == 0 {
		return nil, fmt.Errorf("%w: no mocked response created for operation %s", errNoResponse, method.Name)
	}
	selected, err := pick(ctx, e.deps.Store.State(), method, list, req, id.pathParameters)
	if err != nil {
		return nil, err
	}
	if selected == nil && domain.Deref(method.AutomaticForward) && method.ForwardedEndpoint != "" {
		return e.forward(ctx, req, method)
	}
	if selected == nil {
		return nil, fmt.Errorf("%w: no mocked response created for operation %s", errNoResponse, method.Name)
	}
	body := selected.Body
	if domain.Deref(selected.UsingExpressions) {
		reqBody, reqURL := req.body, req.url
		body = expression.Render(body, expression.Input{
			Body:            &reqBody,
			URL:             &reqURL,
			PathParameters:  id.pathParameters,
			QueryParameters: req.params,
		})
	}
	headers := append([]domain.HTTPHeader{}, selected.HTTPHeaders...)
	encodings := append([]domain.ContentEncoding{}, selected.ContentEncodings...)
	return &domain.RestResponse{
		Body:             body,
		MockResponseName: selected.Name,
		HTTPStatusCode:   selected.HTTPStatusCode,
		HTTPHeaders:      headers,
		ContentEncodings: encodings,
	}, nil
}

// recordEvent stores the request/response log asynchronously
// (CreateRestEventInput through processAsync). resp is nil on errors.
func (e *Engine) recordEvent(req *incoming, id *identified, resp *domain.RestResponse, start time.Time) {
	event := domain.RestEvent{
		Event: domain.Event{
			ID:           domain.NewID(),
			ResourceName: id.method.Name,
			StartDate:    domain.Time{Time: start.UTC()},
			EndDate:      domain.Now(),
		},
		Request: domain.RestRequest{
			Body:           req.body,
			ContentType:    req.contentType,
			URI:            req.uri,
			HTTPMethod:     req.method,
			HTTPHeaders:    req.headers,
			HTTPParameters: req.params,
		},
		ProjectID:     req.projectID,
		ApplicationID: req.applicationID,
		ResourceID:    id.resource.ID,
		MethodID:      id.method.ID,
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
		if err := e.deps.Store.Events().AddRest(ctx, event); err != nil {
			e.log.Error("unable to save REST event", "event", event.ID, "err", err)
		}
	})
}

// async runs fn detached from the request, which may be gone by then.
func (e *Engine) async(fn func(ctx context.Context)) {
	e.background.Add(1)
	go func() {
		defer e.background.Done()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		fn(ctx)
	}()
}
