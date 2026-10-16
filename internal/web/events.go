package web

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/web/views"
)

const eventsPerPage = 50

// registerEvents mounts the Events overview and event detail screens (SPEC §8.2).
func (u *ui) registerEvents(rt routes) {
	rt.read.GET("/event", u.events)
	u.dialog(rt.modify, "/event/clear", u.clearEvents)
	rt.read.GET("/rest/event/:id", u.restEvent)
	rt.read.GET("/soap/event/:id", u.soapEvent)
}

// events lists REST and SOAP events newest first (EventOverview.js sorts by
// start date descending).
func (u *ui) events(c *gin.Context) {
	all, err := u.deps.Service.Events(c)
	if err != nil {
		u.fail(c, err)
		return
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].StartDate.After(all[j].StartDate.Time) })
	v := views.Events{Total: len(all), Query: strings.TrimSpace(c.Query("q")), Type: c.Query("type")}
	if v.Type != "rest" && v.Type != "soap" {
		v.Type = ""
	}
	rows := make([]domain.OverviewEvent, 0, len(all))
	for _, e := range all {
		if v.Type != "" && string(e.Type) != v.Type {
			continue
		}
		if v.Query != "" && !containsFold(e.ResourceName, v.Query) {
			continue
		}
		rows = append(rows, e)
	}
	v.Filtered = len(rows)
	v.Pages = max(1, (len(rows)+eventsPerPage-1)/eventsPerPage)
	v.Page, _ = strconv.Atoi(c.Query("page"))
	v.Page = min(max(v.Page, 1), v.Pages)
	start := (v.Page - 1) * eventsPerPage
	end := min(start+eventsPerPage, len(rows))
	v.Rows, v.From, v.To = rows[start:end], start+1, end

	p := u.withProjects(c, u.page(c, "Events", views.NavEvents, views.Crumb{Label: "Events"}))
	if targets(c, "event-table") {
		render(c, http.StatusOK, views.EventTable(p, v))
		return
	}
	render(c, http.StatusOK, views.EventsPage(p, v))
}

func (u *ui) clearEvents(c *gin.Context) {
	if isPost(c) {
		if err := u.deps.Service.ClearEvents(c); err != nil {
			u.fail(c, err)
			return
		}
		setFlash(c, "success", "Events cleared")
		redirect(c, u.url("/web/event"))
		return
	}
	cf := views.Confirm{
		Modal:   views.Modal{Title: "Clear events", Action: u.url("/web/event/clear"), Submit: "Clear all events"},
		Message: "Delete every recorded REST and SOAP event? This cannot be undone.",
	}
	u.renderModal(c, http.StatusOK, cf.Modal, func(m views.Modal) templ.Component {
		cf.Modal = m
		return views.ConfirmDialog(cf)
	}, u.url("/web/event"), cf.Modal.Title)
}

func (u *ui) eventPage(c *gin.Context, v views.EventDetail) {
	p := u.withProjects(c, u.page(c, v.Event.ResourceName, views.NavEvents,
		views.Crumb{Label: "Events", Href: u.url("/web/event")},
		views.Crumb{Label: v.Event.ID, Mono: true}))
	render(c, http.StatusOK, views.EventDetailPage(p, v))
}

func responseMessage(code int, mockName, contentType string, encodings []domain.ContentEncoding, body string, headers []domain.HTTPHeader) *views.EventMessage {
	parts := make([]string, 0, len(encodings))
	for _, e := range encodings {
		parts = append(parts, string(e))
	}
	return &views.EventMessage{
		Facts: []views.EventFact{
			{Label: "HTTP status", Value: strconv.Itoa(code), Mono: true},
			{Label: "Mock response", Value: mockName},
			{Label: "Content type", Value: contentType, Mono: true},
			{Label: "Content encodings", Value: strings.Join(parts, ", "), Mono: true},
		},
		Body: body, Headers: headers,
	}
}

