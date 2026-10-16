package web

import (
	"context"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/service"
	"github.com/duv0-x/mockingbird/internal/web/views"
)

// bulkSpec describes the bulk actions of one table (REST or SOAP).
// candidates returns the rows of the table in order and the page to go back
// to; nil actions are not offered.
type bulkSpec struct {
	noun          string // plural, lower case: "applications"
	candidates    func(c *gin.Context) ([]bulkItem, string, error)
	statusOptions []views.Option
	statusHelp    string
	setStatus     func(ctx context.Context, ids []string, status string) error
	endpointHelp  string
	setEndpoint   func(ctx context.Context, ids []string, endpoint string) error
	duplicate     func(ctx context.Context, ids []string) error
	remove        func(ctx context.Context, id string) error
	removeHint    string
}

type bulkItem struct{ id, label string }

// bulkRoutes mounts base+"/status|endpoint|duplicate|delete" (GET opens the
// dialog with the ids of the bulk form, POST applies it).
func (u *ui) bulkRoutes(g *gin.RouterGroup, base string, s bulkSpec) {
	if s.setStatus != nil {
		u.dialog(g, base+"/status", func(c *gin.Context) { u.bulk(c, s, "status") })
	}
	if s.setEndpoint != nil {
		u.dialog(g, base+"/endpoint", func(c *gin.Context) { u.bulk(c, s, "endpoint") })
	}
	if s.duplicate != nil {
		u.dialog(g, base+"/duplicate", func(c *gin.Context) { u.bulk(c, s, "duplicate") })
	}
	if s.remove != nil {
		u.dialog(g, base+"/delete", func(c *gin.Context) { u.bulk(c, s, "delete") })
	}
}

func (s bulkSpec) one() string { return strings.TrimSuffix(s.noun, "s") }

// bulk opens (GET) or applies (POST) a bulk action to the selected ids that
// belong to the table; other ids are skipped, like the API does.
func (u *ui) bulk(c *gin.Context, s bulkSpec, action string) {
	items, back, err := s.candidates(c)
	if err != nil {
		u.fail(c, err)
		return
	}
	want := map[string]bool{}
	for _, id := range formIDs(c) {
		want[id] = true
	}
	var ids, labels []string
	for _, it := range items {
		if want[it.id] {
			ids = append(ids, it.id)
			labels = append(labels, it.label)
		}
	}
	errMsg, status := "", http.StatusOK
	if isPost(c) && len(ids) > 0 {
		msg, err := u.applyBulk(c, s, action, ids)
		if err == nil {
			setFlash(c, "success", msg)
			redirect(c, back)
			return
		}
		var ok bool
		if errMsg, status, ok = formError(err); !ok {
			u.fail(c, err)
			return
		}
	}
	modal := views.Modal{Action: c.Request.URL.Path}
	var comp func(views.Modal) templ.Component
	switch {
	case len(ids) == 0:
		modal.Title, modal.Action = views.EnumLabel(s.noun), ""
		comp = func(m views.Modal) templ.Component {
			return views.BulkPlainDialog(views.BulkPlain{Modal: m, Message: "Select at least one row first."})
		}
	case action == "status":
		modal.Title, modal.Submit = "Update status of "+s.noun, "Update status"
		v := views.BulkStatus{IDs: ids, Items: labels, Options: s.statusOptions, Help: s.statusHelp, Error: errMsg}
		if len(s.statusOptions) > 0 {
			v.Selected = s.statusOptions[0].Value
		}
		comp = func(m views.Modal) templ.Component { v.Modal = m; return views.BulkStatusDialog(v) }
	case action == "endpoint":
		modal.Title, modal.Submit = "Update forwarded endpoint", "Update endpoint"
		v := views.BulkEndpoint{IDs: ids, Items: labels, Help: s.endpointHelp, Error: errMsg}
		comp = func(m views.Modal) templ.Component { v.Modal = m; return views.BulkEndpointDialog(v) }
	case action == "duplicate":
		modal.Title, modal.Submit = "Duplicate "+s.noun, "Duplicate"
		v := views.BulkPlain{IDs: ids, Items: labels, Message: "Create a copy of each of these " + s.noun + "?"}
		comp = func(m views.Modal) templ.Component { v.Modal = m; return views.BulkPlainDialog(v) }
	default:
		modal.Title, modal.Submit = "Delete "+s.noun, "Delete "+s.noun
		cf := views.Confirm{IDs: ids, Items: labels, Error: errMsg, Message: strings.TrimSpace("Delete these " + s.noun + "? " + s.removeHint)}
		if len(ids) >= 10 {
			cf.ConfirmName = "delete " + views.Itoa(len(ids)) + " " + s.noun
		}
		comp = func(m views.Modal) templ.Component { cf.Modal = m; return views.ConfirmDialog(cf) }
	}
	u.renderModal(c, status, modal, comp, back, modal.Title)
}

func (u *ui) applyBulk(c *gin.Context, s bulkSpec, action string, ids []string) (string, error) {
	n := views.Plural(len(ids), s.one(), s.noun)
	switch action {
	case "status":
		st := c.PostForm("status")
		if !optionValid(s.statusOptions, st) {
			return "", &service.ValidationError{Field: "status", Message: "choose a status"}
		}
		return "Status of " + n + " set to " + views.EnumLabel(st), s.setStatus(c, ids, st)
	case "endpoint":
		return "Forwarded endpoint of " + n + " updated", s.setEndpoint(c, ids, strings.TrimSpace(c.PostForm("forwardedEndpoint")))
	case "duplicate":
		return n + " duplicated", s.duplicate(c, ids)
	}
	if strings.TrimSpace(c.PostForm("confirmName")) == "" && len(ids) >= 10 {
		return "", &service.ValidationError{Field: "confirmName", Message: "type the confirmation text"}
	}
	for _, id := range ids {
		if err := s.remove(c, id); err != nil {
			return "", err
		}
	}
	return n + " deleted", nil
}

func optionValid(opts []views.Option, v string) bool {
	for _, o := range opts {
		if o.Value == v {
			return true
		}
	}
	return false
}
