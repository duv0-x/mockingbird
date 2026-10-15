package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/config"
)

func named(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(name)) })
}

func testMounts() mounts {
	return mounts{
		rest: named("rest"),
		soap: named("soap"),
		auth: func(c *gin.Context) { c.Next() },
		api: func(g *gin.RouterGroup) {
			g.GET("/api/rest/core/version", func(c *gin.Context) { c.String(200, "api") })
		},
		web: func(g *gin.RouterGroup) {
			g.GET("/web", func(c *gin.Context) { c.String(200, "web") })
		},
	}
}

func TestRouter(t *testing.T) {
	tests := []struct {
		contextPath, method, path, want string
		status                          int
	}{
		{"/castlemock", http.MethodGet, "/castlemock/mock/rest/project/p/application/a/x", "rest", 200},
		{"/castlemock", http.MethodPatch, "/castlemock/mock/rest/project/p/application/a/x", "rest", 200},
		{"/castlemock", "TRACE", "/castlemock/mock/rest/project/p/application/a/x", "rest", 200},
		{"/castlemock", http.MethodPost, "/castlemock/mock/soap/project/p/Port", "soap", 200},
		{"/castlemock", http.MethodGet, "/castlemock/api/rest/core/version", "api", 200},
		{"/castlemock", http.MethodGet, "/castlemock/web", "web", 200},
		{"/castlemock", http.MethodGet, "/", "", http.StatusFound},
		{"/castlemock", http.MethodGet, "/healthz", "ok\n", 200},
		{"", http.MethodGet, "/mock/rest/project/p/application/a/x", "rest", 200},
		{"", http.MethodGet, "/api/rest/core/version", "api", 200},
		{"", http.MethodGet, "/healthz", "ok\n", 200},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.contextPath+tt.path, func(t *testing.T) {
			h := newRouter(config.Config{ContextPath: tt.contextPath}, logger, testMounts())
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if tt.want != "" && rec.Body.String() != tt.want {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tt.want)
			}
		})
	}
}
