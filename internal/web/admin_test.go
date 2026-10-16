package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/duv0-x/mockingbird/internal/domain"
)

func TestEvents(t *testing.T) {
	t.Parallel()
	e := newEnv(t)

	t.Run("overview newest first", func(t *testing.T) {
		w := e.do(req{path: "/web/event", role: domain.RoleReader})
		assertStatus(t, w.Code, http.StatusOK)
		body := w.Body.String()
		assertContains(t, body, "/castlemock/web/rest/event/evt001", "/castlemock/web/soap/event/evt002", "14 ms")
		if strings.Index(body, "evt001") > strings.Index(body, "evt002") {
			t.Fatal("events not sorted by start date desc")
		}
		if strings.Contains(body, "Clear events") {
			t.Fatal("reader sees Clear events")
		}
	})
	t.Run("filter fragment", func(t *testing.T) {
		w := e.do(req{path: "/web/event?type=soap", role: domain.RoleReader, htmx: true, target: "event-table"})
		body := w.Body.String()
		assertContains(t, body, "evt002")
		if strings.Contains(body, "evt001") || strings.Contains(body, "<html") {
			t.Fatal("unexpected fragment")
		}
	})
	t.Run("rest detail", func(t *testing.T) {
		w := e.do(req{path: "/web/rest/event/evt001", role: domain.RoleReader})
		assertStatus(t, w.Code, http.StatusOK)
		assertContains(t, w.Body.String(), "/customers/42", "expand", "{&#34;id&#34;:42}", "badge-status--2xx",
			"/castlemock/web/rest/project/"+e.seed.rest+"/application/"+e.seed.app+"/resource/"+e.seed.resource+"/method/"+e.seed.method)
	})
	t.Run("soap detail without response", func(t *testing.T) {
		w := e.do(req{path: "/web/soap/event/evt002", role: domain.RoleReader})
		assertStatus(t, w.Code, http.StatusOK)
		assertContains(t, w.Body.String(), "GetAccountRequest (http://bank.example/account)", "No response recorded",
			"/castlemock/web/soap/project/"+e.seed.soap+"/port/port01/operation/oper01")
	})
	t.Run("unknown event", func(t *testing.T) {
		w := e.do(req{path: "/web/rest/event/nope", role: domain.RoleReader})
		assertStatus(t, w.Code, http.StatusNotFound)
	})
	t.Run("clear events", func(t *testing.T) {
		w := e.do(req{path: "/web/event/clear", role: domain.RoleModifier, htmx: true, target: "modal-root"})
		assertContains(t, w.Body.String(), "<dialog", "Clear all events")
		w = e.do(req{method: http.MethodPost, path: "/web/event/clear", role: domain.RoleReader, form: url.Values{}})
		assertStatus(t, w.Code, http.StatusForbidden)
		w = e.do(req{method: http.MethodPost, path: "/web/event/clear", role: domain.RoleModifier, form: url.Values{}})
		assertStatus(t, w.Code, http.StatusSeeOther)
		if evs, _ := e.svc.Events(t.Context()); len(evs) != 0 {
			t.Fatalf("events left: %d", len(evs))
		}
	})
}

