package web

import (
	"context"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/service"
	"github.com/duv0-x/mockingbird/internal/web/views"
)

// REST screen routes, relative to {ctx}/web. They mirror CastleMock's hash
// routes (MainContainer.js); dialogs live under the entity they act on.
const (
	restP  = "/rest/project/:p"
	restA  = restP + "/application/:a"
	restR  = restA + "/resource/:r"
	restM  = restR + "/method/:m"
	restMR = restM + "/mockresponse/:mr"
)

func (u *ui) registerRest(rt routes) {
	rt.read.GET(restP, u.restProject)
	rt.read.GET(restP+"/export", u.restExport)
	rt.read.GET(restP+"/tree/application/:a", u.restTreeApplication)
	rt.read.GET(restP+"/tree/resource/:r", u.restTreeResource)
	u.dialog(rt.modify, restP+"/update", u.restProjectUpdate)
	u.dialog(rt.modify, restP+"/delete", u.restProjectDelete)
	u.dialog(rt.modify, restP+"/definition", u.restDefinition)
	u.dialog(rt.modify, restP+"/application/new", u.restApplicationCreate)
	u.bulkRoutes(rt.modify, restP+"/application", u.restApplicationsBulk())

	rt.read.GET(restA, u.restApplication)
	u.dialog(rt.modify, restA+"/update", u.restApplicationUpdate)
	u.dialog(rt.modify, restA+"/delete", u.restApplicationDelete)
	u.dialog(rt.modify, restA+"/resource/new", u.restResourceCreate)
	u.bulkRoutes(rt.modify, restA+"/resource", u.restResourcesBulk())

	rt.read.GET(restR, u.restResource)
	u.dialog(rt.modify, restR+"/update", u.restResourceUpdate)
	u.dialog(rt.modify, restR+"/delete", u.restResourceDelete)
	u.dialog(rt.modify, restR+"/method/new", u.restMethodCreate)
	u.bulkRoutes(rt.modify, restR+"/method", u.restMethodsBulk())

	rt.read.GET(restM, u.restMethod)
	u.dialog(rt.modify, restM+"/update", u.restMethodUpdate)
	u.dialog(rt.modify, restM+"/delete", u.restMethodDelete)
	u.dialog(rt.modify, restM+"/mockresponse/new", u.restResponseCreate)
	u.bulkRoutes(rt.modify, restM+"/mockresponse", u.restResponsesBulk())

	rt.read.GET(restMR, u.restResponse)
	rt.modify.POST(restMR, u.restResponseSave)
	u.dialog(rt.modify, restMR+"/delete", u.restResponseDelete)
	// Validating an expression only needs READER, like POST /core/expression/validate.
	u.dialog(rt.read, restMR+"/validate", u.restResponseValidate)
}

// dialog registers GET (open) and POST (submit) of a dialog on one handler.
func (u *ui) dialog(g *gin.RouterGroup, path string, h gin.HandlerFunc) {
	g.GET(path, h)
	g.POST(path, h)
}

func isPost(c *gin.Context) bool { return c.Request.Method == http.MethodPost }

// ---------- Loading ----------

// restCtx is the entity chain of a REST screen, resolved by the ids of the
// path (each entity by its own id, like the API).
type restCtx struct {
	ids     views.RestIDs
	project domain.RestProject
	app     domain.RestApplication
	res     domain.RestResource
	method  domain.RestMethod
	resp    domain.RestMockResponse
	depth   int
}

const (
	depthProject = iota + 1
	depthApp
	depthResource
	depthMethod
	depthResponse
)

func (u *ui) loadRest(c *gin.Context, depth int) (*restCtx, error) {
	s := u.deps.Service
	rc := &restCtx{depth: depth, ids: views.RestIDs{
		Project: c.Param("p"), App: c.Param("a"), Resource: c.Param("r"), Method: c.Param("m"), Response: c.Param("mr"),
	}}
	var err error
	if rc.project, err = s.RestProject(c, rc.ids.Project); err != nil {
		return nil, err
	}
	if depth >= depthApp {
		if rc.app, err = s.RestApplication(c, rc.ids.App); err != nil {
			return nil, err
		}
	}
	if depth >= depthResource {
		if rc.res, err = s.RestResource(c, rc.ids.Resource); err != nil {
			return nil, err
		}
	}
	if depth >= depthMethod {
		if rc.method, err = s.RestMethod(c, rc.ids.Method); err != nil {
			return nil, err
		}
	}
	if depth >= depthResponse {
		if rc.resp, err = s.RestMockResponse(c, rc.ids.Response); err != nil {
			return nil, err
		}
	}
	return rc, nil
}

func (rc *restCtx) crumbs(u *ui) []views.Crumb {
	cr := []views.Crumb{
		{Label: "Projects", Href: u.url("/web")},
		{Label: rc.project.Name, Href: u.url(rc.ids.ProjectPath()), Mono: true},
	}
	if rc.depth >= depthApp {
		cr = append(cr, views.Crumb{Label: rc.app.Name, Href: u.url(rc.ids.AppPath()), Mono: true})
	}
	if rc.depth >= depthResource {
		cr = append(cr, views.Crumb{Label: rc.res.URI, Href: u.url(rc.ids.ResourcePath()), Mono: true})
	}
	if rc.depth >= depthMethod {
		cr = append(cr, views.Crumb{Label: rc.method.Name, Href: u.url(rc.ids.MethodPath())})
	}
	if rc.depth >= depthResponse {
		cr = append(cr, views.Crumb{Label: rc.resp.Name, Href: u.url(rc.ids.ResponsePath())})
	}
	return cr
}

