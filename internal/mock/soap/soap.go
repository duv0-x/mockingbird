// Package soap serves SOAP mocks under {context}/mock/soap/project/{projectId}/** and
// WSDL documents on GET ...?wsdl (SPEC.md §4.6–4.7).
//
// Owner: T3.
package soap

import (
	"log/slog"
	"net/http"

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
}

func NewEngine(deps Deps) *Engine { return &Engine{deps: deps} }

// ServeHTTP receives the full request path (context path included).
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "SOAP mock engine not implemented (T3)", http.StatusNotImplemented)
}
