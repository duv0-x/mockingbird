package web

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/domain"
)

func TestSession(t *testing.T) {
	t.Parallel()
	e := newEnv(t)

	t.Run("login page is public", func(t *testing.T) {
		w := e.do(req{path: "/web/login"})
		assertStatus(t, w.Code, http.StatusOK)
		assertContains(t, w.Body.String(), `action="/castlemock/web/login"`, "Sign in")
	})
	t.Run("protected page redirects to login", func(t *testing.T) {
		w := e.do(req{path: "/web"})
		assertStatus(t, w.Code, http.StatusFound)
		if loc := w.Header().Get("Location"); loc != "/castlemock/web/login" {
			t.Fatalf("Location = %q", loc)
		}
	})
	t.Run("htmx without session gets HX-Redirect", func(t *testing.T) {
		w := e.do(req{path: "/web", htmx: true})
		assertStatus(t, w.Code, http.StatusUnauthorized)
		if got := w.Header().Get("HX-Redirect"); got != "/castlemock/web/login" {
			t.Fatalf("HX-Redirect = %q", got)
		}
	})
	t.Run("bad credentials", func(t *testing.T) {
		w := e.do(req{method: http.MethodPost, path: "/web/login", form: url.Values{"username": {"admin"}, "password": {"nope"}}})
		assertStatus(t, w.Code, http.StatusUnauthorized)
		assertContains(t, w.Body.String(), "Invalid username or password")
	})
	t.Run("good credentials set the cookie and follow next", func(t *testing.T) {
		w := e.do(req{method: http.MethodPost, path: "/web/login", form: url.Values{"username": {"ADMIN"}, "password": {"secret"}, "next": {"/castlemock/web/event"}}})
		assertStatus(t, w.Code, http.StatusSeeOther)
		if loc := w.Header().Get("Location"); loc != "/castlemock/web/event" {
			t.Fatalf("Location = %q", loc)
		}
		if !strings.Contains(w.Header().Get("Set-Cookie"), auth.CookieName+"=") {
			t.Fatalf("no session cookie: %q", w.Header().Get("Set-Cookie"))
		}
	})
	t.Run("open redirects are ignored", func(t *testing.T) {
		w := e.do(req{method: http.MethodPost, path: "/web/login", form: url.Values{"username": {"admin"}, "password": {"secret"}, "next": {"https://evil.test/"}}})
		if loc := w.Header().Get("Location"); loc != "/castlemock/web" {
			t.Fatalf("Location = %q", loc)
		}
	})
	t.Run("logout clears the cookie", func(t *testing.T) {
		w := e.do(req{path: "/web/logout", role: domain.RoleReader})
		assertStatus(t, w.Code, http.StatusFound)
		if !strings.Contains(w.Header().Get("Set-Cookie"), "Max-Age=0") {
			t.Fatalf("cookie not cleared: %q", w.Header().Get("Set-Cookie"))
		}
	})
	t.Run("root redirects to the UI", func(t *testing.T) {
		w := e.do(req{path: "/"})
		assertStatus(t, w.Code, http.StatusFound)
	})
}

func TestStaticAssets(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	for _, path := range []string{"css/tokens.css", "css/mockingbird.css", "js/htmx.min.js", "js/mockingbird.js", "img/favicon.svg", "fonts/inter-latin-wght-normal.woff2", "fonts/jetbrains-mono-latin-wght-normal.woff2"} {
		t.Run(path, func(t *testing.T) {
			w := e.do(req{path: "/static/" + path + "?v=test"})
			assertStatus(t, w.Code, http.StatusOK)
			if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
				t.Fatalf("Cache-Control = %q", cc)
			}
		})
	}
}

func TestSameOrigin(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	form := url.Values{"name": {"x"}, "type": {"rest"}}
	tests := []struct {
		name   string
		origin string
		want   int
	}{
		{"cross site", "https://evil.test", http.StatusForbidden},
		{"no origin", "-", http.StatusForbidden},
		{"same origin", "", http.StatusSeeOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := e.do(req{method: http.MethodPost, path: "/web/project/new", role: domain.RoleModifier, form: form, origin: tt.origin})
			assertStatus(t, w.Code, tt.want)
		})
	}
}