// restPage builds the layout model of a REST screen: breadcrumbs and the
// project tree expanded down to the current entity.
func (u *ui) restPage(c *gin.Context, rc *restCtx, title string) views.Page {
	p := u.page(c, title, views.NavProjects, rc.crumbs(u)...)
	p.Tree = u.restTree(rc)
	return p
}

func (u *ui) restTree(rc *restCtx) *views.TreeNode {
	ids := rc.ids
	root := &views.TreeNode{
		Label: rc.project.Name, Href: u.url(ids.ProjectPath()), Meta: "REST", Mono: true,
		Open: true, Loaded: true, Current: rc.depth == depthProject,
	}
	for _, a := range rc.project.Applications {
		n := u.restAppNode(ids.Project, a)
		if a.ID == ids.App && rc.depth >= depthApp {
			n.Open, n.Loaded, n.Current = true, true, rc.depth == depthApp
			for _, r := range rc.app.Resources {
				rn := u.restResourceNode(ids.Project, a.ID, r)
				if r.ID == ids.Resource && rc.depth >= depthResource {
					rn.Open, rn.Loaded, rn.Current = true, true, rc.depth == depthResource
					for _, m := range rc.res.Methods {
						mn := u.restMethodNode(ids.Project, a.ID, r.ID, m)
						mn.Current = m.ID == ids.Method && rc.depth >= depthMethod
						rn.Children = append(rn.Children, mn)
					}
				}
				n.Children = append(n.Children, rn)
			}
		}
		root.Children = append(root.Children, n)
	}
	return root
}

func (u *ui) restAppNode(p string, a domain.RestApplication) views.TreeNode {
	return views.TreeNode{Label: a.Name, Href: u.url(views.RestAppPath(p, a.ID)), Meta: "app", Mono: true,
		ChildrenURL: u.url(views.RestProjectPath(p) + "/tree/application/" + a.ID)}
}

func (u *ui) restResourceNode(p, a string, r domain.RestResource) views.TreeNode {
	return views.TreeNode{Label: r.URI, Href: u.url(views.RestResourcePath(p, a, r.ID)), Mono: true,
		ChildrenURL: u.url(views.RestProjectPath(p) + "/tree/resource/" + r.ID + "?a=" + a)}
}

func (u *ui) restMethodNode(p, a, r string, m domain.RestMethod) views.TreeNode {
	return views.TreeNode{Label: m.Name, Href: u.url(views.RestMethodPath(p, a, r, m.ID)), Method: m.HTTPMethod, Mode: string(m.Status)}
}

func (u *ui) restTreeApplication(c *gin.Context) {
	app, err := u.deps.Service.RestApplication(c, c.Param("a"))
	if err != nil {
		u.fail(c, err)
		return
	}
	nodes := make([]views.TreeNode, 0, len(app.Resources))
	for _, r := range app.Resources {
		nodes = append(nodes, u.restResourceNode(c.Param("p"), app.ID, r))
	}
	render(c, http.StatusOK, views.TreeChildren(nodes))
}

func (u *ui) restTreeResource(c *gin.Context) {
	res, err := u.deps.Service.RestResource(c, c.Param("r"))
	if err != nil {
		u.fail(c, err)
		return
	}
	a := c.Query("a")
	if a == "" {
		a = res.ApplicationID
	}
	nodes := make([]views.TreeNode, 0, len(res.Methods))
	for _, m := range res.Methods {
		nodes = append(nodes, u.restMethodNode(c.Param("p"), a, res.ID, m))
	}
	render(c, http.StatusOK, views.TreeChildren(nodes))
}

// ---------- Pages ----------

func (u *ui) restProject(c *gin.Context) {
	rc, err := u.loadRest(c, depthProject)
	if err != nil {
		u.fail(c, err)
		return
	}
	render(c, http.StatusOK, views.RestProjectPage(u.restPage(c, rc, rc.project.Name), views.RestProjectView{
		IDs: rc.ids, Project: rc.project, MockURL: u.mockURL(c, "/rest/project/"+rc.ids.Project),
	}))
}

func (u *ui) restApplication(c *gin.Context) {
	rc, err := u.loadRest(c, depthApp)
	if err != nil {
		u.fail(c, err)
		return
	}
	render(c, http.StatusOK, views.RestApplicationPage(u.restPage(c, rc, rc.app.Name), views.RestApplicationView{
		IDs: rc.ids, App: rc.app, MockURL: u.restAppMockURL(c, rc.ids),
	}))
}

// restAppMockURL is {base}{ctx}/mock/rest/project/{p}/application/{a}, the
// prefix of every resource of the application (AbstractRestServiceController).
func (u *ui) restAppMockURL(c *gin.Context, ids views.RestIDs) string {
	return u.mockURL(c, "/rest/project/"+ids.Project+"/application/"+ids.App)
}

