package web

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/service"
	"github.com/duv0-x/mockingbird/internal/storage"
	"github.com/duv0-x/mockingbird/internal/web/views"
)

// SOAP screen routes, relative to {ctx}/web (CastleMock's MainContainer.js).
const (
	soapP  = "/soap/project/:p"
	soapPo = soapP + "/port/:po"
	soapO  = soapPo + "/operation/:o"
	soapMR = soapO + "/mockresponse/:mr"
	soapRs = soapP + "/resource/:rs"
)

// registerSoap mounts the SOAP project, port, operation, mock response and
// resource screens (SPEC §8.2).
func (u *ui) registerSoap(rt routes) {
	rt.read.GET(soapP, u.soapProject)
	rt.read.GET(soapP+"/export", u.soapExport)
	rt.read.GET(soapP+"/tree/port/:po", u.soapTreePort)
	u.dialog(rt.modify, soapP+"/update", u.soapProjectUpdate)
	u.dialog(rt.modify, soapP+"/delete", u.soapProjectDelete)
	u.dialog(rt.modify, soapP+"/wsdl", u.soapWSDL)
	u.bulkRoutes(rt.modify, soapP+"/port", u.soapPortsBulk())

	rt.read.GET(soapRs, u.soapResource)

	rt.read.GET(soapPo, u.soapPort)
	u.dialog(rt.modify, soapPo+"/update", u.soapPortUpdate)
	u.dialog(rt.modify, soapPo+"/delete", u.soapPortDelete)
	u.bulkRoutes(rt.modify, soapPo+"/operation", u.soapOperationsBulk())

	rt.read.GET(soapO, u.soapOperation)
	u.dialog(rt.modify, soapO+"/update", u.soapOperationUpdate)
	u.dialog(rt.modify, soapO+"/mockresponse/new", u.soapResponseCreate)
	u.bulkRoutes(rt.modify, soapO+"/mockresponse", u.soapResponsesBulk())

	rt.read.GET(soapMR, u.soapResponse)
	rt.modify.POST(soapMR, u.soapResponseSave)
	u.dialog(rt.modify, soapMR+"/delete", u.soapResponseDelete)
	u.dialog(rt.read, soapMR+"/validate", u.soapResponseValidate)
}

// ---------- Loading ----------

// soapCtx is the entity chain of a SOAP screen, each entity resolved by its
// own id like the API.
type soapCtx struct {
	ids     views.SoapIDs
	project domain.SoapProject
	port    domain.SoapPort
	op      domain.SoapOperation
	resp    domain.SoapMockResponse
	res     domain.SoapResource
	depth   int
}

const (
	soapDepthProject = iota + 1
	soapDepthPort
	soapDepthOperation
	soapDepthResponse
)

func (u *ui) loadSoap(c *gin.Context, depth int) (*soapCtx, error) {
	s := u.deps.Service
	sc := &soapCtx{depth: depth, ids: views.SoapIDs{
		Project: c.Param("p"), Port: c.Param("po"), Operation: c.Param("o"), Response: c.Param("mr"), Resource: c.Param("rs"),
	}}
	var err error
	if sc.project, err = s.SoapProject(c, sc.ids.Project); err != nil {
		return nil, err
	}
	if depth >= soapDepthPort {
		if sc.port, err = s.SoapPort(c, sc.ids.Port); err != nil {
			return nil, err
		}
	}
	if depth >= soapDepthOperation {
		if sc.op, err = s.SoapOperation(c, sc.ids.Operation); err != nil {
			return nil, err
		}
	}
	if depth >= soapDepthResponse {
		if sc.resp, err = s.SoapMockResponse(c, sc.ids.Response); err != nil {
			return nil, err
		}
	}
	return sc, nil
}

