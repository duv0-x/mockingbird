// Package auth authenticates admin API calls and UI sessions (SPEC.md §5.1):
// JWT cookie "token", Bearer token and HTTP Basic, with bcrypt-hashed users.
//
// Owner: T4. Consumers: internal/api (T4), internal/web (T5a/T5b).
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

const CookieName = "token"

const (
	issuer      = "castlemock"
	tokenMaxAge = 7 * 24 * time.Hour
	keySize     = 64
)

type Options struct {
	KeyFile string // persisted HS256 key; generated on first start
	// Now overrides the clock (tests); nil means time.Now.
	Now func() time.Time
}

type Service struct {
	users storage.UserRepository
	opts  Options
	key   []byte
}

// New loads the HS256 key from opts.KeyFile, creating it (0600, parent dir
// 0700) on first start. CastleMock generated an RSA key at every start, so
// every restart logged everybody out (D-1).
func New(users storage.UserRepository, opts Options) (*Service, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	key, err := loadOrCreateKey(opts.KeyFile)
	if err != nil {
		return nil, err
	}
	return &Service{users: users, opts: opts, key: key}, nil
}

func loadOrCreateKey(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("auth: jwt key file not configured")
	}
	data, err := os.ReadFile(path)
	if err == nil {
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("decode jwt key %s: %w", path, err)
		}
		if len(key) < 32 {
			return nil, fmt.Errorf("jwt key %s: too short (%d bytes, want >= 32)", path, len(key))
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read jwt key %s: %w", path, err)
	}
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate jwt key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create jwt key dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".jwt-key-*")
	if err != nil {
		return nil, fmt.Errorf("create jwt key: %w", err)
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("chmod jwt key: %w", err)
	}
	if _, err := tmp.WriteString(base64.StdEncoding.EncodeToString(key) + "\n"); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("write jwt key: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("sync jwt key: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close jwt key: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return nil, fmt.Errorf("install jwt key: %w", err)
	}
	return key, nil
}

type claims struct {
	UserID string `json:"userId"`
	jwt.RegisteredClaims
}

// Token signs a session token for userID (JWTEncoderDecoder.createToken).
func (s *Service) Token(userID string) (string, error) {
	now := s.opts.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenMaxAge)),
		},
	})
	signed, err := t.SignedString(s.key)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// userIDFromToken verifies signature, issuer and expiry.
func (s *Service) userIDFromToken(token string) (string, bool) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return s.key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.opts.Now),
	)
	if err != nil || c.UserID == "" {
		return "", false
	}
	return c.UserID, true
}

// Login verifies credentials of an ACTIVE user and returns a signed token.
// The username lookup is case-insensitive.
func (s *Service) Login(ctx context.Context, username, password string) (string, domain.User, error) {
	u, err := s.authenticate(ctx, username, password)
	if err != nil {
		return "", domain.User{}, err
	}
	token, err := s.Token(u.ID)
	if err != nil {
		return "", domain.User{}, err
	}
	return token, u.Redacted(), nil
}

func (s *Service) authenticate(ctx context.Context, username, password string) (domain.User, error) {
	if username == "" {
		return domain.User{}, ErrInvalidCredentials
	}
	u, err := s.users.GetByUsername(ctx, username)
	if errors.Is(err, storage.ErrNotFound) {
		return domain.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("load user: %w", err)
	}
	// Spring's DaoAuthenticationProvider rejects disabled (INACTIVE) and
	// locked (LOCKED) accounts (UserDetailSecurityService).
	if u.Status != domain.UserActive || !CheckPassword(u.Password, password) {
		return domain.User{}, ErrInvalidCredentials
	}
	return u, nil
}

type invalidSessionKey struct{}

const ginPrincipalKey = "mockingbird.principal"

// Middleware resolves the principal (cookie, Bearer, Basic) and stores it in
// the request context. It never rejects; RequireAPI/RequireUI enforce access.
//
// Order (SecurityInterceptor, RestSecurityConfig): cookie "token"; then an
// Authorization header with exactly two space-separated parts whose second
// part is a valid JWT (any scheme word); then HTTP Basic. A valid token for a
// deleted or non-ACTIVE user clears the cookie and leaves the request without
// principal, so protected API calls answer 401 instead of CastleMock's 302 (D-5).
func (s *Service) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userID, ok := "", false
		if ck, err := c.Request.Cookie(CookieName); err == nil && ck.Value != "" {
			userID, ok = s.userIDFromToken(ck.Value)
		}
		if !ok {
			if parts := strings.Split(c.GetHeader("Authorization"), " "); len(parts) == 2 {
				userID, ok = s.userIDFromToken(parts[1])
			}
		}
		if ok {
			u, err := s.users.Get(ctx, userID)
			if err != nil || u.Status != domain.UserActive {
				ClearSessionCookie(c)
				c.Request = c.Request.WithContext(context.WithValue(ctx, invalidSessionKey{}, true))
				c.Next()
				return
			}
			setPrincipal(c, u)
			c.Next()
			return
		}
		if user, pass, hasBasic := c.Request.BasicAuth(); hasBasic {
			if u, err := s.authenticate(ctx, user, pass); err == nil {
				setPrincipal(c, u)
			}
		}
		c.Next()
	}
}