func (u *ui) restResource(c *gin.Context) {
	rc, err := u.loadRest(c, depthResource)
	if err != nil {
		u.fail(c, err)
		return
	}
	render(c, http.StatusOK, views.RestResourcePage(u.restPage(c, rc, rc.res.Name), views.RestResourceView{
		IDs: rc.ids, Resource: rc.res, Address: u.restAppMockURL(c, rc.ids) + rc.res.URI,
	}))
}

func (u *ui) restMethod(c *gin.Context) {
	rc, err := u.loadRest(c, depthMethod)
	if err != nil {
		u.fail(c, err)
		return
	}
	address := u.restAppMockURL(c, rc.ids) + rc.res.URI
	render(c, http.StatusOK, views.RestMethodPage(u.restPage(c, rc, rc.method.Name), views.RestMethodView{
		IDs: rc.ids, Method: rc.method, URI: rc.res.URI, Address: address, Curl: curlCommand(rc.method.HTTPMethod, address),
	}))
}

func curlCommand(m domain.HTTPMethod, address string) string {
	cmd := "curl -i"
	if m != domain.MethodGET {
		cmd += " -X " + string(m)
	}
	return cmd + " '" + strings.ReplaceAll(address, "'", `'\''`) + "'"
}

func (u *ui) restResponse(c *gin.Context) {
	rc, err := u.loadRest(c, depthResponse)
	if err != nil {
		u.fail(c, err)
		return
	}
	u.renderRestResponse(c, http.StatusOK, rc, rc.resp, "")
}

func (u *ui) renderRestResponse(c *gin.Context, status int, rc *restCtx, resp domain.RestMockResponse, errMsg string) {
	params, err := u.deps.Service.RestResourceParameters(c, rc.ids.Resource)
	if err != nil {
		params = service.PathParameters(rc.res.URI)
	}
	render(c, status, views.RestResponsePage(u.restPage(c, rc, resp.Name), views.RestResponseView{
		IDs: rc.ids, Method: rc.method, Response: resp, Params: params, Error: errMsg,
	}))
}

// restResponseSave is "Save" of the mock response editor: every list is
// replaced by the posted rows (PUT .../mockresponse/{id}).
func (u *ui) restResponseSave(c *gin.Context) {
	rc, err := u.loadRest(c, depthResponse)
	if err != nil {
		u.fail(c, err)
		return
	}
	in, resp := restResponseFromForm(c, rc.resp)
	if _, err := u.deps.Service.UpdateRestMockResponse(c, rc.ids.Response, in); err != nil {
		msg, status, ok := formError(err)
		if !ok {
			u.fail(c, err)
			return
		}
		u.renderRestResponse(c, status, rc, resp, msg)
		return
	}
	setFlash(c, "success", "Mock response saved")
	redirect(c, u.url(rc.ids.ResponsePath()))
}

// restResponseFromForm decodes the editor; resp echoes the input back when
// the form has to be shown again with an error.
func restResponseFromForm(c *gin.Context, current domain.RestMockResponse) (service.RestMockResponseInput, domain.RestMockResponse) {
	_ = c.Request.ParseForm()
	f := c.Request.PostForm
	body := f.Get("body")
	status := domain.RestMockResponseDisabled
	if formBool(c, "enabled") {
		status = domain.RestMockResponseEnabled
	}
	using := formBool(c, "usingExpressions")
	in := service.RestMockResponseInput{
		Name:             strings.TrimSpace(f.Get("name")),
		Body:             &body,
		Status:           &status,
		UsingExpressions: &using,
		ContentEncodings: current.ContentEncodings,
	}
	code := current.HTTPStatusCode
	if n, err := atoiField(f.Get("httpStatusCode")); err == nil {
		code = n
		in.HTTPStatusCode = &n
	}
	for _, r := range indexedRows(f, "h") {
		if name := strings.TrimSpace(r["name"]); name != "" {
			in.HTTPHeaders = append(in.HTTPHeaders, domain.HTTPHeader{Name: name, Value: r["value"]})
		}
	}
	for _, r := range indexedRows(f, "pq") {
		if r["parameter"] == "" && r["query"] == "" {
			continue
		}
		in.ParameterQueries = append(in.ParameterQueries, domain.RestParameterQuery{
			Parameter: r["parameter"], Query: r["query"],
			MatchAny: rowBool(r, "matchAny"), MatchCase: rowBool(r, "matchCase"), MatchRegex: rowBool(r, "matchRegex"), URLEncoded: rowBool(r, "urlEncoded"),
		})
	}
	for _, r := range indexedRows(f, "hq") {
		if r["header"] == "" && r["query"] == "" {
			continue
		}
		in.HeaderQueries = append(in.HeaderQueries, domain.RestHeaderQuery{
			Header: r["header"], Query: r["query"],
			MatchAny: rowBool(r, "matchAny"), MatchCase: rowBool(r, "matchCase"), MatchRegex: rowBool(r, "matchRegex"),
		})
	}
	for _, r := range indexedRows(f, "jp") {
		if ex := strings.TrimSpace(r["expression"]); ex != "" {
			in.JSONPathExpressions = append(in.JSONPathExpressions, domain.RestJSONPathExpression{Expression: ex})
		}
	}
	for _, r := range indexedRows(f, "xp") {
		if ex := strings.TrimSpace(r["expression"]); ex != "" {
			in.XPathExpressions = append(in.XPathExpressions, domain.RestXPathExpression{Expression: ex})
		}
	}
	resp := current
	resp.Name, resp.Body, resp.HTTPStatusCode, resp.Status, resp.UsingExpressions = in.Name, body, code, status, &using
	resp.HTTPHeaders, resp.ParameterQueries, resp.HeaderQueries = in.HTTPHeaders, in.ParameterQueries, in.HeaderQueries
	resp.JSONPathExpressions, resp.XPathExpressions = in.JSONPathExpressions, in.XPathExpressions
	return in, resp
}

