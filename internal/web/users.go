package web

import (
	"context"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/service"
	"github.com/duv0-x/mockingbird/internal/web/views"
)

// registerUsers mounts the Users and user screens, ADMIN only (SPEC §8.2).
func (u *ui) registerUsers(rt routes) {
	rt.admin.GET("/user", u.users)
	u.dialog(rt.admin, "/user/new", u.userCreate)
	u.bulkRoutes(rt.admin, "/user", u.usersBulk())
	rt.admin.GET("/user/:id", u.user)
	u.dialog(rt.admin, "/user/:id/update", u.userUpdate)
	u.dialog(rt.admin, "/user/:id/delete", u.userDelete)
}

func (u *ui) usersCrumbs(extra ...views.Crumb) []views.Crumb {
	return append([]views.Crumb{{Label: "Users", Href: u.url("/web/user")}}, extra...)
}

// users is the user overview (UserOverview.js) with a text filter.
func (u *ui) users(c *gin.Context) {
	all, err := u.deps.Service.Users(c)
	if err != nil {
		u.fail(c, err)
		return
	}
	v := views.Users{Total: len(all), Query: strings.TrimSpace(c.Query("q"))}
	if me, ok := auth.PrincipalFrom(c); ok {
		v.Self = me.ID
	}
	for _, usr := range all {
		if v.Query == "" || containsFold(usr.Username, v.Query) || containsFold(usr.FullName, v.Query) || containsFold(usr.Email, v.Query) {
			v.Rows = append(v.Rows, usr)
		}
	}
	p := u.withProjects(c, u.page(c, "Users", views.NavUsers, views.Crumb{Label: "Users"}))
	render(c, http.StatusOK, views.UsersPage(p, v))
}

func (u *ui) user(c *gin.Context) {
	usr, err := u.deps.Service.User(c, c.Param("id"))
	if err != nil {
		u.fail(c, err)
		return
	}
	p := u.withProjects(c, u.page(c, usr.Username, views.NavUsers, u.usersCrumbs(views.Crumb{Label: usr.Username, Mono: true})...))
	render(c, http.StatusOK, views.UserPage(p, views.UserView{User: usr}))
}

func (u *ui) userModal(c *gin.Context, status int, f views.UserForm, back string) {
	u.renderModal(c, status, f.Modal, func(m views.Modal) templ.Component {
		f.Modal = m
		return views.UserDialog(f)
	}, back, f.Modal.Title)
}

// submitUser saves a user dialog; validation, conflict and last-admin errors
// re-open it with the message.
func (u *ui) submitUser(c *gin.Context, f views.UserForm, back string, save func() (string, string, error)) {
	to, msg, err := save()
	if err != nil {
		emsg, status, ok := formError(err)
		if !ok {
			u.fail(c, err)
			return
		}
		f.Error = emsg
		u.userModal(c, status, f, back)
		return
	}
	setFlash(c, "success", msg)
	redirect(c, u.url(to))
}

func userFromForm(c *gin.Context) (service.UserInput, domain.User) {
	in := service.UserInput{
		Username: strings.TrimSpace(c.PostForm("username")),
		Password: c.PostForm("password"),
		Email:    strings.TrimSpace(c.PostForm("email")),
		FullName: strings.TrimSpace(c.PostForm("fullName")),
		Status:   domain.UserStatus(c.PostForm("status")),
		Role:     domain.Role(c.PostForm("role")),
	}
	return in, domain.User{Username: in.Username, Email: in.Email, FullName: in.FullName, Status: in.Status, Role: in.Role}
}

func (u *ui) userCreate(c *gin.Context) {
	back := u.url("/web/user")
	f := views.UserForm{
		Modal:  views.Modal{Title: "New user", Size: "md", Action: u.url("/web/user/new"), Submit: "Create user"},
		User:   domain.User{Role: domain.RoleReader, Status: domain.UserActive},
		Create: true,
	}
	if !isPost(c) {
		u.userModal(c, http.StatusOK, f, back)
		return
	}
	in, echo := userFromForm(c)
	f.User = echo
	u.submitUser(c, f, back, func() (string, string, error) {
		if in.Password == "" {
			return "", "", &service.ValidationError{Field: "password", Message: "password cannot be empty"}
		}
		usr, err := u.deps.Service.CreateUser(c, in)
		return views.UserPath(usr.ID), "User " + in.Username + " created", err
	})
}

func (u *ui) userUpdate(c *gin.Context) {
	usr, err := u.deps.Service.User(c, c.Param("id"))
	if err != nil {
		u.fail(c, err)
		return
	}
	back := u.url(views.UserPath(usr.ID))
	f := views.UserForm{Modal: views.Modal{Title: "Update user", Size: "md", Action: back + "/update", Submit: "Update user"}, User: usr}
	if !isPost(c) {
		u.userModal(c, http.StatusOK, f, back)
		return
	}
	in, echo := userFromForm(c)
	echo.ID = usr.ID
	f.User = echo
	u.submitUser(c, f, back, func() (string, string, error) {
		// An empty password keeps the stored one (D-2).
		_, err := u.deps.Service.UpdateUser(c, usr.ID, in)
		return views.UserPath(usr.ID), "User updated", err
	})
}

func (u *ui) userDelete(c *gin.Context) {
	usr, err := u.deps.Service.User(c, c.Param("id"))
	if err != nil {
		u.fail(c, err)
		return
	}
	cf := views.Confirm{
		Modal:   views.Modal{Title: "Delete user", Action: u.url(views.UserPath(usr.ID) + "/delete"), Submit: "Delete user"},
		Message: "Delete user " + usr.Username + "? They will no longer be able to sign in.",
	}
	if isPost(c) {
		err := u.deps.Service.DeleteUser(c, usr.ID)
		if err == nil {
			setFlash(c, "success", "User "+usr.Username+" deleted")
			redirect(c, u.url("/web/user"))
			return
		}
		msg, _, ok := formError(err)
		if !ok {
			u.fail(c, err)
			return
		}
		cf.Error = msg
	}
	status := http.StatusOK
	if cf.Error != "" {
		status = http.StatusUnprocessableEntity
	}
	u.renderModal(c, status, cf.Modal, func(m views.Modal) templ.Component {
		cf.Modal = m
		return views.ConfirmDialog(cf)
	}, u.url(views.UserPath(usr.ID)), cf.Modal.Title)
}

func (u *ui) usersBulk() bulkSpec {
	s := u.deps.Service
	return bulkSpec{
		noun: "users",
		candidates: func(c *gin.Context) ([]bulkItem, string, error) {
			all, err := s.Users(c)
			var items []bulkItem
			for _, usr := range all {
				items = append(items, bulkItem{usr.ID, usr.Username})
			}
			return items, u.url("/web/user"), err
		},
		remove:     func(ctx context.Context, id string) error { return s.DeleteUser(ctx, id) },
		removeHint: "They will no longer be able to sign in.",
	}
}
