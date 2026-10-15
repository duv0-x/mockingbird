// Package web is the server-rendered admin UI: templ components + htmx,
// served by gin under {ctx}/web/** with assets under {ctx}/static/** (SPEC.md §8).
//
// Owner: T5a (REST screens) and T5b (SOAP, events, users, profile, system,
// login, layout). Visual design comes from design/ (DESIGN.md, mockups);
// do not invent styles. Run `make generate` from the repo root after editing
// a .templ file; *_templ.go files are committed.
package web

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/config"
	"github.com/duv0-x/mockingbird/internal/service"
	"github.com/duv0-x/mockingbird/internal/web/views"
)

//go:embed static
var staticFS embed.FS

type Deps struct {
	Config  config.Config
	Service *service.Service
	Auth    *auth.Service
	Version string
	Logger  *slog.Logger
}

type ui struct {
	deps Deps
}

// Register mounts the UI on g, the router group of the context path. The
// auth middleware is already installed on g.
func Register(g *gin.RouterGroup, deps Deps) {
	u := &ui{deps: deps}
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	g.StaticFS("/static", http.FS(static))
	g.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, deps.Config.ContextPath+"/web") })
	g.GET("/web", u.placeholder)
}

func (u *ui) page(title string) views.Page {
	return views.Page{Title: title, Version: u.deps.Version, ContextPath: u.deps.Config.ContextPath}
}

func (u *ui) placeholder(c *gin.Context) {
	render(c, http.StatusOK, views.Placeholder(u.page("mockingbird")))
}

func render(c *gin.Context, status int, comp templ.Component) {
	c.Status(status)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := comp.Render(c.Request.Context(), c.Writer); err != nil {
		_ = c.Error(err)
	}
}