// ---------- Export ----------

func (u *ui) restExport(c *gin.Context) {
	format := service.ExportCastleMockXML
	if c.Query("format") == "json" {
		format = service.ExportNativeJSON
	}
	exp, err := u.deps.Service.ExportRestProject(c, c.Param("p"), format)
	if err != nil {
		u.fail(c, err)
		return
	}
	download(c, exp)
}

// download sends an export as an attachment.
func download(c *gin.Context, exp service.Export) {
	c.Header("Content-Disposition", `attachment; filename="`+strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(exp.Filename)+`"`)
	c.Data(http.StatusOK, exp.ContentType, exp.Data)
}

// ---------- Shared dialog plumbing ----------

func (u *ui) entityModal(c *gin.Context, status int, f views.EntityForm, back string) {
	u.renderModal(c, status, f.Modal, func(m views.Modal) templ.Component {
		f.Modal = m
		return views.EntityDialog(f)
	}, back, f.Modal.Title)
}

// submitEntity runs save for a POSTed entity dialog: on success it flashes
// and navigates to the path save returns; validation errors re-open the
// dialog with the message.
func (u *ui) submitEntity(c *gin.Context, f views.EntityForm, back string, save func() (string, string, error)) {
	to, msg, err := save()
	if err != nil {
		emsg, status, ok := formError(err)
		if !ok {
			u.fail(c, err)
			return
		}
		f.Error = emsg
		u.entityModal(c, status, f, back)
		return
	}
	setFlash(c, "success", msg)
	redirect(c, u.url(to))
}

func (u *ui) confirm(c *gin.Context, cf views.Confirm, back string) {
	u.renderModal(c, http.StatusOK, cf.Modal, func(m views.Modal) templ.Component {
		cf.Modal = m
		return views.ConfirmDialog(cf)
	}, back, cf.Modal.Title)
}

// ---------- Project dialogs ----------

func (u *ui) restProjectUpdate(c *gin.Context) {
	rc, err := u.loadRest(c, depthProject)
	if err != nil {
		u.fail(c, err)
		return
	}
	back := u.url(rc.ids.ProjectPath())
	f := views.EntityForm{
		Modal: views.Modal{Title: "Update project", Action: u.url(rc.ids.ProjectPath() + "/update"), Submit: "Update project"},
		Name:  rc.project.Name, Description: rc.project.Description, ShowDescription: true,
	}
	if !isPost(c) {
		u.entityModal(c, http.StatusOK, f, back)
		return
	}
	f.Name, f.Description = strings.TrimSpace(c.PostForm("name")), strings.TrimSpace(c.PostForm("description"))
	u.submitEntity(c, f, back, func() (string, string, error) {
		_, err := u.deps.Service.UpdateRestProject(c, rc.ids.Project, service.ProjectInput{Name: f.Name, Description: f.Description})
		return rc.ids.ProjectPath(), "Project updated", err
	})
}

func (u *ui) restProjectDelete(c *gin.Context) {
	rc, err := u.loadRest(c, depthProject)
	if err != nil {
		u.fail(c, err)
		return
	}
	if isPost(c) {
		if _, err := u.deps.Service.DeleteRestProject(c, rc.ids.Project); err != nil {
			u.fail(c, err)
			return
		}
		setFlash(c, "success", "Project "+rc.project.Name+" deleted")
		redirect(c, u.url("/web"))
		return
	}
	u.confirm(c, views.Confirm{
		Modal:       views.Modal{Title: "Delete project", Action: u.url(rc.ids.ProjectPath() + "/delete"), Submit: "Delete project"},
		Message:     "Delete project " + rc.project.Name + " and its " + views.Plural(len(rc.project.Applications), "application", "applications") + " with all their resources, methods and mock responses? This cannot be undone.",
		ConfirmName: rc.project.Name,
	}, u.url(rc.ids.ProjectPath()))
}

