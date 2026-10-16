package web

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/service"
	"github.com/duv0-x/mockingbird/internal/web/views"
)

const projectsPerPage = 25

func (u *ui) registerProjects(rt routes) {
	rt.read.GET("", u.projects)
	rt.modify.GET("/project/new", u.newProjectForm)
	rt.modify.POST("/project/new", u.createProject)
	rt.modify.POST("/project/import", u.importProject)
	rt.modify.GET("/project/delete", u.deleteProjectsForm)
	rt.modify.POST("/project/delete", u.deleteProjects)
}

// projects is the project overview (ProjectOverview.js): searchable,
// paginated, with REST/SOAP filter and per-project mode counts.
func (u *ui) projects(c *gin.Context) {
	all, err := u.deps.Service.Projects(c)
	if err != nil {
		u.fail(c, err)
		return
	}
	v := views.Projects{Total: len(all), Query: strings.TrimSpace(c.Query("q")), Type: c.Query("type")}
	if v.Type != "rest" && v.Type != "soap" {
		v.Type = ""
	}
	rows := make([]views.ProjectRow, 0, len(all))
	for _, p := range all {
		row := views.ProjectRow{OverviewProject: p, Counts: map[string]int{}}
		u.projectCounts(c, &row)
		v.Endpoints += row.Endpoints
		v.Mocked += row.Counts[string(domain.RestMethodMocked)]
		if v.Type != "" && string(p.Type) != v.Type {
			continue
		}
		if v.Query != "" && !containsFold(p.Name, v.Query) && !containsFold(p.Description, v.Query) {
			continue
		}
		rows = append(rows, row)
	}
	v.Requests = u.recentRequests(c)
	v.Filtered = len(rows)
	v.Pages = max(1, (len(rows)+projectsPerPage-1)/projectsPerPage)
	v.Page, _ = strconv.Atoi(c.Query("page"))
	v.Page = min(max(v.Page, 1), v.Pages)
	start := (v.Page - 1) * projectsPerPage
	end := min(start+projectsPerPage, len(rows))
	v.Rows = rows[start:end]
	v.From, v.To = start+1, end

	p := u.page(c, "Projects", views.NavProjects, views.Crumb{Label: "Projects"})
	p.Projects = all
	if targets(c, "project-table") {
		render(c, http.StatusOK, views.ProjectTable(p, v))
		return
	}
	render(c, http.StatusOK, views.ProjectsPage(p, v))
}

// projectCounts sums the statusCount of the project's applications or ports.
func (u *ui) projectCounts(c *gin.Context, row *views.ProjectRow) {
	switch row.Type {
	case domain.ProjectTypeREST:
		p, err := u.deps.Service.RestProject(c, row.ID)
		if err != nil {
			return
		}
		for _, a := range p.Applications {
			for k, n := range a.StatusCount {
				row.Counts[string(k)] += n
				row.Endpoints += n
			}
		}
	case domain.ProjectTypeSOAP:
		p, err := u.deps.Service.SoapProject(c, row.ID)
		if err != nil {
			return
		}
		for _, po := range p.Ports {
			for k, n := range po.StatusCount {
				row.Counts[string(k)] += n
				row.Endpoints += n
			}
		}
	}
}

