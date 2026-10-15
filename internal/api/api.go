// Package api implements the CastleMock-compatible JSON admin API under
// {ctx}/api/rest/**, plus {ctx}/doc/api/rest and {ctx}/resource/soap/** (SPEC.md §5).
// Handlers translate HTTP <-> internal/service; no business logic here.
//
// Owner: T4.
package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/config"
	"github.com/duv0-x/mockingbird/internal/service"
)

type Deps struct {
	Config  config.Config
	Service *service.Service
	Auth    *auth.Service
	Logger  *slog.Logger
}

// Register mounts the API on g, the router group of the context path. The
// auth middleware is already installed on g.
func Register(g *gin.RouterGroup, deps Deps) {
	notImplemented := func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"status": http.StatusNotImplemented, "error": "admin API not implemented (T4)", "path": c.Request.URL.Path})
	}
	g.Any("/api/rest/*path", notImplemented)
	g.GET("/doc/api/rest", notImplemented)
	g.GET("/resource/soap/project/:projectId/resource/:resourceId", notImplemented)
}