// restEvent is the REST event detail (RestEvent.js).
func (u *ui) restEvent(c *gin.Context) {
	e, err := u.deps.Service.RestEvent(c, c.Param("id"))
	if err != nil {
		u.fail(c, err)
		return
	}
	v := views.EventDetail{
		Event: e.Event, Type: "REST", Method: e.Request.HTTPMethod,
		Links: eventLinks(
			views.EventLink{Label: "Project", ID: e.ProjectID, Href: views.RestProjectPath(e.ProjectID)},
			views.EventLink{Label: "Application", ID: e.ApplicationID, Href: views.RestAppPath(e.ProjectID, e.ApplicationID)},
			views.EventLink{Label: "Resource", ID: e.ResourceID, Href: views.RestResourcePath(e.ProjectID, e.ApplicationID, e.ResourceID)},
			views.EventLink{Label: "Method", ID: e.MethodID, Href: views.RestMethodPath(e.ProjectID, e.ApplicationID, e.ResourceID, e.MethodID)},
		),
		Request: views.EventMessage{
			Facts: []views.EventFact{
				{Label: "URI", Value: e.Request.URI, Mono: true},
				{Label: "Method", Value: string(e.Request.HTTPMethod), Mono: true},
				{Label: "Content type", Value: e.Request.ContentType, Mono: true},
			},
			Body: e.Request.Body, Headers: e.Request.HTTPHeaders, Parameters: e.Request.HTTPParameters, ShowParameters: true,
		},
	}
	if r := e.Response; r != nil {
		v.Status = r.HTTPStatusCode
		v.Response = responseMessage(r.HTTPStatusCode, r.MockResponseName, r.ContentType, r.ContentEncodings, r.Body, r.HTTPHeaders)
	}
	u.eventPage(c, v)
}

// soapEvent is the SOAP event detail (SoapEvent.js).
func (u *ui) soapEvent(c *gin.Context) {
	e, err := u.deps.Service.SoapEvent(c, c.Param("id"))
	if err != nil {
		u.fail(c, err)
		return
	}
	id := e.Request.OperationIdentifier
	ident := id.Name
	if id.Namespace != "" {
		ident += " (" + id.Namespace + ")"
	}
	v := views.EventDetail{
		Event: e.Event, Type: "SOAP", Method: e.Request.HTTPMethod,
		Links: eventLinks(
			views.EventLink{Label: "Project", ID: e.ProjectID, Href: views.SoapProjectPath(e.ProjectID)},
			views.EventLink{Label: "Port", ID: e.PortID, Href: views.SoapPortPath(e.ProjectID, e.PortID)},
			views.EventLink{Label: "Operation", ID: e.OperationID, Href: views.SoapOperationPath(e.ProjectID, e.PortID, e.OperationID)},
		),
		Request: views.EventMessage{
			Facts: []views.EventFact{
				{Label: "URI", Value: e.Request.URI, Mono: true},
				{Label: "Method", Value: string(e.Request.HTTPMethod), Mono: true},
				{Label: "Operation", Value: e.Request.OperationName, Mono: true},
				{Label: "Identifier", Value: ident, Mono: true},
				{Label: "SOAP version", Value: views.SoapVersionLabel(e.Request.SoapVersion)},
				{Label: "Content type", Value: e.Request.ContentType, Mono: true},
			},
			Body: e.Request.Body, Headers: e.Request.HTTPHeaders,
		},
	}
	if r := e.Response; r != nil {
		v.Status = r.HTTPStatusCode
		v.Response = responseMessage(r.HTTPStatusCode, r.MockResponseName, r.ContentType, r.ContentEncodings, r.Body, r.HTTPHeaders)
	}
	u.eventPage(c, v)
}

// eventLinks drops the href of a link, and of every link after it, once an
// id of the chain is missing (e.g. an event whose method was not matched).
func eventLinks(links ...views.EventLink) []views.EventLink {
	broken := false
	for i := range links {
		if links[i].ID == "" {
			broken = true
		}
		if broken {
			links[i].Href = ""
		}
	}
	return links
}
