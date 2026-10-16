package web

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/service"
	"github.com/duv0-x/mockingbird/internal/web/views"
)

// registerProfile mounts the Profile screen (SPEC §8.2): every signed-in user
// can see and update their own account, but not their role or status.
func (u *ui) registerProfile(rt routes) {
	rt.read.GET("/profile", u.profile)
	u.dialog(rt.read, "/profile/update", u.profileUpdate)
}

func (u *ui) profile(c *gin.Context) {
	me, _ := auth.PrincipalFrom(c)
	usr, err := u.deps.Service.Profile(c, me.ID)
	if err != nil {
		u.fail(c, err)
		return
	}
	p := u.withProjects(c, u.page(c, "Profile", views.NavProfile, views.Crumb{Label: "Profile"}))
	render(c, http.StatusOK, views.UserPage(p, views.UserView{User: usr, Profile: true}))
}

func (u *ui) profileUpdate(c *gin.Context) {
	me, _ := auth.PrincipalFrom(c)
	usr, err := u.deps.Service.Profile(c, me.ID)
	if err != nil {
		u.fail(c, err)
		return
	}
	back := u.url("/web/profile")
	f := views.UserForm{Modal: views.Modal{Title: "Update profile", Size: "md", Action: back + "/update", Submit: "Update profile"}, User: usr, Profile: true}
	if !isPost(c) {
		u.userModal(c, http.StatusOK, f, back)
		return
	}
	in := service.ProfileInput{
		Username: strings.TrimSpace(c.PostForm("username")),
		Password: c.PostForm("password"),
		Email:    strings.TrimSpace(c.PostForm("email")),
		FullName: strings.TrimSpace(c.PostForm("fullName")),
	}
	f.User.Username, f.User.Email, f.User.FullName = in.Username, in.Email, in.FullName
	u.submitUser(c, f, back, func() (string, string, error) {
		_, err := u.deps.Service.UpdateProfile(c, me.ID, in)
		return "/web/profile", "Profile updated", err
	})
}
