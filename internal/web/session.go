package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/web/views"
)

func (u *ui) registerSession(rt routes) {
	rt.public.GET("/login", u.loginPage)
	rt.public.POST("/login", u.loginSubmit)
	rt.public.GET("/logout", u.logout)
}

func (u *ui) loginPage(c *gin.Context) {
	if _, ok := auth.PrincipalFrom(c); ok {
		c.Redirect(http.StatusFound, u.safeNext(c.Query("next")))
		return
	}
	render(c, http.StatusOK, views.Login(u.page(c, "Sign in", ""), views.LoginForm{Next: c.Query("next")}))
}

// loginSubmit mirrors POST /api/rest/core/login: same credentials check, same
// token cookie, then a redirect into the UI.
func (u *ui) loginSubmit(c *gin.Context) {
	username := strings.TrimSpace(c.PostForm("username"))
	next := c.PostForm("next")
	token, _, err := u.deps.Auth.Login(c, username, c.PostForm("password"))
	if err != nil {
		status := http.StatusUnauthorized
		msg := "Invalid username or password."
		if !errors.Is(err, auth.ErrInvalidCredentials) {
			u.deps.Logger.Error("ui login", "err", err)
			status, msg = http.StatusInternalServerError, "Sign in failed. Please try again."
		}
		render(c, status, views.Login(u.page(c, "Sign in", ""), views.LoginForm{Username: username, Next: next, Error: msg}))
		return
	}
	auth.SetSessionCookie(c, token)
	c.Redirect(http.StatusSeeOther, u.safeNext(next))
}

func (u *ui) logout(c *gin.Context) {
	auth.ClearSessionCookie(c)
	c.Redirect(http.StatusFound, u.loginPath())
}

// safeNext only follows redirects to UI paths of this server.
func (u *ui) safeNext(next string) string {
	home := u.url("/web")
	if next == "" || !strings.HasPrefix(next, home) || strings.HasPrefix(next, "//") || strings.ContainsAny(next, "\\\r\n") {
		return home
	}
	return next
}