// restDefinition imports a RAML / Swagger / OpenAPI / WADL definition from a
// file or a URL into the project (§6.1).
func (u *ui) restDefinition(c *gin.Context) {
	rc, err := u.loadRest(c, depthProject)
	if err != nil {
		u.fail(c, err)
		return
	}
	base := rc.ids.ProjectPath()
	f := views.DefinitionForm{Type: strings.ToUpper(c.Query("type")), Source: c.Query("source"), GenerateResponse: !isPost(c)}
	if isPost(c) {
		f.Type, f.Source, f.URL, f.GenerateResponse = c.PostForm("definitionType"), c.PostForm("source"), strings.TrimSpace(c.PostForm("url")), formBool(c, "generateResponse")
	}
	if !optionValid(views.RestDefinitionTypes, f.Type) {
		f.Type = string(domain.RestDefinitionOpenAPI)
	}
	if f.Source != "url" {
		f.Source = "file"
	}
	f.Modal = views.Modal{Title: "Import " + views.DefinitionLabel(f.Type), Size: "md", Action: u.url(base + "/definition"), Multipart: f.Source == "file", Submit: "Import"}
	p := u.page(c, "", "")
	show := func(status int) {
		if isHTMX(c) {
			f.Modal.Tabs = views.DefinitionTabs(p, base, f.Type, f.Source)
		}
		u.renderModal(c, status, f.Modal, func(m views.Modal) templ.Component {
			f.Modal = m
			return views.DefinitionDialog(p, f, base)
		}, u.url(base), f.Modal.Title)
	}
	if !isPost(c) {
		show(http.StatusOK)
		return
	}
	var src service.Upload
	if f.Source == "url" {
		src.URL = f.URL
		if src.URL == "" {
			err = &service.ValidationError{Field: "url", Message: "enter the URL of the definition"}
		}
	} else {
		src, err = u.formUpload(c, "file")
	}
	if err == nil {
		err = u.deps.Service.ImportRestDefinition(c, rc.ids.Project, domain.RestDefinitionType(f.Type), src, f.GenerateResponse)
	}
	if err != nil {
		msg, status, ok := formError(err)
		if !ok {
			u.deps.Logger.Warn("ui import definition", "project", rc.ids.Project, "err", err)
			msg, status = "The definition could not be imported: "+err.Error(), http.StatusUnprocessableEntity
		}
		f.Error = msg
		show(status)
		return
	}
	setFlash(c, "success", views.DefinitionLabel(f.Type)+" definition imported")
	redirect(c, u.url(base))
}

func (u *ui) restApplicationCreate(c *gin.Context) {
	rc, err := u.loadRest(c, depthProject)
	if err != nil {
		u.fail(c, err)
		return
	}
	back := u.url(rc.ids.ProjectPath())
	f := views.EntityForm{Modal: views.Modal{Title: "New application", Action: u.url(rc.ids.ProjectPath() + "/application/new"), Submit: "Create application"}}
	if !isPost(c) {
		u.entityModal(c, http.StatusOK, f, back)
		return
	}
	f.Name = strings.TrimSpace(c.PostForm("name"))
	u.submitEntity(c, f, back, func() (string, string, error) {
		a, err := u.deps.Service.CreateRestApplication(c, rc.ids.Project, f.Name)
		return views.RestAppPath(rc.ids.Project, a.ID), "Application " + f.Name + " created", err
	})
}

// ---------- Application dialogs ----------

func (u *ui) restApplicationUpdate(c *gin.Context) {
	rc, err := u.loadRest(c, depthApp)
	if err != nil {
		u.fail(c, err)
		return
	}
	back := u.url(rc.ids.AppPath())
	f := views.EntityForm{Modal: views.Modal{Title: "Update application", Action: u.url(rc.ids.AppPath() + "/update"), Submit: "Update application"}, Name: rc.app.Name}
	if !isPost(c) {
		u.entityModal(c, http.StatusOK, f, back)
		return
	}
	f.Name = strings.TrimSpace(c.PostForm("name"))
	u.submitEntity(c, f, back, func() (string, string, error) {
		_, err := u.deps.Service.UpdateRestApplication(c, rc.ids.App, f.Name)
		return rc.ids.AppPath(), "Application updated", err
	})
}

func (u *ui) restApplicationDelete(c *gin.Context) {
	rc, err := u.loadRest(c, depthApp)
	if err != nil {
		u.fail(c, err)
		return
	}
	if isPost(c) {
		if _, err := u.deps.Service.DeleteRestApplication(c, rc.ids.App); err != nil {
			u.fail(c, err)
			return
		}
		setFlash(c, "success", "Application "+rc.app.Name+" deleted")
		redirect(c, u.url(rc.ids.ProjectPath()))
		return
	}
	u.confirm(c, views.Confirm{
		Modal:   views.Modal{Title: "Delete application", Action: u.url(rc.ids.AppPath() + "/delete"), Submit: "Delete application"},
		Message: "Delete application " + rc.app.Name + " and its " + views.Plural(len(rc.app.Resources), "resource", "resources") + " with all their methods and mock responses?",
	}, u.url(rc.ids.AppPath()))
}

func (u *ui) restResourceCreate(c *gin.Context) {
	rc, err := u.loadRest(c, depthApp)
	if err != nil {
		u.fail(c, err)
		return
	}
	back := u.url(rc.ids.AppPath())
	f := views.EntityForm{Modal: views.Modal{Title: "New resource", Action: u.url(rc.ids.AppPath() + "/resource/new"), Submit: "Create resource"}, ShowURI: true}
	if !isPost(c) {
		u.entityModal(c, http.StatusOK, f, back)
		return
	}
	f.Name, f.URI = strings.TrimSpace(c.PostForm("name")), strings.TrimSpace(c.PostForm("uri"))
	u.submitEntity(c, f, back, func() (string, string, error) {
		r, err := u.deps.Service.CreateRestResource(c, rc.ids.App, f.Name, f.URI)
		return views.RestResourcePath(rc.ids.Project, rc.ids.App, r.ID), "Resource " + f.Name + " created", err
	})
}

