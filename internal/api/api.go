// Package api implements the CastleMock-compatible JSON admin API under
// {ctx}/api/rest/**, plus {ctx}/doc/api/rest and {ctx}/resource/soap/** (SPEC.md §5).
// Handlers translate HTTP <-> internal/service; no business logic here.
//
// Owner: T4.
package api

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/config"
	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/service"
	"github.com/duv0-x/mockingbird/internal/storage"
)

type Deps struct {
	Config  config.Config
	Service *service.Service
	Auth    *auth.Service
	Logger  *slog.Logger
}

// Access is the role a route requires (SPEC.md §5.1).
type Access string

const (
	Public   Access = "public"
	Reader   Access = "R" // READER, MODIFIER or ADMIN
	Modifier Access = "M" // MODIFIER or ADMIN
	Admin    Access = "A" // ADMIN
)

func (a Access) allowed() func(domain.Role) bool {
	switch a {
	case Reader:
		return domain.Role.CanRead
	case Modifier:
		return domain.Role.CanModify
	case Admin:
		return domain.Role.IsAdmin
	}
	return nil
}

// Route is one admin API endpoint. Path is relative to the context path.
type Route struct {
	Method  string
	Path    string
	Access  Access
	Summary string
	handler func(*handlers, *gin.Context)
}

// Register mounts the API on g, the router group of the context path. The
// auth middleware is already installed on g.
func Register(g *gin.RouterGroup, deps Deps) {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	h := &handlers{deps: deps}
	for _, r := range Routes() {
		chain := []gin.HandlerFunc{}
		if r.Access != Public {
			chain = append(chain, auth.RequireAPI(r.Access.allowed()))
		}
		handle := r.handler
		chain = append(chain, func(c *gin.Context) { handle(h, c) })
		g.Handle(r.Method, r.Path, chain...)
	}
	g.GET("/doc/api/rest", h.openAPI)
	g.GET("/resource/soap/project/:projectId/resource/:resourceId", h.soapResourceRaw)
}

type handlers struct {
	deps Deps
}

func (h *handlers) svc() *service.Service { return h.deps.Service }

// fail maps service errors to HTTP (SPEC.md §5.0, D-12): not found → 404
// with an empty body; conflict → 409, validation → 400, anything else → 500,
// all with Spring Boot's error JSON.
func (h *handlers) fail(c *gin.Context, err error) {
	var ve *service.ValidationError
	switch {
	case errors.Is(err, storage.ErrNotFound):
		c.Status(http.StatusNotFound)
	case errors.As(err, &ve):
		h.deps.Logger.Debug("admin api bad request", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, auth.NewErrorBody(c, http.StatusBadRequest))
	case errors.Is(err, storage.ErrConflict):
		h.deps.Logger.Debug("admin api conflict", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusConflict, auth.NewErrorBody(c, http.StatusConflict))
	default:
		h.deps.Logger.Warn("admin api failure", "method", c.Request.Method, "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusInternalServerError, auth.NewErrorBody(c, http.StatusInternalServerError))
	}
}

func (h *handlers) badRequest(c *gin.Context, err error) {
	h.deps.Logger.Debug("admin api bad request", "path", c.Request.URL.Path, "err", err)
	c.JSON(http.StatusBadRequest, auth.NewErrorBody(c, http.StatusBadRequest))
}

// bind decodes a JSON body; an empty body decodes to the zero value.
func (h *handlers) bind(c *gin.Context, v any) bool {
	if c.Request.ContentLength == 0 && c.Request.Body == http.NoBody {
		return true
	}
	if err := c.ShouldBindJSON(v); err != nil && !errors.Is(err, io.EOF) {
		h.badRequest(c, err)
		return false
	}
	return true
}

// respond writes v as JSON without HTML escaping (Jackson parity) or the error.
func respond[T any](h *handlers, c *gin.Context, v T, err error) {
	if err != nil {
		h.fail(c, err)
		return
	}
	c.PureJSON(http.StatusOK, v)
}

// respondEmpty answers 200 with an empty body (Spring's ResponseEntity<Void>).
func (h *handlers) respondEmpty(c *gin.Context, err error) {
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusOK)
	c.Writer.WriteHeaderNow()
}

// upload reads the multipart "file" part, limited to MOCKINGBIRD_UPLOAD_MAX_SIZE.
func (h *handlers) upload(c *gin.Context) (service.Upload, bool) {
	if max := h.deps.Config.UploadMaxBytes; max > 0 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
	}
	fh, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, auth.NewErrorBody(c, http.StatusRequestEntityTooLarge))
			return service.Upload{}, false
		}
		h.badRequest(c, err)
		return service.Upload{}, false
	}
	f, err := fh.Open()
	if err != nil {
		h.badRequest(c, err)
		return service.Upload{}, false
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		h.badRequest(c, err)
		return service.Upload{}, false
	}
	return service.Upload{Filename: fh.Filename, Data: data}, true
}

// formBool parses a multipart/query boolean; absent means false.
func formBool(c *gin.Context, name string) (bool, error) {
	v := c.PostForm(name)
	if v == "" {
		return false, nil
	}
	return strconv.ParseBool(v)
}

func principal(c *gin.Context) domain.User {
	u, _ := auth.PrincipalFrom(c)
	return u
}
