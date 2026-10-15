package main

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
			t.Parallel()
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

func noEnv(string) string { return "" }

func TestRun(t *testing.T) {
	t.Run("version", func(t *testing.T) {
		t.Parallel()
		var out strings.Builder
		if err := run([]string{"version"}, noEnv, &out); err != nil {
			t.Fatalf("run version: %v", err)
		}
		if want := version + "\n"; out.String() != want {
			t.Fatalf("output = %q, want %q", out.String(), want)
		}
	})

	t.Run("version ignores configuration errors", func(t *testing.T) {
		t.Parallel()
		badPort := func(key string) string {
			if key == "MOCKINGBIRD_PORT" {
				return "not-a-number"
			}
			return ""
		}
		if err := run([]string{"version"}, badPort, io.Discard); err != nil {
			t.Fatalf("run version with invalid env: %v", err)
		}
	})

	t.Run("default command is serve with invalid config", func(t *testing.T) {
		t.Parallel()
		err := run(nil, func(key string) string {
			if key == "MOCKINGBIRD_PORT" {
				return "not-a-number"
			}
			return ""
		}, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "MOCKINGBIRD_PORT") {
			t.Fatalf("err = %v, want a MOCKINGBIRD_PORT configuration error", err)
		}
	})

	t.Run("import validates flags", func(t *testing.T) {
		t.Parallel()
		err := run([]string{"import"}, noEnv, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "--castlemock") {
			t.Fatalf("err = %v, want the --castlemock requirement", err)
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		t.Parallel()
		err := run([]string{"fly"}, noEnv, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "fly") {
			t.Fatalf("err = %v, want an unknown-command error naming it", err)
		}
	})
}

func TestImportCastleMockValidation(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()

	file := filepath.Join(t.TempDir(), "castlemock.file")
	if err := os.WriteFile(file, []byte("<restMethod/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		args  []string
		match string
	}{
		{"missing --castlemock", nil, "--castlemock"},
		{"source is a file", []string{"--castlemock", file}, "not a directory"},
		{"source is a dir with extra args", []string{"--castlemock", t.TempDir(), "extra"}, "unexpected argument"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := importCastleMock(cfg, tt.args, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tt.match) {
				t.Fatalf("err = %v, want it to mention %q", err, tt.match)
			}
		})
	}
}

func TestLimitBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		max    int64
		body   string
		wantEx bool
	}{
		{"body within the limit", 16, "hello", false},
		{"body over the limit", 5, "hello world", true},
		{"limit disabled", 0, "hello world", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := gin.New()
			r.Use(limitBody(tt.max))
			var readErr error
			r.POST("/x", func(c *gin.Context) {
				_, readErr = io.ReadAll(c.Request.Body)
				c.Status(http.StatusOK)
			})
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(tt.body)))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			var maxErr *http.MaxBytesError
			if got := errors.As(readErr, &maxErr); got != tt.wantEx {
				t.Fatalf("read error = %v, MaxBytesError = %t, want %t", readErr, got, tt.wantEx)
			}
		})
	}
}