// ---------- Resource dialogs ----------

func (u *ui) restResourceUpdate(c *gin.Context) {
	rc, err := u.loadRest(c, depthResource)
	if err != nil {
		u.fail(c, err)
		return
	}
	back := u.url(rc.ids.ResourcePath())
	f := views.EntityForm{Modal: views.Modal{Title: "Update resource", Action: u.url(rc.ids.ResourcePath() + "/update"), Submit: "Update resource"}, Name: rc.res.Name, URI: rc.res.URI, ShowURI: true}
	if !isPost(c) {
		u.entityModal(c, http.StatusOK, f, back)
		return
	}
	f.Name, f.URI = strings.TrimSpace(c.PostForm("name")), strings.TrimSpace(c.PostForm("uri"))
	u.submitEntity(c, f, back, func() (string, string, error) {
		_, err := u.deps.Service.UpdateRestResource(c, rc.ids.Resource, f.Name, f.URI)
		return rc.ids.ResourcePath(), "Resource updated", err
	})
}

func (u *ui) restResourceDelete(c *gin.Context) {
	rc, err := u.loadRest(c, depthResource)
	if err != nil {
		u.fail(c, err)
		return
	}
	if isPost(c) {
		if _, err := u.deps.Service.DeleteRestResource(c, rc.ids.Resource); err != nil {
			u.fail(c, err)
			return
		}
		setFlash(c, "success", "Resource "+rc.res.Name+" deleted")
		redirect(c, u.url(rc.ids.AppPath()))
		return
	}
	u.confirm(c, views.Confirm{
		Modal:   views.Modal{Title: "Delete resource", Action: u.url(rc.ids.ResourcePath() + "/delete"), Submit: "Delete resource"},
		Message: "Delete resource " + rc.res.URI + " and its " + views.Plural(len(rc.res.Methods), "method", "methods") + " with all their mock responses?",
	}, u.url(rc.ids.ResourcePath()))
}

func (u *ui) restMethodCreate(c *gin.Context) {
	rc, err := u.loadRest(c, depthResource)
	if err != nil {
		u.fail(c, err)
		return
	}
	back := u.url(rc.ids.ResourcePath())
	f := views.EntityForm{Modal: views.Modal{Title: "New method", Action: u.url(rc.ids.ResourcePath() + "/method/new"), Submit: "Create method"}, ShowMethod: true, HTTPMethod: string(domain.MethodGET)}
	if !isPost(c) {
		u.entityModal(c, http.StatusOK, f, back)
		return
	}
	f.Name, f.HTTPMethod = strings.TrimSpace(c.PostForm("name")), c.PostForm("httpMethod")
	u.submitEntity(c, f, back, func() (string, string, error) {
		hm, err := domain.ParseHTTPMethod(f.HTTPMethod)
		if err != nil {
			return "", "", &service.ValidationError{Field: "httpMethod", Message: "choose an HTTP method"}
		}
		m, err := u.deps.Service.CreateRestMethod(c, rc.ids.Resource, service.RestMethodCreate{Name: f.Name, HTTPMethod: hm})
		return views.RestMethodPath(rc.ids.Project, rc.ids.App, rc.ids.Resource, m.ID), "Method " + f.Name + " created", err
	})
}

// ---------- Method dialogs ----------

func (u *ui) restMethodUpdate(c *gin.Context) {
	rc, err := u.loadRest(c, depthMethod)
	if err != nil {
		u.fail(c, err)
		return
	}
	back := u.url(rc.ids.MethodPath())
	f := views.RestMethodForm{Modal: views.Modal{Title: "Update method", Size: "md", Action: u.url(rc.ids.MethodPath() + "/update"), Submit: "Update method"}, Method: rc.method}
	show := func(status int) {
		u.renderModal(c, status, f.Modal, func(m views.Modal) templ.Component {
			f.Modal = m
			return views.RestMethodDialog(f)
		}, back, f.Modal.Title)
	}
	if !isPost(c) {
		show(http.StatusOK)
		return
	}
	in, echo, err := restMethodFromForm(c, rc.method)
	if err == nil {
		_, err = u.deps.Service.UpdateRestMethod(c, rc.ids.Method, in)
	}
	if err != nil {
		msg, status, ok := formError(err)
		if !ok {
			u.fail(c, err)
			return
		}
		f.Method, f.Error = echo, msg
		show(status)
		return
	}
	setFlash(c, "success", "Method updated")
	redirect(c, back)
}