func (sc *soapCtx) crumbs(u *ui) []views.Crumb {
	cr := []views.Crumb{
		{Label: "Projects", Href: u.url("/web")},
		{Label: sc.project.Name, Href: u.url(sc.ids.ProjectPath()), Mono: true},
	}
	if sc.depth >= soapDepthPort {
		cr = append(cr, views.Crumb{Label: sc.port.Name, Href: u.url(sc.ids.PortPath()), Mono: true})
	}
	if sc.depth >= soapDepthOperation {
		cr = append(cr, views.Crumb{Label: sc.op.Name, Href: u.url(sc.ids.OperationPath()), Mono: true})
	}
	if sc.depth >= soapDepthResponse {
		cr = append(cr, views.Crumb{Label: sc.resp.Name, Href: u.url(sc.ids.ResponsePath())})
	}
	return cr
}

func (u *ui) soapPage(c *gin.Context, sc *soapCtx, title string) views.Page {
	p := u.page(c, title, views.NavProjects, sc.crumbs(u)...)
	p.Tree = u.soapTree(sc)
	return p
}

// soapTree is the sidebar tree: project → ports → operations, expanded down
// to the current entity; other ports load their operations lazily.
func (u *ui) soapTree(sc *soapCtx) *views.TreeNode {
	ids := sc.ids
	root := &views.TreeNode{
		Label: sc.project.Name, Href: u.url(ids.ProjectPath()), Meta: "SOAP", Mono: true,
		Open: true, Loaded: true, Current: sc.depth == soapDepthProject && ids.Resource == "",
	}
	for _, po := range sc.project.Ports {
		n := u.soapPortNode(ids.Project, po)
		if po.ID == ids.Port && sc.depth >= soapDepthPort {
			n.Open, n.Loaded, n.Current = true, true, sc.depth == soapDepthPort
			for _, o := range sc.port.Operations {
				on := u.soapOperationNode(ids.Project, po.ID, o)
				on.Current = o.ID == ids.Operation && sc.depth >= soapDepthOperation
				n.Children = append(n.Children, on)
			}
		}
		root.Children = append(root.Children, n)
	}
	return root
}

func (u *ui) soapPortNode(p string, po domain.SoapPort) views.TreeNode {
	return views.TreeNode{Label: po.Name, Href: u.url(views.SoapPortPath(p, po.ID)), Meta: "port", Mono: true,
		ChildrenURL: u.url(views.SoapProjectPath(p) + "/tree/port/" + po.ID)}
}

func (u *ui) soapOperationNode(p, po string, o domain.SoapOperation) views.TreeNode {
	return views.TreeNode{Label: o.Name, Href: u.url(views.SoapOperationPath(p, po, o.ID)), Mode: string(o.Status), Mono: true}
}

func (u *ui) soapTreePort(c *gin.Context) {
	po, err := u.deps.Service.SoapPort(c, c.Param("po"))
	if err != nil {
		u.fail(c, err)
		return
	}
	nodes := make([]views.TreeNode, 0, len(po.Operations))
	for _, o := range po.Operations {
		nodes = append(nodes, u.soapOperationNode(c.Param("p"), po.ID, o))
	}
	render(c, http.StatusOK, views.TreeChildren(nodes))
}

// soapPortAddress is the mock URL of a port: {base}{ctx}/mock/soap/project/{p}/{port uri}.
func (u *ui) soapPortAddress(c *gin.Context, projectID string, po domain.SoapPort) string {
	return u.mockURL(c, "/soap/project/"+projectID+"/"+strings.TrimPrefix(po.URI, "/"))
}

// ---------- Pages ----------

func (u *ui) soapProject(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthProject)
	if err != nil {
		u.fail(c, err)
		return
	}
	render(c, http.StatusOK, views.SoapProjectPage(u.soapPage(c, sc, sc.project.Name), views.SoapProjectView{
		IDs: sc.ids, Project: sc.project, MockURL: u.mockURL(c, "/soap/project/"+sc.ids.Project),
	}))
}

func (u *ui) soapPort(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthPort)
	if err != nil {
		u.fail(c, err)
		return
	}
	render(c, http.StatusOK, views.SoapPortPage(u.soapPage(c, sc, sc.port.Name), views.SoapPortView{
		IDs: sc.ids, Port: sc.port, Address: u.soapPortAddress(c, sc.ids.Project, sc.port),
	}))
}