func (u *ui) recentRequests(c *gin.Context) int {
	events, err := u.deps.Service.Events(c)
	if err != nil {
		return 0
	}
	since := time.Now().Add(-24 * time.Hour)
	n := 0
	for _, e := range events {
		if e.StartDate.After(since) {
			n++
		}
	}
	return n
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// ---------- New / import ----------

func (u *ui) newProjectModal(v views.NewProject) views.NewProject {
	if v.Tab == "import" {
		v.Modal = views.Modal{Title: "New project", Size: "md", Action: u.url("/web/project/import"), Multipart: true, Submit: "Import project"}
	} else {
		v.Modal = views.Modal{Title: "New project", Size: "md", Action: u.url("/web/project/new"), Submit: "Create project"}
	}
	return v
}

func (u *ui) renderNewProject(c *gin.Context, status int, v views.NewProject) {
	v = u.newProjectModal(v)
	p := u.page(c, "New project", views.NavProjects)
	if isHTMX(c) {
		v.Modal.Tabs = views.NewProjectTabs(p, v.Tab)
	}
	u.renderModal(c, status, v.Modal, func(m views.Modal) templ.Component {
		v.Modal = m
		return views.NewProjectDialog(v)
	}, u.url("/web"), "New project")
}

func (u *ui) newProjectForm(c *gin.Context) {
	tab := c.Query("tab")
	if tab != "import" {
		tab = "blank"
	}
	u.renderNewProject(c, http.StatusOK, views.NewProject{Tab: tab, Type: c.Query("type")})
}

func (u *ui) createProject(c *gin.Context) {
	v := views.NewProject{Tab: "blank", Name: strings.TrimSpace(c.PostForm("name")), Description: strings.TrimSpace(c.PostForm("description")), Type: c.PostForm("type")}
	in := service.ProjectInput{Name: v.Name, Description: v.Description}
	var (
		id  string
		err error
	)
	if v.Type == "soap" {
		var sp domain.SoapProject
		sp, err = u.deps.Service.CreateSoapProject(c, in)
		id = sp.ID
	} else {
		v.Type = "rest"
		var rp domain.RestProject
		rp, err = u.deps.Service.CreateRestProject(c, in)
		id = rp.ID
	}
	if err != nil {
		u.newProjectError(c, v, err)
		return
	}
	setFlash(c, "success", "Project "+v.Name+" created")
	redirect(c, u.url("/web/"+v.Type+"/project/"+id))
}

func (u *ui) importProject(c *gin.Context) {
	v := views.NewProject{Tab: "import", Type: c.PostForm("type")}
	src, err := u.formUpload(c, "file")
	if err != nil {
		v.Error = err.Error()
		u.renderNewProject(c, http.StatusUnprocessableEntity, v)
		return
	}
	var id, name string
	if v.Type == "soap" {
		var sp domain.SoapProject
		sp, err = u.deps.Service.ImportSoapProject(c, src)
		id, name = sp.ID, sp.Name
	} else {
		v.Type = "rest"
		var rp domain.RestProject
		rp, err = u.deps.Service.ImportRestProject(c, src)
		id, name = rp.ID, rp.Name
	}
	if err != nil {
		u.newProjectError(c, v, err)
		return
	}
	setFlash(c, "success", "Project "+name+" imported")
	redirect(c, u.url("/web/"+v.Type+"/project/"+id))
}

func (u *ui) newProjectError(c *gin.Context, v views.NewProject, err error) {
	msg, status, ok := formError(err)
	if !ok {
		u.deps.Logger.Error("ui create/import project", "err", err)
		msg, status = "The project could not be saved: "+err.Error(), http.StatusUnprocessableEntity
	}
	v.Error = msg
	u.renderNewProject(c, status, v)
}

// formUpload reads an uploaded file into a service.Upload, enforcing
// MOCKINGBIRD_UPLOAD_MAX_SIZE like the API.
func (u *ui) formUpload(c *gin.Context, field string) (service.Upload, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, u.deps.Config.UploadMaxBytes+1<<20)
	fh, err := c.FormFile(field)
	if err != nil {
		return service.Upload{}, &service.ValidationError{Field: field, Message: "choose a file to upload (max " + strconv.FormatInt(u.deps.Config.UploadMaxBytes>>20, 10) + " MB)"}
	}
	if fh.Size > u.deps.Config.UploadMaxBytes {
		return service.Upload{}, &service.ValidationError{Field: field, Message: "the file is larger than " + strconv.FormatInt(u.deps.Config.UploadMaxBytes>>20, 10) + " MB"}
	}
	f, err := fh.Open()
	if err != nil {
		return service.Upload{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return service.Upload{}, err
	}
	return service.Upload{Filename: fh.Filename, Data: data}, nil
}

// ---------- Bulk delete ----------

func (u *ui) projectsByID(c *gin.Context, ids []string) ([]domain.OverviewProject, error) {
	all, err := u.deps.Service.Projects(c)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []domain.OverviewProject
	for _, p := range all {
		if want[p.ID] {
			out = append(out, p)
		}
	}
	return out, nil
}

func (u *ui) deleteProjectsConfirm(c *gin.Context, status int, ids []string, errMsg string) {
	sel, err := u.projectsByID(c, ids)
	if err != nil {
		u.fail(c, err)
		return
	}
	cf := views.Confirm{
		Modal:   views.Modal{Title: "Delete projects", Action: u.url("/web/project/delete"), Submit: "Delete projects"},
		Message: "Delete these projects with all their applications, resources, ports, operations and mock responses? This cannot be undone.",
		Error:   errMsg,
	}
	for _, p := range sel {
		cf.IDs = append(cf.IDs, p.ID)
		cf.Items = append(cf.Items, p.Name+" ("+strings.ToUpper(string(p.Type))+")")
	}
	if len(sel) == 1 {
		cf.Modal.Title, cf.Modal.Submit = "Delete project", "Delete project"
		cf.Message = "Delete project " + sel[0].Name + " with everything it contains? This cannot be undone."
		cf.ConfirmName = sel[0].Name
	} else if len(sel) >= 10 {
		cf.ConfirmName = "delete " + strconv.Itoa(len(sel)) + " projects"
	}
	if len(sel) == 0 {
		cf.Message = "Select at least one project."
		cf.Modal.Action = ""
	}
	u.renderModal(c, status, cf.Modal, func(m views.Modal) templ.Component {
		cf.Modal = m
		return views.ConfirmDialog(cf)
	}, u.url("/web"), "Delete projects")
}

func (u *ui) deleteProjectsForm(c *gin.Context) {
	u.deleteProjectsConfirm(c, http.StatusOK, formIDs(c), "")
}

func (u *ui) deleteProjects(c *gin.Context) {
	sel, err := u.projectsByID(c, formIDs(c))
	if err != nil {
		u.fail(c, err)
		return
	}
	for _, p := range sel {
		if p.Type == domain.ProjectTypeSOAP {
			_, err = u.deps.Service.DeleteSoapProject(c, p.ID)
		} else {
			_, err = u.deps.Service.DeleteRestProject(c, p.ID)
		}
		if err != nil {
			u.fail(c, err)
			return
		}
	}
	setFlash(c, "success", views.Plural(len(sel), "project", "projects")+" deleted")
	redirect(c, u.url("/web"))
}
