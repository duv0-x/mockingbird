// Package soap serves SOAP mocks under {context}/mock/soap/project/{projectId}/** and
// WSDL documents on GET ...?wsdl (SPEC.md §4.6–4.8).
//
// Port of web/web-mock/web-mock-soap: SoapServiceController,
// AbstractSoapServiceController, HttpServletRequestConverter and the
// stategy/factory packages.
//
// Owner: T3.
package soap

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

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
	// background tracks the asynchronous event and recording writes
	// (ServiceProcessor.processAsync).
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
// (events, recorded responses, sequence cursors) have finished. Call it on
// shutdown before closing the store.
func (e *Engine) Wait() { e.background.Wait() }

const projectPrefix = "/mock/soap/project/"

// ServeHTTP receives the full request path (context path included).
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	path := rawRequestPath(r)
	rest, found := strings.CutPrefix(path, e.deps.ContextPath+projectPrefix)
	projectID, _, _ := strings.Cut(rest, "/")
	if !found || projectID == "" {
		e.log.WarnContext(r.Context(), "SOAP mock request rejected", "path", path)
		writeError(w, http.StatusNotFound, path)
		return
	}
	switch r.Method {
	case http.MethodPost:
		e.servePost(w, r, projectID, path, start)
	case http.MethodGet, http.MethodHead:
		e.serveWSDL(w, r, projectID, path)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, path)
	}
}

// rawRequestPath is HttpServletRequest.getRequestURI: the path as sent,
// still percent-encoded.
func rawRequestPath(r *http.Request) string {
	if r.RequestURI != "" && !strings.Contains(r.RequestURI, "://") {
		path, _, _ := strings.Cut(r.RequestURI, "?")
		return path
	}
	return r.URL.EscapedPath()
}

// requestURL is HttpServletRequest.getRequestURL: scheme://host[:port]/path.
func requestURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host + rawRequestPath(r)
}

// errNoResponse covers the failures CastleMock reports with a SoapException;
// they all end as HTTP 500.
var errNoResponse = errors.New("no response")

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