func (u *ui) soapOperation(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthOperation)
	if err != nil {
		u.fail(c, err)
		return
	}
	op := sc.op
	op.DefaultBody = prettyXML(op.DefaultBody)
	render(c, http.StatusOK, views.SoapOperationPage(u.soapPage(c, sc, sc.op.Name), views.SoapOperationView{
		IDs: sc.ids, Operation: op, Address: u.soapPortAddress(c, sc.ids.Project, sc.port),
	}))
}

func (u *ui) soapResource(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthProject)
	if err != nil {
		u.fail(c, err)
		return
	}
	s := u.deps.Service
	if sc.res, err = s.SoapResource(c, sc.ids.Resource); err != nil {
		u.fail(c, err)
		return
	}
	// A resource without stored content renders as empty, not as 404.
	content, err := s.SoapResourceContent(c, sc.ids.Resource)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		u.fail(c, err)
		return
	}
	p := u.soapPage(c, sc, sc.res.Name)
	p.Crumbs = append(p.Crumbs, views.Crumb{Label: sc.res.Name, Mono: true})
	render(c, http.StatusOK, views.SoapResourcePage(p, views.SoapResourceView{
		IDs: sc.ids, Resource: sc.res, Content: prettyXML(content),
		RawURL: u.url("/resource/soap/project/" + sc.ids.Project + "/resource/" + sc.ids.Resource),
	}))
}

// prettyXML re-indents XML for display; anything that does not parse is
// returned unchanged.
func prettyXML(s string) string {
	if strings.TrimSpace(s) == "" {
		return s
	}
	dec := xml.NewDecoder(strings.NewReader(s))
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	for {
		tok, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return s
		}
		if cd, ok := tok.(xml.CharData); ok {
			if len(bytes.TrimSpace(cd)) == 0 {
				continue
			}
			tok = xml.CharData(bytes.TrimSpace(cd))
		}
		if err := enc.EncodeToken(tok); err != nil {
			return s
		}
	}
	if err := enc.Flush(); err != nil {
		return s
	}
	return buf.String()
}

func (u *ui) soapResponse(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthResponse)
	if err != nil {
		u.fail(c, err)
		return
	}
	render(c, http.StatusOK, views.SoapResponsePage(u.soapPage(c, sc, sc.resp.Name), views.SoapResponseView{IDs: sc.ids, Response: sc.resp}))
}

// soapResponseSave is "Save" of the SOAP mock response editor.
func (u *ui) soapResponseSave(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthResponse)
	if err != nil {
		u.fail(c, err)
		return
	}
	_ = c.Request.ParseForm()
	f := c.Request.PostForm
	in := service.SoapMockResponseInput{
		Name:           strings.TrimSpace(f.Get("name")),
		Body:           f.Get("body"),
		Status:         domain.SoapMockResponseDisabled,
		HTTPStatusCode: sc.resp.HTTPStatusCode,
	}
	if formBool(c, "enabled") {
		in.Status = domain.SoapMockResponseEnabled
	}
	using := formBool(c, "usingExpressions")
	in.UsingExpressions = &using
	if n, err := atoiField(f.Get("httpStatusCode")); err == nil {
		in.HTTPStatusCode = n
	}
	for _, r := range indexedRows(f, "h") {
		if name := strings.TrimSpace(r["name"]); name != "" {
			in.HTTPHeaders = append(in.HTTPHeaders, domain.HTTPHeader{Name: name, Value: r["value"]})
		}
	}
	for _, r := range indexedRows(f, "xp") {
		if ex := strings.TrimSpace(r["expression"]); ex != "" {
			in.XPathExpressions = append(in.XPathExpressions, domain.SoapXPathExpression{Expression: ex})
		}
	}
	if _, err := u.deps.Service.UpdateSoapMockResponse(c, sc.ids.Response, in); err != nil {
		msg, status, ok := formError(err)
		if !ok {
			u.fail(c, err)
			return
		}
		echo := sc.resp
		echo.Name, echo.Body, echo.Status, echo.HTTPStatusCode, echo.UsingExpressions = in.Name, in.Body, in.Status, in.HTTPStatusCode, &using
		echo.HTTPHeaders, echo.XPathExpressions = in.HTTPHeaders, in.XPathExpressions
		render(c, status, views.SoapResponsePage(u.soapPage(c, sc, sc.resp.Name), views.SoapResponseView{IDs: sc.ids, Response: echo, Error: msg}))
		return
	}
	setFlash(c, "success", "Mock response saved")
	redirect(c, u.url(sc.ids.ResponsePath()))
}

