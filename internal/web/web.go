// Package web is the server-rendered admin UI: templ components + htmx,
// served by gin under {ctx}/web/** with assets under {ctx}/static/** (SPEC.md §8).
//
// Owner: T5a (layout, shared components, login, projects and REST screens)
// and T5b (SOAP, events, users, profile, system). Visual design comes from
// design/ (DESIGN.md, mockups); do not invent styles. Run `make generate` from
// the repo root after editing a .templ file; *_templ.go files are committed.
//
// Conventions shared by every screen:
//   - Full pages for normal and boosted navigation; fragments only when htmx
//     targets a known element (#modal-root, a table body, the tree).
//   - Dialogs are GET .../<name> rendered into #modal-root (or as a standalone
//     page on a direct visit) and POST back to the same URL. Success answers
//     HX-Location (htmx) or 303 (plain form) plus a flash toast; validation
//     errors re-render the dialog with 422.
//   - Mutations are POST and pass the same-origin check (sameOrigin).
package web

import (
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/config"
	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/service"
	"github.com/duv0-x/mockingbird/internal/storage"
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

// routes are the router groups handlers register on. read requires any
// signed-in user, modify MODIFIER or ADMIN, admin ADMIN. All of them run the
// same-origin check on unsafe methods.
type routes struct {
	public *gin.RouterGroup
	read   *gin.RouterGroup
	modify *gin.RouterGroup
	admin  *gin.RouterGroup
}

// Register mounts the UI on g, the router group of the context path. The
// auth middleware is already installed on g.
func Register(g *gin.RouterGroup, deps Deps) {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	u := &ui{deps: deps}
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	g.Group("/static", cacheStatic).StaticFS("/", http.FS(static))
	g.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, deps.Config.ContextPath+"/web") })

	w := g.Group("/web", sameOrigin)
	login := u.loginPath()
	rt := routes{
		public: w,
		read:   w.Group("", auth.RequireUI(login, nil)),
		modify: w.Group("", auth.RequireUI(login, domain.Role.CanModify)),
		admin:  w.Group("", auth.RequireUI(login, domain.Role.IsAdmin)),
	}
	u.registerSession(rt)
	u.registerProjects(rt)
	u.registerRest(rt)
	// T5b screens.
	u.registerSoap(rt)
	u.registerEvents(rt)
	u.registerUsers(rt)
	u.registerProfile(rt)
	u.registerSystem(rt)
}

func cacheStatic(c *gin.Context) {
	if c.Query("v") != "" {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "no-cache")
	}
}

// sameOrigin rejects cross-site form posts (SPEC §8.1): unsafe methods must
// carry an Origin (or, failing that, a Referer) whose host is this server.
func sameOrigin(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		c.Next()
		return
	}
	src := c.GetHeader("Origin")
	if src == "" || src == "null" {
		src = c.GetHeader("Referer")
	}
	if src == "" || !sameHost(src, c.Request) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	c.Next()
}

func sameHost(raw string, r *http.Request) bool {
	o, err := url.Parse(raw)
	if err != nil || o.Host == "" {
		return false
	}
	if strings.EqualFold(o.Host, r.Host) {
		return true
	}
	fwd := r.Header.Get("X-Forwarded-Host")
	return fwd != "" && strings.EqualFold(o.Host, strings.TrimSpace(strings.Split(fwd, ",")[0]))
}

func (u *ui) loginPath() string { return u.deps.Config.ContextPath + "/web/login" }

// url prefixes a UI path with the context path.
func (u *ui) url(path string) string { return u.deps.Config.ContextPath + path }

// page builds the layout model for the current request.
func (u *ui) page(c *gin.Context, title, nav string, crumbs ...views.Crumb) views.Page {
	p := views.Page{
		Title:       title,
		Version:     u.deps.Version,
		ContextPath: u.deps.Config.ContextPath,
		Nav:         nav,
		Crumbs:      crumbs,
	}
	if usr, ok := auth.PrincipalFrom(c); ok {
		p.User = &usr
	}
	p.Flash = takeFlash(c)
	return p
}

// withProjects fills the sidebar's project list (pages outside a project).
func (u *ui) withProjects(c *gin.Context, p views.Page) views.Page {
	projects, err := u.deps.Service.Projects(c)
	if err != nil {
		u.deps.Logger.Warn("list projects for sidebar", "err", err)
	}
	p.Projects = projects
	return p
}

func render(c *gin.Context, status int, comp templ.Component) {
	c.Status(status)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	if err := comp.Render(c.Request.Context(), c.Writer); err != nil {
		_ = c.Error(err)
	}
}

func isHTMX(c *gin.Context) bool { return c.GetHeader("HX-Request") == "true" }

