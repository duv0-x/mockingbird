package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/service"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token    string      `json:"token"`
	Username string      `json:"username"`
	Role     domain.Role `json:"role"`
}

func (h *handlers) login(c *gin.Context) {
	var req loginRequest
	if !h.bind(c, &req) {
		return
	}
	token, u, err := h.deps.Auth.Login(c.Request.Context(), req.Username, req.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		c.JSON(http.StatusUnauthorized, auth.NewErrorBody(c, http.StatusUnauthorized))
		return
	}
	if err != nil {
		h.fail(c, err)
		return
	}
	auth.SetSessionCookie(c, token)
	c.PureJSON(http.StatusOK, loginResponse{Token: token, Username: u.Username, Role: u.Role})
}

func (h *handlers) logout(c *gin.Context) {
	auth.ClearSessionCookie(c)
	h.respondEmpty(c, nil)
}

func (h *handlers) version(c *gin.Context) {
	c.PureJSON(http.StatusOK, gin.H{"version": h.svc().Version()})
}

func (h *handlers) context(c *gin.Context) {
	c.PureJSON(http.StatusOK, gin.H{"context": h.deps.Config.ContextPath})
}

func (h *handlers) profile(c *gin.Context) {
	u, err := h.svc().Profile(c.Request.Context(), principal(c).ID)
	respond(h, c, u, err)
}

func (h *handlers) updateProfile(c *gin.Context) {
	var in service.ProfileInput
	if !h.bind(c, &in) {
		return
	}
	u, err := h.svc().UpdateProfile(c.Request.Context(), principal(c).ID, in)
	respond(h, c, u, err)
}

func (h *handlers) projects(c *gin.Context) {
	p, err := h.svc().Projects(c.Request.Context())
	respond(h, c, p, err)
}

func (h *handlers) system(c *gin.Context) {
	info, err := h.svc().SystemInformation(c.Request.Context())
	respond(h, c, info, err)
}

func (h *handlers) events(c *gin.Context) {
	e, err := h.svc().Events(c.Request.Context())
	respond(h, c, e, err)
}

func (h *handlers) clearEvents(c *gin.Context) {
	h.respondEmpty(c, h.svc().ClearEvents(c.Request.Context()))
}

type validateExpressionRequest struct {
	RequestBody  string `json:"requestBody"`
	ResponseBody string `json:"responseBody"`
}

func (h *handlers) validateExpression(c *gin.Context) {
	var req validateExpressionRequest
	if !h.bind(c, &req) {
		return
	}
	out, err := h.svc().ValidateExpression(c.Request.Context(), req.RequestBody, req.ResponseBody)
	respond(h, c, gin.H{"output": out}, err)
}

func (h *handlers) users(c *gin.Context) {
	u, err := h.svc().Users(c.Request.Context())
	respond(h, c, u, err)
}

func (h *handlers) user(c *gin.Context) {
	u, err := h.svc().User(c.Request.Context(), c.Param("userId"))
	respond(h, c, u, err)
}

func (h *handlers) createUser(c *gin.Context) {
	var in service.UserInput
	if !h.bind(c, &in) {
		return
	}
	u, err := h.svc().CreateUser(c.Request.Context(), in)
	respond(h, c, u, err)
}

func (h *handlers) updateUser(c *gin.Context) {
	var in service.UserInput
	if !h.bind(c, &in) {
		return
	}
	u, err := h.svc().UpdateUser(c.Request.Context(), c.Param("userId"), in)
	respond(h, c, u, err)
}

func (h *handlers) deleteUser(c *gin.Context) {
	h.respondEmpty(c, h.svc().DeleteUser(c.Request.Context(), c.Param("userId")))
}