// restMethodFromForm decodes "Update method". Like CastleMock's React form,
// every optional field is sent; an empty endpoint/default response is unset.
func restMethodFromForm(c *gin.Context, current domain.RestMethod) (service.RestMethodUpdate, domain.RestMethod, error) {
	echo := current
	in := service.RestMethodUpdate{
		Name:             strings.TrimSpace(c.PostForm("name")),
		Status:           domain.RestMethodStatus(c.PostForm("status")),
		ResponseStrategy: domain.RestResponseStrategy(c.PostForm("responseStrategy")),
	}
	simulate, auto := formBool(c, "simulateNetworkDelay"), formBool(c, "automaticForward")
	in.SimulateNetworkDelay, in.AutomaticForward = &simulate, &auto
	if ep := strings.TrimSpace(c.PostForm("forwardedEndpoint")); ep != "" {
		in.ForwardedEndpoint = &ep
	}
	if id := c.PostForm("defaultMockResponseId"); id != "" {
		in.DefaultMockResponseID = &id
	}
	echo.Name, echo.Status, echo.ResponseStrategy = in.Name, in.Status, in.ResponseStrategy
	echo.SimulateNetworkDelay, echo.AutomaticForward = in.SimulateNetworkDelay, in.AutomaticForward
	echo.ForwardedEndpoint, echo.DefaultMockResponseID = domain.Deref(in.ForwardedEndpoint), domain.Deref(in.DefaultMockResponseID)
	hm, err := domain.ParseHTTPMethod(c.PostForm("httpMethod"))
	if err != nil {
		return in, echo, &service.ValidationError{Field: "httpMethod", Message: "choose an HTTP method"}
	}
	in.HTTPMethod, echo.HTTPMethod = hm, hm
	echo.NetworkDelay = nil
	if d := strings.TrimSpace(c.PostForm("networkDelay")); d != "" {
		n, err := atoiField(d)
		if err != nil || n < 0 {
			return in, echo, &service.ValidationError{Field: "networkDelay", Message: "network delay must be a positive number of milliseconds"}
		}
		delay := int64(n)
		in.NetworkDelay, echo.NetworkDelay = &delay, &delay
	}
	return in, echo, nil
}

func (u *ui) restMethodDelete(c *gin.Context) {
	rc, err := u.loadRest(c, depthMethod)
	if err != nil {
		u.fail(c, err)
		return
	}
	if isPost(c) {
		if _, err := u.deps.Service.DeleteRestMethod(c, rc.ids.Method); err != nil {
			u.fail(c, err)
			return
		}
		setFlash(c, "success", "Method "+rc.method.Name+" deleted")
		redirect(c, u.url(rc.ids.ResourcePath()))
		return
	}
	u.confirm(c, views.Confirm{
		Modal:   views.Modal{Title: "Delete method", Action: u.url(rc.ids.MethodPath() + "/delete"), Submit: "Delete method"},
		Message: "Delete method " + string(rc.method.HTTPMethod) + " " + rc.res.URI + " (" + rc.method.Name + ") and its " + views.Plural(len(rc.method.MockResponses), "mock response", "mock responses") + "?",
	}, u.url(rc.ids.MethodPath()))
}

func (u *ui) restResponseCreate(c *gin.Context) {
	rc, err := u.loadRest(c, depthMethod)
	if err != nil {
		u.fail(c, err)
		return
	}
	back := u.url(rc.ids.MethodPath())
	f := views.EntityForm{Modal: views.Modal{Title: "New mock response", Action: u.url(rc.ids.MethodPath() + "/mockresponse/new"), Submit: "Create response"}, ShowStatus: true, Status: string(domain.RestMockResponseEnabled)}
	if !isPost(c) {
		u.entityModal(c, http.StatusOK, f, back)
		return
	}
	f.Name, f.Status = strings.TrimSpace(c.PostForm("name")), c.PostForm("status")
	u.submitEntity(c, f, back, func() (string, string, error) {
		// CreateMockResponseModal.js posts {name, status, httpStatusCode: 200}.
		st := domain.RestMockResponseStatus(f.Status)
		mr, err := u.deps.Service.CreateRestMockResponse(c, rc.ids.Method, service.RestMockResponseInput{Name: f.Name, Status: &st, HTTPStatusCode: domain.Ptr(http.StatusOK)})
		return views.RestResponsePath(rc.ids.Project, rc.ids.App, rc.ids.Resource, rc.ids.Method, mr.ID), "Mock response " + f.Name + " created", err
	})
}

// ---------- Mock response dialogs ----------

func (u *ui) restResponseDelete(c *gin.Context) {
	rc, err := u.loadRest(c, depthResponse)
	if err != nil {
		u.fail(c, err)
		return
	}
	if isPost(c) {
		if _, err := u.deps.Service.DeleteRestMockResponse(c, rc.ids.Response); err != nil {
			u.fail(c, err)
			return
		}
		setFlash(c, "success", "Mock response "+rc.resp.Name+" deleted")
		redirect(c, u.url(rc.ids.MethodPath()))
		return
	}
	u.confirm(c, views.Confirm{
		Modal:   views.Modal{Title: "Delete mock response", Action: u.url(rc.ids.ResponsePath() + "/delete"), Submit: "Delete response"},
		Message: "Delete mock response " + rc.resp.Name + "?",
	}, u.url(rc.ids.ResponsePath()))
}

