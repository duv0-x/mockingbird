// Package auth authenticates admin API calls and UI sessions (SPEC.md §5.1):
// JWT cookie "token", Bearer token and HTTP Basic, with bcrypt-hashed users.
//
// Owner: T4. Consumers: internal/api (T4), internal/web (T5a/T5b).
package auth

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

const CookieName = "token"

type Options struct {
	KeyFile string // persisted HS256 key; generated on first start
}

type Service struct {
	users storage.UserRepository
	opts  Options
}

func New(users storage.UserRepository, opts Options) (*Service, error) {
	return &Service{users: users, opts: opts}, nil
}

// Login verifies credentials of an ACTIVE user and returns a signed token.
func (s *Service) Login(ctx context.Context, username, password string) (string, domain.User, error) {
	return "", domain.User{}, errors.New("auth: not implemented (T4)")
}

// Middleware resolves the principal (cookie, Bearer, Basic) and stores it in
// the request context. It never rejects; RequireAPI/RequireUI enforce access.
func (s *Service) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

// RequireAPI aborts with 401 (no principal) or 403 (allowed returns false).
func RequireAPI(allowed func(domain.Role) bool) gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

// RequireUI redirects to the login page without a principal and renders a 403
// page when allowed returns false.
func RequireUI(loginPath string, allowed func(domain.Role) bool) gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

// SetSessionCookie writes token=<jwt>; Max-Age=604800; Path=/; HttpOnly.
func SetSessionCookie(c *gin.Context, token string) {}

// ClearSessionCookie expires the token and JSESSIONID cookies.
func ClearSessionCookie(c *gin.Context) {}

type principalKey struct{}

func WithPrincipal(ctx context.Context, u domain.User) context.Context {
	return context.WithValue(ctx, principalKey{}, u)
}

func PrincipalFrom(ctx context.Context) (domain.User, bool) {
	u, ok := ctx.Value(principalKey{}).(domain.User)
	return u, ok
}

func HashPassword(password string) (string, error) {
	return "", errors.New("auth: not implemented (T4)")
}

func CheckPassword(hash, password string) bool { return false }