// targets reports whether htmx asked for a fragment swapped into #id.
func targets(c *gin.Context, id string) bool {
	return isHTMX(c) && c.GetHeader("HX-Target") == id
}

// redirect navigates after a successful mutation: HX-Location for htmx
// (boosted navigation with history), 303 See Other for plain forms.
func redirect(c *gin.Context, to string) {
	if isHTMX(c) {
		c.Header("HX-Location", to)
		c.Status(http.StatusOK)
		return
	}
	c.Redirect(http.StatusSeeOther, to)
}

// renderModal answers a dialog: the fragment for htmx, otherwise a page with
// the dialog rendered standalone.
func (u *ui) renderModal(c *gin.Context, status int, m views.Modal, dialog func(views.Modal) templ.Component, back string, title string) {
	if isHTMX(c) {
		render(c, status, dialog(m))
		return
	}
	m.Standalone = true
	m.CancelHref = back
	p := u.withProjects(c, u.page(c, title, views.NavProjects))
	render(c, status, views.ModalPage(p, dialog(m)))
}

// ---------- Flash toasts ----------

const flashCookie = "mb_flash"

func setFlash(c *gin.Context, typ, msg string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     flashCookie,
		Value:    url.QueryEscape(typ + ":" + msg),
		Path:     "/",
		MaxAge:   60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func takeFlash(c *gin.Context) *views.Flash {
	ck, err := c.Request.Cookie(flashCookie)
	if err != nil || ck.Value == "" {
		return nil
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: flashCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	raw, err := url.QueryUnescape(ck.Value)
	if err != nil {
		return nil
	}
	typ, msg, ok := strings.Cut(raw, ":")
	if !ok || msg == "" {
		return nil
	}
	switch typ {
	case "success", "info", "warning", "error":
	default:
		typ = "info"
	}
	return &views.Flash{Type: typ, Msg: msg}
}

// ---------- Errors ----------

// fail renders the error page for err (404 for storage.ErrNotFound).
func (u *ui) fail(c *gin.Context, err error) {
	status, title, msg := http.StatusInternalServerError, "Something went wrong", "The server could not complete the request."
	var verr *service.ValidationError
	switch {
	case errors.Is(err, storage.ErrNotFound):
		status, title, msg = http.StatusNotFound, "Not found", "The page or the item you are looking for does not exist."
	case errors.As(err, &verr):
		status, title, msg = http.StatusBadRequest, "Invalid request", verr.Error()
	case errors.Is(err, storage.ErrConflict):
		status, title, msg = http.StatusConflict, "Conflict", "The item conflicts with an existing one."
	default:
		u.deps.Logger.Error("ui request failed", "path", c.Request.URL.Path, "err", err)
	}
	if isHTMX(c) && c.GetHeader("HX-Boosted") != "true" && c.GetHeader("HX-Target") != "" {
		c.Header("HX-Trigger", toastTrigger("error", title+": "+msg))
		c.Status(status)
		return
	}
	p := u.withProjects(c, u.page(c, title, views.NavProjects))
	render(c, status, views.ErrorPage(p, status, title, msg))
}

// formError maps a service error to a message shown inside a form. ok is
// false for errors that are not the user's fault (render with u.fail).
func formError(err error) (msg string, status int, ok bool) {
	var verr *service.ValidationError
	switch {
	case errors.As(err, &verr):
		return verr.Error(), http.StatusUnprocessableEntity, true
	case errors.Is(err, storage.ErrConflict):
		return "An item with that name or id already exists.", http.StatusUnprocessableEntity, true
	}
	return "", 0, false
}

func toastTrigger(typ, msg string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", " ")
	return `{"toast":{"type":"` + typ + `","msg":"` + r.Replace(msg) + `"}}`
}

// requestBase is scheme://host of the current request, honouring
// X-Forwarded-Proto/Host from a reverse proxy.
func requestBase(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if p := c.GetHeader("X-Forwarded-Proto"); p != "" {
		scheme = strings.TrimSpace(strings.Split(p, ",")[0])
	}
	host := c.Request.Host
	if h := c.GetHeader("X-Forwarded-Host"); h != "" {
		host = strings.TrimSpace(strings.Split(h, ",")[0])
	}
	return scheme + "://" + host
}

// mockURL is the absolute mock address of a path under {ctx}/mock (SPEC §8.1).
func (u *ui) mockURL(c *gin.Context, path string) string {
	return requestBase(c) + u.deps.Config.ContextPath + "/mock" + path
}

// formIDs returns the bulk selection posted as ids=…
func formIDs(c *gin.Context) []string {
	_ = c.Request.ParseMultipartForm(32 << 20)
	var ids []string
	for _, id := range c.Request.Form["ids"] {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func formBool(c *gin.Context, name string) bool {
	v := c.PostForm(name)
	return v == "true" || v == "on" || v == "1"
}
