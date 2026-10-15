package views

// Page carries what the layout needs on every screen.
type Page struct {
	Title       string
	Version     string
	ContextPath string
}

// Asset returns the URL of a static file under the context path.
func (p Page) Asset(path string) string {
	return p.ContextPath + "/static/" + path + "?v=" + p.Version
}