func (u *ui) soapExport(c *gin.Context) {
	format := service.ExportCastleMockXML
	if c.Query("format") == "json" {
		format = service.ExportNativeJSON
	}
	exp, err := u.deps.Service.ExportSoapProject(c, c.Param("p"), format)
	if err != nil {
		u.fail(c, err)
		return
	}
	download(c, exp)
}

// ---------- Project dialogs ----------

func (u *ui) soapProjectUpdate(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthProject)
	if err != nil {
		u.fail(c, err)
		return
	}
	back := u.url(sc.ids.ProjectPath())
	f := views.EntityForm{
		Modal: views.Modal{Title: "Update project", Action: u.url(sc.ids.ProjectPath() + "/update"), Submit: "Update project"},
		Name:  sc.project.Name, Description: sc.project.Description, ShowDescription: true,
	}
	if !isPost(c) {
		u.entityModal(c, http.StatusOK, f, back)
		return
	}
	f.Name, f.Description = strings.TrimSpace(c.PostForm("name")), strings.TrimSpace(c.PostForm("description"))
	u.submitEntity(c, f, back, func() (string, string, error) {
		_, err := u.deps.Service.UpdateSoapProject(c, sc.ids.Project, service.ProjectInput{Name: f.Name, Description: f.Description})
		return sc.ids.ProjectPath(), "Project updated", err
	})
}

func (u *ui) soapProjectDelete(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthProject)
	if err != nil {
		u.fail(c, err)
		return
	}
	if isPost(c) {
		if _, err := u.deps.Service.DeleteSoapProject(c, sc.ids.Project); err != nil {
			u.fail(c, err)
			return
		}
		setFlash(c, "success", "Project "+sc.project.Name+" deleted")
		redirect(c, u.url("/web"))
		return
	}
	u.confirm(c, views.Confirm{
		Modal:       views.Modal{Title: "Delete project", Action: u.url(sc.ids.ProjectPath() + "/delete"), Submit: "Delete project"},
		Message:     "Delete project " + sc.project.Name + " and its " + views.Plural(len(sc.project.Ports), "port", "ports") + " with all their operations, mock responses and WSDL resources? This cannot be undone.",
		ConfirmName: sc.project.Name,
	}, u.url(sc.ids.ProjectPath()))
}

// soapWSDL imports a WSDL from a file or a URL (UploadWSDLModal.js, §6.2).
// Linked WSDLs are only followed for URLs, like CastleMock.
func (u *ui) soapWSDL(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthProject)
	if err != nil {
		u.fail(c, err)
		return
	}
	base := sc.ids.ProjectPath()
	f := views.WSDLForm{Source: c.Query("source"), GenerateResponse: true}
	if isPost(c) {
		f.Source, f.URL = c.PostForm("source"), strings.TrimSpace(c.PostForm("url"))
		f.GenerateResponse, f.IncludeImports = formBool(c, "generateResponse"), formBool(c, "includeImports")
	}
	if f.Source != "url" {
		f.Source = "file"
	}
	f.Modal = views.Modal{Title: "Import WSDL", Size: "md", Action: u.url(base + "/wsdl"), Multipart: f.Source == "file", Submit: "Import"}
	p := u.page(c, "", "")
	show := func(status int) {
		if isHTMX(c) {
			f.Modal.Tabs = views.WSDLTabs(p, base, f.Source)
		}
		u.renderModal(c, status, f.Modal, func(m views.Modal) templ.Component {
			f.Modal = m
			return views.WSDLDialog(f)
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
			err = &service.ValidationError{Field: "url", Message: "enter the URL of the WSDL"}
		}
	} else {
		src, err = u.formUpload(c, "file")
		f.IncludeImports = false
	}
	if err == nil {
		err = u.deps.Service.ImportWSDL(c, sc.ids.Project, src, f.GenerateResponse, f.IncludeImports)
	}
	if err != nil {
		msg, status, ok := formError(err)
		if !ok {
			u.deps.Logger.Warn("ui import wsdl", "project", sc.ids.Project, "err", err)
			msg, status = "The WSDL could not be imported: "+err.Error(), http.StatusUnprocessableEntity
		}
		f.Error = msg
		show(status)
		return
	}
	setFlash(c, "success", "WSDL imported")
	redirect(c, u.url(base))
}