// restResponseValidate renders the response body with its expressions
// against a sample request body (ValidateExpression, §4.4). The body being
// edited is included from the page; a direct visit uses the saved body.
func (u *ui) restResponseValidate(c *gin.Context) {
	rc, err := u.loadRest(c, depthResponse)
	if err != nil {
		u.fail(c, err)
		return
	}
	f := views.ValidateForm{Modal: views.Modal{Title: "Validate expression", Size: "md", Action: u.url(rc.ids.ResponsePath() + "/validate"), Submit: "Validate", Include: "#mr-body"}}
	if isPost(c) {
		f.RequestBody = c.PostForm("requestBody")
		body, ok := c.GetPostForm("body")
		if !ok {
			body = rc.resp.Body
		}
		f.Output, err = u.deps.Service.ValidateExpression(c, f.RequestBody, body)
		f.Validated = err == nil
		if err != nil {
			f.Error = "The expression could not be evaluated: " + err.Error()
		}
	}
	u.renderModal(c, http.StatusOK, f.Modal, func(m views.Modal) templ.Component {
		f.Modal = m
		return views.ValidateDialog(f)
	}, u.url(rc.ids.ResponsePath()), f.Modal.Title)
}

// ---------- Bulk specs ----------

func (u *ui) restApplicationsBulk() bulkSpec {
	s := u.deps.Service
	return bulkSpec{
		noun: "applications",
		candidates: func(c *gin.Context) ([]bulkItem, string, error) {
			p, err := s.RestProject(c, c.Param("p"))
			var items []bulkItem
			for _, a := range p.Applications {
				items = append(items, bulkItem{a.ID, a.Name})
			}
			return items, u.url(views.RestProjectPath(c.Param("p"))), err
		},
		statusOptions: views.RestMethodStatusOptions(),
		statusHelp:    "Applies to every method of the selected applications.",
		setStatus: func(ctx context.Context, ids []string, st string) error {
			return s.SetRestApplicationsStatus(ctx, ids, domain.RestMethodStatus(st))
		},
		endpointHelp: "Applies to every method of the selected applications.",
		setEndpoint:  s.SetRestApplicationsForwardedEndpoint,
		remove: func(ctx context.Context, id string) error {
			_, err := s.DeleteRestApplication(ctx, id)
			return err
		},
		removeHint: "Their resources, methods and mock responses are deleted too.",
	}
}

func (u *ui) restResourcesBulk() bulkSpec {
	s := u.deps.Service
	return bulkSpec{
		noun: "resources",
		candidates: func(c *gin.Context) ([]bulkItem, string, error) {
			a, err := s.RestApplication(c, c.Param("a"))
			var items []bulkItem
			for _, r := range a.Resources {
				items = append(items, bulkItem{r.ID, r.Name + " " + r.URI})
			}
			return items, u.url(views.RestAppPath(c.Param("p"), c.Param("a"))), err
		},
		statusOptions: views.RestMethodStatusOptions(),
		statusHelp:    "Applies to every method of the selected resources.",
		setStatus: func(ctx context.Context, ids []string, st string) error {
			return s.SetRestResourcesStatus(ctx, ids, domain.RestMethodStatus(st))
		},
		endpointHelp: "Applies to every method of the selected resources.",
		setEndpoint:  s.SetRestResourcesForwardedEndpoint,
		remove: func(ctx context.Context, id string) error {
			_, err := s.DeleteRestResource(ctx, id)
			return err
		},
		removeHint: "Their methods and mock responses are deleted too.",
	}
}

func (u *ui) restMethodsBulk() bulkSpec {
	s := u.deps.Service
	return bulkSpec{
		noun: "methods",
		candidates: func(c *gin.Context) ([]bulkItem, string, error) {
			r, err := s.RestResource(c, c.Param("r"))
			var items []bulkItem
			for _, m := range r.Methods {
				items = append(items, bulkItem{m.ID, string(m.HTTPMethod) + " " + m.Name})
			}
			return items, u.url(views.RestResourcePath(c.Param("p"), c.Param("a"), c.Param("r"))), err
		},
		statusOptions: views.RestMethodStatusOptions(),
		setStatus: func(ctx context.Context, ids []string, st string) error {
			return s.SetRestMethodsStatus(ctx, ids, domain.RestMethodStatus(st))
		},
		setEndpoint: s.SetRestMethodsForwardedEndpoint,
		remove: func(ctx context.Context, id string) error {
			_, err := s.DeleteRestMethod(ctx, id)
			return err
		},
		removeHint: "Their mock responses are deleted too.",
	}
}

func (u *ui) restResponsesBulk() bulkSpec {
	s := u.deps.Service
	return bulkSpec{
		noun: "mock responses",
		candidates: func(c *gin.Context) ([]bulkItem, string, error) {
			m, err := s.RestMethod(c, c.Param("m"))
			var items []bulkItem
			for _, r := range m.MockResponses {
				items = append(items, bulkItem{r.ID, r.Name + " (" + views.Itoa(r.HTTPStatusCode) + ")"})
			}
			return items, u.url(views.RestMethodPath(c.Param("p"), c.Param("a"), c.Param("r"), c.Param("m"))), err
		},
		statusOptions: views.ResponseStatusOptions(),
		setStatus: func(ctx context.Context, ids []string, st string) error {
			return s.SetRestMockResponsesStatus(ctx, ids, domain.RestMockResponseStatus(st))
		},
		duplicate: s.DuplicateRestMockResponses,
		remove: func(ctx context.Context, id string) error {
			_, err := s.DeleteRestMockResponse(ctx, id)
			return err
		},
	}
}