func setPrincipal(c *gin.Context, u domain.User) {
	u = u.Redacted()
	c.Set(ginPrincipalKey, u)
	c.Request = c.Request.WithContext(WithPrincipal(c.Request.Context(), u))
}

// RequireAPI aborts with 401 (no principal) or 403 (allowed returns false).
// The body is Spring Boot's error JSON; no WWW-Authenticate header is sent
// (NoPopupBasicAuthenticationEntryPoint).
func RequireAPI(allowed func(domain.Role) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := PrincipalFrom(c)
		if !ok {
			abortJSON(c, http.StatusUnauthorized)
			return
		}
		if allowed != nil && !allowed(u.Role) {
			abortJSON(c, http.StatusForbidden)
			return
		}
		c.Next()
	}
}

// RequireUI redirects to the login page without a principal and renders a 403
// page when allowed returns false. htmx requests get an HX-Redirect header
// instead of a 302, so the whole page navigates to the login form.
func RequireUI(loginPath string, allowed func(domain.Role) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := PrincipalFrom(c)
		if !ok {
			if c.GetHeader("HX-Request") == "true" {
				c.Header("HX-Redirect", loginPath)
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			c.Redirect(http.StatusFound, loginPath)
			c.Abort()
			return
		}
		if allowed != nil && !allowed(u.Role) {
			c.Data(http.StatusForbidden, "text/html; charset=utf-8",
				[]byte("<!doctype html><title>Forbidden</title><h1>403 Forbidden</h1><p>You do not have permission to access this page.</p>"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// ErrorBody is Spring Boot's default error JSON (SPEC.md §4.8).
type ErrorBody struct {
	Timestamp domain.Time `json:"timestamp"`
	Status    int         `json:"status"`
	Error     string      `json:"error"`
	Path      string      `json:"path"`
}

// NewErrorBody builds the error JSON for the current request.
func NewErrorBody(c *gin.Context, status int) ErrorBody {
	return ErrorBody{Timestamp: domain.Now(), Status: status, Error: http.StatusText(status), Path: c.Request.URL.Path}
}

func abortJSON(c *gin.Context, status int) {
	c.AbortWithStatusJSON(status, NewErrorBody(c, status))
}

// SessionInvalidated reports whether Middleware dropped a token of a deleted
// or non-ACTIVE user on this request.
func SessionInvalidated(ctx context.Context) bool {
	if c, ok := ctx.(*gin.Context); ok {
		ctx = c.Request.Context()
	}
	v, _ := ctx.Value(invalidSessionKey{}).(bool)
	return v
}

// SetSessionCookie writes token=<jwt>; Max-Age=604800; Path=/; HttpOnly; SameSite=Lax.
func SetSessionCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(tokenMaxAge / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie expires the token and JSESSIONID cookies.
func ClearSessionCookie(c *gin.Context) {
	for _, name := range []string{CookieName, "JSESSIONID"} {
		http.SetCookie(c.Writer, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	}
}

type principalKey struct{}

func WithPrincipal(ctx context.Context, u domain.User) context.Context {
	return context.WithValue(ctx, principalKey{}, u)
}

// PrincipalFrom returns the authenticated user (password redacted). It
// accepts a *gin.Context as well as a request context.
func PrincipalFrom(ctx context.Context) (domain.User, bool) {
	if c, ok := ctx.(*gin.Context); ok {
		if v, exists := c.Get(ginPrincipalKey); exists {
			u, ok := v.(domain.User)
			return u, ok
		}
		if c.Request == nil {
			return domain.User{}, false
		}
		ctx = c.Request.Context()
	}
	u, ok := ctx.Value(principalKey{}).(domain.User)
	return u, ok
}

// HashPassword returns a bcrypt hash with Spring's default cost (10).
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(h), nil
}

// CheckPassword verifies a bcrypt hash; Spring's $2a$ hashes are accepted.
func CheckPassword(hash, password string) bool {
	return hash != "" && bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