// ---------- Port dialogs ----------

func (u *ui) soapPortUpdate(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthPort)
	if err != nil {
		u.fail(c, err)
		return
	}
	back := u.url(sc.ids.PortPath())
	f := views.EntityForm{
		Modal: views.Modal{Title: "Update port", Action: u.url(sc.ids.PortPath() + "/update"), Submit: "Update port"},
		URI:   sc.port.URI, ShowURI: true, HideName: true,
		Hint: "The URI is the path of the port after " + u.deps.Config.ContextPath + "/mock/soap/project/" + sc.ids.Project + "/.",
	}
	if !isPost(c) {
		u.entityModal(c, http.StatusOK, f, back)
		return
	}
	f.URI = strings.TrimSpace(c.PostForm("uri"))
	u.submitEntity(c, f, back, func() (string, string, error) {
		_, err := u.deps.Service.UpdateSoapPort(c, sc.ids.Port, f.URI)
		return sc.ids.PortPath(), "Port updated", err
	})
}

func (u *ui) soapPortDelete(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthPort)
	if err != nil {
		u.fail(c, err)
		return
	}
	if isPost(c) {
		if _, err := u.deps.Service.DeleteSoapPort(c, sc.ids.Port); err != nil {
			u.fail(c, err)
			return
		}
		setFlash(c, "success", "Port "+sc.port.Name+" deleted")
		redirect(c, u.url(sc.ids.ProjectPath()))
		return
	}
	u.confirm(c, views.Confirm{
		Modal:   views.Modal{Title: "Delete port", Action: u.url(sc.ids.PortPath() + "/delete"), Submit: "Delete port"},
		Message: "Delete port " + sc.port.Name + " and its " + views.Plural(len(sc.port.Operations), "operation", "operations") + " with all their mock responses?",
	}, u.url(sc.ids.PortPath()))
}

// ---------- Operation dialogs ----------

func (u *ui) soapOperationUpdate(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthOperation)
	if err != nil {
		u.fail(c, err)
		return
	}
	back := u.url(sc.ids.OperationPath())
	f := views.SoapOperationForm{Modal: views.Modal{Title: "Update operation", Size: "md", Action: u.url(sc.ids.OperationPath() + "/update"), Submit: "Update operation"}, Operation: sc.op}
	show := func(status int) {
		u.renderModal(c, status, f.Modal, func(m views.Modal) templ.Component {
			f.Modal = m
			return views.SoapOperationDialog(f)
		}, back, f.Modal.Title)
	}
	if !isPost(c) {
		show(http.StatusOK)
		return
	}
	in, echo, err := soapOperationFromForm(c, sc.op)
	if err == nil {
		_, err = u.deps.Service.UpdateSoapOperation(c, sc.ids.Operation, in)
	}
	if err != nil {
		msg, status, ok := formError(err)
		if !ok {
			u.fail(c, err)
			return
		}
		f.Operation, f.Error = echo, msg
		show(status)
		return
	}
	setFlash(c, "success", "Operation updated")
	redirect(c, back)
}

