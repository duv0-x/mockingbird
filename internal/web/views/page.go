package views

import (
	"github.com/a-h/templ"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// Nav sections of the sidebar; Page.Nav marks the active one.
const (
	NavProjects = "projects"
	NavEvents   = "events"
	NavUsers    = "users"
	NavSystem   = "system"
	NavProfile  = "profile"
)

// Page carries what the layout needs on every screen.
type Page struct {
	Title       string
	Version     string
	ContextPath string
	// User is the signed-in principal; nil on public pages.
	User *domain.User
	Nav  string
	// Crumbs is the breadcrumb trail; the last crumb is the current page.
	Crumbs []Crumb
	// Tree replaces the sidebar's project list with the current project's
	// context tree.
	Tree *TreeNode
	// Projects feeds the sidebar list when Tree is nil.
	Projects []domain.OverviewProject
	// Flash is a one-shot toast carried over a redirect.
	Flash *Flash
}

type Crumb struct {
	Label string
	Href  string
	Mono  bool
}

type Flash struct {
	Type string // success | info | warning | error
	Msg  string
}

// Asset returns the URL of a static file under the context path.
func (p Page) Asset(path string) string {
	return p.ContextPath + "/static/" + path + "?v=" + p.Version
}

// URL prefixes an absolute UI path with the context path.
func (p Page) URL(path string) string { return p.ContextPath + path }

// SafeURL is URL as a templ.SafeURL for href/action attributes.
func (p Page) SafeURL(path string) templ.SafeURL { return templ.SafeURL(p.ContextPath + path) }

func (p Page) CanModify() bool { return p.User != nil && p.User.Role.CanModify() }
func (p Page) IsAdmin() bool   { return p.User != nil && p.User.Role.IsAdmin() }

// TreeNode is a row of the sidebar context tree. Children are rendered when
// Loaded; otherwise ChildrenURL lazy-loads them the first time the node opens.
type TreeNode struct {
	Label       string
	Href        string
	Meta        string            // short right-aligned label ("app", "REST", ...)
	Method      domain.HTTPMethod // method badge for REST methods
	Mode        string            // RestMethodStatus / SoapOperationStatus for the mode dot
	Mono        bool
	Current     bool
	Open        bool
	Loaded      bool
	ChildrenURL string
	Children    []TreeNode
}

// Leaf reports whether the node never has children.
func (n TreeNode) Leaf() bool { return !n.Loaded && n.ChildrenURL == "" }
