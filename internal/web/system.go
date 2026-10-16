package web

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/web/views"
)

// registerSystem mounts the System information screen, ADMIN only (SPEC §8.2).
func (u *ui) registerSystem(rt routes) {
	rt.admin.GET("/system", u.system)
}

func (u *ui) system(c *gin.Context) {
	info, err := u.deps.Service.SystemInformation(c)
	if err != nil {
		u.fail(c, err)
		return
	}
	p := u.withProjects(c, u.page(c, "System", views.NavSystem, views.Crumb{Label: "System"}))
	render(c, http.StatusOK, views.SystemPage(p, info))
}