// soapOperationFromForm decodes "Update operation". Like UpdateOperationModal.js,
// automatic forward is only kept when a forwarded endpoint is set.
func soapOperationFromForm(c *gin.Context, current domain.SoapOperation) (service.SoapOperationUpdate, domain.SoapOperation, error) {
	in := service.SoapOperationUpdate{
		Status:           domain.SoapOperationStatus(c.PostForm("status")),
		ResponseStrategy: domain.SoapResponseStrategy(c.PostForm("responseStrategy")),
		IdentifyStrategy: domain.SoapOperationIdentifyStrategy(c.PostForm("identifyStrategy")),
	}
	endpoint := strings.TrimSpace(c.PostForm("forwardedEndpoint"))
	simulate, mockOnFailure := formBool(c, "simulateNetworkDelay"), formBool(c, "mockOnFailure")
	auto := endpoint != "" && formBool(c, "automaticForward")
	in.SimulateNetworkDelay, in.MockOnFailure, in.AutomaticForward = &simulate, &mockOnFailure, &auto
	if endpoint != "" {
		in.ForwardedEndpoint = &endpoint
	}
	if id := c.PostForm("defaultMockResponseId"); id != "" {
		in.DefaultMockResponseID = &id
	}
	echo := current
	echo.Status, echo.ResponseStrategy, echo.IdentifyStrategy = in.Status, in.ResponseStrategy, in.IdentifyStrategy
	echo.ForwardedEndpoint, echo.DefaultMockResponseID = endpoint, domain.Deref(in.DefaultMockResponseID)
	echo.SimulateNetworkDelay, echo.MockOnFailure, echo.AutomaticForward = in.SimulateNetworkDelay, in.MockOnFailure, in.AutomaticForward
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

// soapResponseCreate creates a mock response with the operation's default
// body (SPEC §8.2) and opens it in the editor.
func (u *ui) soapResponseCreate(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthOperation)
	if err != nil {
		u.fail(c, err)
		return
	}
	back := u.url(sc.ids.OperationPath())
	f := views.EntityForm{
		Modal:      views.Modal{Title: "New mock response", Action: u.url(sc.ids.OperationPath() + "/mockresponse/new"), Submit: "Create response"},
		ShowStatus: true, Status: string(domain.SoapMockResponseEnabled),
	}
	if sc.op.DefaultBody != "" {
		f.Hint = "The body starts as the operation's default response body."
	}
	if !isPost(c) {
		u.entityModal(c, http.StatusOK, f, back)
		return
	}
	f.Name, f.Status = strings.TrimSpace(c.PostForm("name")), c.PostForm("status")
	u.submitEntity(c, f, back, func() (string, string, error) {
		mr, err := u.deps.Service.CreateSoapMockResponse(c, sc.ids.Operation, service.SoapMockResponseInput{
			Name: f.Name, Status: domain.SoapMockResponseStatus(f.Status), HTTPStatusCode: http.StatusOK, Body: sc.op.DefaultBody,
		})
		return views.SoapResponsePath(sc.ids.Project, sc.ids.Port, sc.ids.Operation, mr.ID), "Mock response " + f.Name + " created", err
	})
}

// ---------- Mock response dialogs ----------

func (u *ui) soapResponseDelete(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthResponse)
	if err != nil {
		u.fail(c, err)
		return
	}
	if isPost(c) {
		if _, err := u.deps.Service.DeleteSoapMockResponse(c, sc.ids.Response); err != nil {
			u.fail(c, err)
			return
		}
		setFlash(c, "success", "Mock response "+sc.resp.Name+" deleted")
		redirect(c, u.url(sc.ids.OperationPath()))
		return
	}
	u.confirm(c, views.Confirm{
		Modal:   views.Modal{Title: "Delete mock response", Action: u.url(sc.ids.ResponsePath() + "/delete"), Submit: "Delete response"},
		Message: "Delete mock response " + sc.resp.Name + "?",
	}, u.url(sc.ids.ResponsePath()))
}

func (u *ui) soapResponseValidate(c *gin.Context) {
	sc, err := u.loadSoap(c, soapDepthResponse)
	if err != nil {
		u.fail(c, err)
		return
	}
	u.validateExpression(c, sc.resp.Body, u.url(sc.ids.ResponsePath()))
}

