// Package rest serves REST mocks under {context}/mock/rest/project/{projectId}/application/{applicationId}/**
// (SPEC.md §4.1–4.3).
//
// Owner: T2.
package rest

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
	http.Error(w, "REST mock engine not implemented (T2)", http.StatusNotImplemented)
}