func TestProjects(t *testing.T) {
	t.Parallel()
	e := newEnv(t)

	t.Run("overview lists projects with counts", func(t *testing.T) {
		w := e.do(req{path: "/web", role: domain.RoleReader})
		assertStatus(t, w.Code, http.StatusOK)
		body := w.Body.String()
		assertContains(t, body, "payments-gateway", "core-banking", "2 mocked · 1 forwarded", `/castlemock/web/rest/project/`+e.seed.rest)
		if strings.Contains(body, "New project") {
			t.Fatal("reader sees New project")
		}
	})
	t.Run("modifier sees actions", func(t *testing.T) {
		w := e.do(req{path: "/web", role: domain.RoleModifier})
		assertContains(t, w.Body.String(), "New project", `name="ids"`)
	})
	t.Run("filter fragment", func(t *testing.T) {
		w := e.do(req{path: "/web?q=core&type=soap", role: domain.RoleReader, htmx: true, target: "project-table"})
		assertStatus(t, w.Code, http.StatusOK)
		body := w.Body.String()
		assertContains(t, body, "core-banking")
		if strings.Contains(body, "payments-gateway") || strings.Contains(body, "<html") {
			t.Fatalf("unexpected fragment: %s", body)
		}
	})
	t.Run("reader cannot open the new project dialog", func(t *testing.T) {
		w := e.do(req{path: "/web/project/new", role: domain.RoleReader})
		assertStatus(t, w.Code, http.StatusForbidden)
	})
	t.Run("new project dialog fragment and standalone page", func(t *testing.T) {
		w := e.do(req{path: "/web/project/new", role: domain.RoleModifier, htmx: true, target: "modal-root"})
		assertStatus(t, w.Code, http.StatusOK)
		assertContains(t, w.Body.String(), "<dialog", "Create project")
		w = e.do(req{path: "/web/project/new?tab=import", role: domain.RoleModifier})
		assertContains(t, w.Body.String(), "<html", "modal-page", `enctype="multipart/form-data"`)
	})
	t.Run("create project validation error", func(t *testing.T) {
		w := e.do(req{method: http.MethodPost, path: "/web/project/new", role: domain.RoleModifier, form: url.Values{"name": {""}, "type": {"rest"}}, htmx: true, target: "modal-root"})
		assertStatus(t, w.Code, http.StatusUnprocessableEntity)
		assertContains(t, w.Body.String(), "alert--error")
	})
	t.Run("create project redirects with HX-Location", func(t *testing.T) {
		w := e.do(req{method: http.MethodPost, path: "/web/project/new", role: domain.RoleModifier, form: url.Values{"name": {"orders"}, "type": {"rest"}}, htmx: true, target: "modal-root"})
		assertStatus(t, w.Code, http.StatusOK)
		if loc := w.Header().Get("HX-Location"); !strings.HasPrefix(loc, "/castlemock/web/rest/project/") {
			t.Fatalf("HX-Location = %q", loc)
		}
	})
	t.Run("bulk delete confirm and delete", func(t *testing.T) {
		w := e.do(req{path: "/web/project/delete?ids=" + e.seed.soap, role: domain.RoleModifier, htmx: true, target: "modal-root"})
		assertStatus(t, w.Code, http.StatusOK)
		assertContains(t, w.Body.String(), "core-banking", "Delete project", `value="`+e.seed.soap+`"`)
		w = e.do(req{method: http.MethodPost, path: "/web/project/delete", role: domain.RoleModifier, form: url.Values{"ids": {e.seed.soap}}})
		assertStatus(t, w.Code, http.StatusSeeOther)
		if _, err := e.svc.SoapProject(t.Context(), e.seed.soap); err == nil {
			t.Fatal("project not deleted")
		}
	})
}

func assertStatus(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func assertContains(t *testing.T, body string, frags ...string) {
	t.Helper()
	for _, f := range frags {
		if !strings.Contains(body, f) {
			t.Fatalf("body does not contain %q", f)
		}
	}
}

func TestImportProject(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	upload := func(role domain.Role, filename, content string) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		_ = mw.WriteField("type", "rest")
		if filename != "" {
			fw, _ := mw.CreateFormFile("file", filename)
			_, _ = fw.Write([]byte(content))
		}
		_ = mw.Close()
		r := httptest.NewRequest(http.MethodPost, testCtx+"/web/project/import", &buf)
		r.Host = "mock.test"
		r.Header.Set("Origin", "http://mock.test")
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.Header.Set("HX-Request", "true")
		tok, _ := e.auth.Token(e.users[role].ID)
		r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok})
		w := httptest.NewRecorder()
		e.engine.ServeHTTP(w, r)
		return w
	}
	t.Run("missing file", func(t *testing.T) {
		w := upload(domain.RoleModifier, "", "")
		assertStatus(t, w.Code, http.StatusUnprocessableEntity)
		assertContains(t, w.Body.String(), "choose a file")
	})
	t.Run("unrecognised file", func(t *testing.T) {
		w := upload(domain.RoleModifier, "x.txt", "not a project")
		assertStatus(t, w.Code, http.StatusUnprocessableEntity)
		assertContains(t, w.Body.String(), "<dialog", "alert--error")
	})
	t.Run("too large", func(t *testing.T) {
		w := upload(domain.RoleModifier, "big.json", strings.Repeat("x", 2<<20))
		assertStatus(t, w.Code, http.StatusUnprocessableEntity)
	})
	t.Run("reader is forbidden", func(t *testing.T) {
		w := upload(domain.RoleReader, "x.json", "{}")
		assertStatus(t, w.Code, http.StatusForbidden)
	})
}