// validateExpression is the "Validate expression" dialog shared by the REST
// and SOAP editors: it renders the body being edited (included from the page)
// or, on a direct visit, the saved body.
func (u *ui) validateExpression(c *gin.Context, saved, path string) {
	f := views.ValidateForm{Modal: views.Modal{Title: "Validate expression", Size: "md", Action: path + "/validate", Submit: "Validate", Include: "#mr-body"}}
	if isPost(c) {
		f.RequestBody = c.PostForm("requestBody")
		body, ok := c.GetPostForm("body")
		if !ok {
			body = saved
		}
		var err error
		f.Output, err = u.deps.Service.ValidateExpression(c, f.RequestBody, body)
		f.Validated = err == nil
		if err != nil {
			f.Error = "The expression could not be evaluated: " + err.Error()
		}
	}
	u.renderModal(c, http.StatusOK, f.Modal, func(m views.Modal) templ.Component {
		f.Modal = m
		return views.ValidateDialog(f)
	}, path, f.Modal.Title)
}

// ---------- Bulk specs ----------

func (u *ui) soapPortsBulk() bulkSpec {
	s := u.deps.Service
	return bulkSpec{
		noun: "ports",
		candidates: func(c *gin.Context) ([]bulkItem, string, error) {
			p, err := s.SoapProject(c, c.Param("p"))
			var items []bulkItem
			for _, po := range p.Ports {
				items = append(items, bulkItem{po.ID, po.Name})
			}
			return items, u.url(views.SoapProjectPath(c.Param("p"))), err
		},
		statusOptions: views.RestMethodStatusOptions(),
		statusHelp:    "Applies to every operation of the selected ports.",
		setStatus: func(ctx context.Context, ids []string, st string) error {
			return s.SetSoapPortsStatus(ctx, ids, domain.SoapOperationStatus(st))
		},
		endpointHelp: "Applies to every operation of the selected ports.",
		setEndpoint:  s.SetSoapPortsForwardedEndpoint,
		remove: func(ctx context.Context, id string) error {
			_, err := s.DeleteSoapPort(ctx, id)
			return err
		},
		removeHint: "Their operations and mock responses are deleted too.",
	}
}

func (u *ui) soapOperationsBulk() bulkSpec {
	s := u.deps.Service
	return bulkSpec{
		noun: "operations",
		candidates: func(c *gin.Context) ([]bulkItem, string, error) {
			po, err := s.SoapPort(c, c.Param("po"))
			var items []bulkItem
			for _, o := range po.Operations {
				items = append(items, bulkItem{o.ID, o.Name})
			}
			return items, u.url(views.SoapPortPath(c.Param("p"), c.Param("po"))), err
		},
		statusOptions: views.RestMethodStatusOptions(),
		setStatus: func(ctx context.Context, ids []string, st string) error {
			return s.SetSoapOperationsStatus(ctx, ids, domain.SoapOperationStatus(st))
		},
		setEndpoint: s.SetSoapOperationsForwardedEndpoint,
	}
}

func (u *ui) soapResponsesBulk() bulkSpec {
	s := u.deps.Service
	return bulkSpec{
		noun: "mock responses",
		candidates: func(c *gin.Context) ([]bulkItem, string, error) {
			o, err := s.SoapOperation(c, c.Param("o"))
			var items []bulkItem
			for _, r := range o.MockResponses {
				items = append(items, bulkItem{r.ID, r.Name + " (" + views.Itoa(r.HTTPStatusCode) + ")"})
			}
			return items, u.url(views.SoapOperationPath(c.Param("p"), c.Param("po"), c.Param("o"))), err
		},
		statusOptions: views.ResponseStatusOptions(),
		setStatus: func(ctx context.Context, ids []string, st string) error {
			return s.SetSoapMockResponsesStatus(ctx, ids, domain.SoapMockResponseStatus(st))
		},
		duplicate: s.DuplicateSoapMockResponses,
		remove: func(ctx context.Context, id string) error {
			_, err := s.DeleteSoapMockResponse(ctx, id)
			return err
		},
	}
}
