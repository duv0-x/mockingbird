package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/service/servicetest"
)

func newAuth(t *testing.T, keyFile string, now func() time.Time) (*auth.Service, *servicetest.MemStore) {
	t.Helper()
	store := servicetest.NewMemStore()
	hash, err := auth.HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []domain.User{
		{ID: "admin1", Username: "Admin", Password: hash, Status: domain.UserActive, Role: domain.RoleAdmin},
		{ID: "reader", Username: "reader", Password: hash, Status: domain.UserActive, Role: domain.RoleReader},
		{ID: "locked", Username: "locked", Password: hash, Status: domain.UserLocked, Role: domain.RoleAdmin},
	} {
		_ = store.Users().Save(context.Background(), u)
	}
	a, err := auth.New(store.Users(), auth.Options{KeyFile: keyFile, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	return a, store
}

func TestKeyIsPersisted(t *testing.T) {
	t.Parallel()
	keyFile := filepath.Join(t.TempDir(), "secrets", "jwt.key")
	a1, _ := newAuth(t, keyFile, nil)
	token, err := a1.Token("admin1")
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("key mode %v", st.Mode().Perm())
	}
	if dir, _ := os.Stat(filepath.Dir(keyFile)); dir.Mode().Perm() != 0o700 {
		t.Fatalf("secrets dir mode %v", dir.Mode().Perm())
	}
	// A restart reads the same key, so the session survives (D-1).
	a2, _ := newAuth(t, keyFile, nil)
	if code := call(a2, func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }); code != http.StatusOK {
		t.Fatalf("token rejected after restart: %d", code)
	}
}

func TestBadKeyFile(t *testing.T) {
	t.Parallel()
	keyFile := filepath.Join(t.TempDir(), "jwt.key")
	if err := os.WriteFile(keyFile, []byte("c2hvcnQ=\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.New(servicetest.NewMemStore().Users(), auth.Options{KeyFile: keyFile}); err == nil {
		t.Fatal("short key accepted")
	}
}

func TestLogin(t *testing.T) {
	t.Parallel()
	a, _ := newAuth(t, filepath.Join(t.TempDir(), "k"), nil)
	tests := []struct {
		user, pass string
		ok         bool
	}{
		{"admin", "secret", true},
		{"ADMIN", "secret", true},
		{"admin", "wrong", false},
		{"locked", "secret", false},
		{"ghost", "secret", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.user+"/"+tt.pass, func(t *testing.T) {
			token, u, err := a.Login(context.Background(), tt.user, tt.pass)
			if !tt.ok {
				if !errors.Is(err, auth.ErrInvalidCredentials) {
					t.Fatalf("want ErrInvalidCredentials, got %v", err)
				}
				return
			}
			if err != nil || token == "" || u.ID != "admin1" || u.Password != "" {
				t.Fatalf("login: %v %q %+v", err, token, u)
			}
		})
	}
}

// call runs the middleware plus RequireAPI(CanRead) and returns the status.
func call(a *auth.Service, opt func(*http.Request)) int {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/p", a.Middleware(), auth.RequireAPI(domain.Role.CanRead), func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	opt(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestTokenValidation(t *testing.T) {
	t.Parallel()
	keyFile := filepath.Join(t.TempDir(), "k")
	past := func() time.Time { return time.Now().Add(-8 * 24 * time.Hour) }
	old, _ := newAuth(t, keyFile, past)
	expired, err := old.Token("admin1")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := newAuth(t, keyFile, nil)
	other, _ := newAuth(t, filepath.Join(t.TempDir(), "other"), nil)
	foreign, _ := other.Token("admin1")
	valid, _ := a.Token("admin1")
	lockedToken, _ := a.Token("locked")
	tests := []struct {
		name  string
		token string
		want  int
	}{
		{"valid", valid, 200},
		{"expired", expired, 401},
		{"signed with another key", foreign, 401},
		{"user not active", lockedToken, 401},
		{"garbage", "x.y.z", 401},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := call(a, func(r *http.Request) { r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tt.token}) }); got != tt.want {
				t.Fatalf("cookie: status %d, want %d", got, tt.want)
			}
		})
	}
}

func TestInvalidCookieFallsBackToHeader(t *testing.T) {
	t.Parallel()
	a, _ := newAuth(t, filepath.Join(t.TempDir(), "k"), nil)
	valid, _ := a.Token("reader")
	code := call(a, func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "stale"})
		r.Header.Set("Authorization", "Bearer "+valid)
	})
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
}

func TestRequireAPIRoles(t *testing.T) {
	t.Parallel()
	a, _ := newAuth(t, filepath.Join(t.TempDir(), "k"), nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin", a.Middleware(), auth.RequireAPI(domain.Role.IsAdmin), func(c *gin.Context) {
		u, ok := auth.PrincipalFrom(c)
		if !ok || u.Password != "" {
			c.Status(http.StatusInternalServerError)
			return
		}
		if _, ok := auth.PrincipalFrom(c.Request.Context()); !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.String(http.StatusOK, u.Username)
	})
	tests := []struct {
		user string
		want int
	}{{"admin", 200}, {"reader", 403}}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.SetBasicAuth(tt.user, "secret")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tt.want {
			t.Fatalf("%s: status %d, want %d", tt.user, w.Code, tt.want)
		}
	}
}

func TestRequireUI(t *testing.T) {
	t.Parallel()
	a, store := newAuth(t, filepath.Join(t.TempDir(), "k"), nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/web/user", a.Middleware(), auth.RequireUI("/castlemock/web/login", domain.Role.IsAdmin), func(c *gin.Context) { c.Status(http.StatusOK) })
	reader, _ := a.Token("reader")
	admin, _ := a.Token("admin1")
	tests := []struct {
		name     string
		token    string
		htmx     bool
		want     int
		location string
		hx       string
	}{
		{"anonymous", "", false, http.StatusFound, "/castlemock/web/login", ""},
		{"anonymous htmx", "", true, http.StatusUnauthorized, "", "/castlemock/web/login"},
		{"reader", reader, false, http.StatusForbidden, "", ""},
		{"admin", admin, false, http.StatusOK, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/web/user", nil)
			if tt.token != "" {
				req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tt.token})
			}
			if tt.htmx {
				req.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.want || w.Header().Get("Location") != tt.location || w.Header().Get("HX-Redirect") != tt.hx {
				t.Fatalf("status %d location %q hx %q", w.Code, w.Header().Get("Location"), w.Header().Get("HX-Redirect"))
			}
		})
	}

	t.Run("deleted user is redirected and its cookie cleared", func(t *testing.T) {
		tok, _ := a.Token("reader")
		if _, err := store.Users().Delete(context.Background(), "reader"); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/web/user", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusFound || w.Header().Get("Set-Cookie") == "" {
			t.Fatalf("status %d set-cookie %q", w.Code, w.Header().Get("Set-Cookie"))
		}
	})
}

func TestPasswordHashing(t *testing.T) {
	t.Parallel()
	// Spring's BCryptPasswordEncoder writes "$2a$10$" hashes; ours must look the same.
	h, err := auth.HashPassword("pw")
	if err != nil || h[:7] != "$2a$10$" {
		t.Fatalf("hash %q err %v", h, err)
	}
	tests := []struct {
		hash, pass string
		want       bool
	}{
		{h, "pw", true},
		{h, "PW", false},
		{"", "", false},
		{"not-a-hash", "x", false},
	}
	for _, tt := range tests {
		if got := auth.CheckPassword(tt.hash, tt.pass); got != tt.want {
			t.Errorf("CheckPassword(%q, %q) = %v", tt.hash, tt.pass, got)
		}
	}
}