func TestUsers(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	admin := func(e *testEnv, method, path string, form url.Values) *httptestResult {
		w := e.do(req{method: method, path: path, role: domain.RoleAdmin, form: form, htmx: true, target: "modal-root"})
		return &httptestResult{code: w.Code, body: w.Body.String(), location: w.Header().Get("HX-Location")}
	}

	t.Run("admin only", func(t *testing.T) {
		e := newEnv(t)
		for _, path := range []string{"/web/user", "/web/user/uadmin", "/web/user/new", "/web/system"} {
			w := e.do(req{path: path, role: domain.RoleModifier})
			assertStatus(t, w.Code, http.StatusForbidden)
		}
		w := e.do(req{path: "/web", role: domain.RoleModifier})
		if strings.Contains(w.Body.String(), "/castlemock/web/user\"") {
			t.Fatal("modifier sees the Users nav")
		}
	})
	t.Run("list, filter and detail", func(t *testing.T) {
		e := newEnv(t)
		w := e.do(req{path: "/web/user?q=modif", role: domain.RoleAdmin})
		assertStatus(t, w.Code, http.StatusOK)
		body := w.Body.String()
		assertContains(t, body, "/castlemock/web/user/umodifier", "badge-role--modifier")
		if strings.Contains(body, "/castlemock/web/user/ureader\"") {
			t.Fatal("filter ignored")
		}
		w = e.do(req{path: "/web/user/ureader", role: domain.RoleAdmin})
		assertContains(t, w.Body.String(), "Test READER", "Read-only access", "/castlemock/web/user/ureader/update")
	})
	t.Run("create, update and delete", func(t *testing.T) {
		e := newEnv(t)
		r := admin(e, http.MethodPost, "/web/user/new", url.Values{"username": {"qa-bot"}, "password": {"pw"}, "fullName": {"QA"}, "email": {"qa@example.com"}, "role": {"MODIFIER"}, "status": {"ACTIVE"}})
		assertStatus(t, r.code, http.StatusOK)
		id := r.location[strings.LastIndex(r.location, "/")+1:]
		usr, err := e.svc.User(ctx, id)
		if err != nil || usr.Role != domain.RoleModifier || usr.Email != "qa@example.com" {
			t.Fatalf("user = %+v, %v", usr, err)
		}
		if _, _, err := e.auth.Login(ctx, "qa-bot", "pw"); err != nil {
			t.Fatalf("login with new password: %v", err)
		}
		r = admin(e, http.MethodPost, "/web/user/"+id+"/update", url.Values{"username": {"qa-bot"}, "password": {""}, "role": {"READER"}, "status": {"LOCKED"}})
		assertStatus(t, r.code, http.StatusOK)
		if usr, _ = e.svc.User(ctx, id); usr.Role != domain.RoleReader || usr.Status != domain.UserLocked {
			t.Fatalf("user = %+v", usr)
		}
		r = admin(e, http.MethodPost, "/web/user/"+id+"/delete", url.Values{})
		assertStatus(t, r.code, http.StatusOK)
		if _, err := e.svc.User(ctx, id); err == nil {
			t.Fatal("user not deleted")
		}
	})
	t.Run("validation errors", func(t *testing.T) {
		e := newEnv(t)
		r := admin(e, http.MethodPost, "/web/user/new", url.Values{"username": {"x"}, "password": {""}, "role": {"READER"}, "status": {"ACTIVE"}})
		assertStatus(t, r.code, http.StatusUnprocessableEntity)
		assertContains(t, r.body, "password cannot be empty")
		r = admin(e, http.MethodPost, "/web/user/new", url.Values{"username": {"READER"}, "password": {"pw"}, "role": {"READER"}, "status": {"ACTIVE"}})
		assertStatus(t, r.code, http.StatusUnprocessableEntity)
		r = admin(e, http.MethodPost, "/web/user/uadmin/update", url.Values{"username": {"admin"}, "role": {"READER"}, "status": {"ACTIVE"}})
		assertStatus(t, r.code, http.StatusUnprocessableEntity)
		r = admin(e, http.MethodPost, "/web/user/uadmin/delete", url.Values{})
		assertStatus(t, r.code, http.StatusUnprocessableEntity)
		assertContains(t, r.body, "alert--error")
	})
	t.Run("bulk delete", func(t *testing.T) {
		e := newEnv(t)
		w := e.do(req{path: "/web/user/delete?ids=ureader&ids=umodifier", role: domain.RoleAdmin, htmx: true, target: "modal-root"})
		assertContains(t, w.Body.String(), "Delete users", "reader", "modifier")
		r := admin(e, http.MethodPost, "/web/user/delete", url.Values{"ids": {"ureader", "umodifier"}})
		assertStatus(t, r.code, http.StatusOK)
		if users, _ := e.svc.Users(ctx); len(users) != 1 {
			t.Fatalf("users left: %d", len(users))
		}
	})
}

func TestProfileAndSystem(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := t.Context()

	t.Run("profile page", func(t *testing.T) {
		w := e.do(req{path: "/web/profile", role: domain.RoleReader})
		assertStatus(t, w.Code, http.StatusOK)
		assertContains(t, w.Body.String(), "Test READER", "Update profile", "/castlemock/web/profile/update")
	})
	t.Run("profile dialog has no role", func(t *testing.T) {
		w := e.do(req{path: "/web/profile/update", role: domain.RoleReader, htmx: true, target: "modal-root"})
		body := w.Body.String()
		assertContains(t, body, "<dialog", "Update profile", "Leave empty to keep")
		if strings.Contains(body, `name="role"`) {
			t.Fatal("profile dialog edits the role")
		}
	})
	t.Run("update profile keeps the password when empty", func(t *testing.T) {
		w := e.do(req{method: http.MethodPost, path: "/web/profile/update", role: domain.RoleReader, form: url.Values{"username": {"reader2"}, "fullName": {"R"}, "email": {"r@x"}, "password": {""}}})
		assertStatus(t, w.Code, http.StatusSeeOther)
		if _, _, err := e.auth.Login(ctx, "reader2", "secret"); err != nil {
			t.Fatalf("login after profile update: %v", err)
		}
		usr, _ := e.svc.User(ctx, "ureader")
		if usr.Role != domain.RoleReader {
			t.Fatalf("role changed: %s", usr.Role)
		}
	})
	t.Run("profile username conflict", func(t *testing.T) {
		w := e.do(req{method: http.MethodPost, path: "/web/profile/update", role: domain.RoleModifier, form: url.Values{"username": {"admin"}}, htmx: true, target: "modal-root"})
		assertStatus(t, w.Code, http.StatusUnprocessableEntity)
	})
	t.Run("system", func(t *testing.T) {
		w := e.do(req{path: "/web/system", role: domain.RoleAdmin})
		assertStatus(t, w.Code, http.StatusOK)
		assertContains(t, w.Body.String(), "mockingbird", "Processors", "data-theme-set")
	})
}
