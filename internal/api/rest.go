package api

import (
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/service"
)

// Bulk request bodies (Update*StatusRequest, Update*ForwardedEndpointRequest).
type idsRequest struct {
	ApplicationIDs    []string `json:"applicationIds"`
	ResourceIDs       []string `json:"resourceIds"`
	MethodIDs         []string `json:"methodIds"`
	MockResponseIDs   []string `json:"mockResponseIds"`
	PortIDs           []string `json:"portIds"`
	OperationIDs      []string `json:"operationIds"`
	Status            string   `json:"status"`
	ForwardedEndpoint string   `json:"forwardedEndpoint"`
}

type nameRequest struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

type linkRequest struct {
	URL              string                    `json:"url"`
	GenerateResponse bool                      `json:"generateResponse"`
	DefinitionType   domain.RestDefinitionType `json:"definitionType"`
	IncludeImports   bool                      `json:"includeImports"`
}

func (h *handlers) bulk(c *gin.Context, apply func(req idsRequest) error) {
	var req idsRequest
	if !h.bind(c, &req) {
		return
	}
	h.respondEmpty(c, apply(req))
}

func (h *handlers) export(c *gin.Context, exp service.Export, err error) {
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": exp.Filename}))
	c.Data(http.StatusOK, exp.ContentType, exp.Data)
}

func exportFormat(c *gin.Context) service.ExportFormat {
	if f := c.Query("format"); f != "" {
		return service.ExportFormat(f)
	}
	return service.ExportCastleMockXML
}

// Projects.

func (h *handlers) createRestProject(c *gin.Context) {
	var in service.ProjectInput
	if !h.bind(c, &in) {
		return
	}
	p, err := h.svc().CreateRestProject(c.Request.Context(), in)
	respond(h, c, p, err)
}

func (h *handlers) restProject(c *gin.Context) {
	p, err := h.svc().RestProject(c.Request.Context(), c.Param("projectId"))
	respond(h, c, p, err)
}

func (h *handlers) updateRestProject(c *gin.Context) {
	var in service.ProjectInput
	if !h.bind(c, &in) {
		return
	}
	p, err := h.svc().UpdateRestProject(c.Request.Context(), c.Param("projectId"), in)
	respond(h, c, p, err)
}

func (h *handlers) deleteRestProject(c *gin.Context) {
	p, err := h.svc().DeleteRestProject(c.Request.Context(), c.Param("projectId"))
	respond(h, c, p, err)
}

func (h *handlers) restApplicationsStatus(c *gin.Context) {
	h.bulk(c, func(req idsRequest) error {
		return h.svc().SetRestApplicationsStatus(c.Request.Context(), req.ApplicationIDs, domain.RestMethodStatus(req.Status))
	})
}

func (h *handlers) restApplicationsEndpoint(c *gin.Context) {
	h.bulk(c, func(req idsRequest) error {
		return h.svc().SetRestApplicationsForwardedEndpoint(c.Request.Context(), req.ApplicationIDs, req.ForwardedEndpoint)
	})
}

func (h *handlers) restDefinitionFile(c *gin.Context) {
	src, ok := h.upload(c)
	if !ok {
		return
	}
	generate, err := formBool(c, "generateResponse")
	if err != nil {
		h.badRequest(c, err)
		return
	}
	defType := domain.RestDefinitionType(c.PostForm("definitionType"))
	h.respondEmpty(c, h.svc().ImportRestDefinition(c.Request.Context(), c.Param("projectId"), defType, src, generate))
}

func (h *handlers) restDefinitionLink(c *gin.Context) {
	var req linkRequest
	if !h.bind(c, &req) {
		return
	}
	h.respondEmpty(c, h.svc().ImportRestDefinition(c.Request.Context(), c.Param("projectId"), req.DefinitionType,
		service.Upload{URL: req.URL}, req.GenerateResponse))
}

func (h *handlers) importRestProject(c *gin.Context) {
	src, ok := h.upload(c)
	if !ok {
		return
	}
	p, err := h.svc().ImportRestProject(c.Request.Context(), src)
	respond(h, c, p, err)
}

func (h *handlers) exportRestProject(c *gin.Context) {
	exp, err := h.svc().ExportRestProject(c.Request.Context(), c.Param("projectId"), exportFormat(c))
	h.export(c, exp, err)
}

// Applications.

func (h *handlers) createRestApplication(c *gin.Context) {
	var req nameRequest
	if !h.bind(c, &req) {
		return
	}
	a, err := h.svc().CreateRestApplication(c.Request.Context(), c.Param("projectId"), req.Name)
	respond(h, c, a, err)
}

func (h *handlers) restApplication(c *gin.Context) {
	a, err := h.svc().RestApplication(c.Request.Context(), c.Param("applicationId"))
	respond(h, c, a, err)
}

func (h *handlers) updateRestApplication(c *gin.Context) {
	var req nameRequest
	if !h.bind(c, &req) {
		return
	}
	a, err := h.svc().UpdateRestApplication(c.Request.Context(), c.Param("applicationId"), req.Name)
	respond(h, c, a, err)
}

func (h *handlers) deleteRestApplication(c *gin.Context) {
	a, err := h.svc().DeleteRestApplication(c.Request.Context(), c.Param("applicationId"))
	respond(h, c, a, err)
}

func (h *handlers) restResourcesStatus(c *gin.Context) {
	h.bulk(c, func(req idsRequest) error {
		return h.svc().SetRestResourcesStatus(c.Request.Context(), req.ResourceIDs, domain.RestMethodStatus(req.Status))
	})
}

func (h *handlers) restResourcesEndpoint(c *gin.Context) {
	h.bulk(c, func(req idsRequest) error {
		return h.svc().SetRestResourcesForwardedEndpoint(c.Request.Context(), req.ResourceIDs, req.ForwardedEndpoint)
	})
}

// Resources.

func (h *handlers) createRestResource(c *gin.Context) {
	var req nameRequest
	if !h.bind(c, &req) {
		return
	}
	r, err := h.svc().CreateRestResource(c.Request.Context(), c.Param("applicationId"), req.Name, req.URI)
	respond(h, c, r, err)
}

func (h *handlers) restResource(c *gin.Context) {
	r, err := h.svc().RestResource(c.Request.Context(), c.Param("resourceId"))
	respond(h, c, r, err)
}

func (h *handlers) updateRestResource(c *gin.Context) {
	var req nameRequest
	if !h.bind(c, &req) {
		return
	}
	r, err := h.svc().UpdateRestResource(c.Request.Context(), c.Param("resourceId"), req.Name, req.URI)
	respond(h, c, r, err)
}

func (h *handlers) deleteRestResource(c *gin.Context) {
	r, err := h.svc().DeleteRestResource(c.Request.Context(), c.Param("resourceId"))
	respond(h, c, r, err)
}

func (h *handlers) restResourceParameters(c *gin.Context) {
	p, err := h.svc().RestResourceParameters(c.Request.Context(), c.Param("resourceId"))
	respond(h, c, p, err)
}

func (h *handlers) restMethodsStatus(c *gin.Context) {
	h.bulk(c, func(req idsRequest) error {
		return h.svc().SetRestMethodsStatus(c.Request.Context(), req.MethodIDs, domain.RestMethodStatus(req.Status))
	})
}

func (h *handlers) restMethodsEndpoint(c *gin.Context) {
	h.bulk(c, func(req idsRequest) error {
		return h.svc().SetRestMethodsForwardedEndpoint(c.Request.Context(), req.MethodIDs, req.ForwardedEndpoint)
	})
}

// Methods.

func (h *handlers) createRestMethod(c *gin.Context) {
	var in service.RestMethodCreate
	if !h.bind(c, &in) {
		return
	}
	m, err := h.svc().CreateRestMethod(c.Request.Context(), c.Param("resourceId"), in)
	respond(h, c, m, err)
}

func (h *handlers) restMethod(c *gin.Context) {
	m, err := h.svc().RestMethod(c.Request.Context(), c.Param("methodId"))
	respond(h, c, m, err)
}

func (h *handlers) updateRestMethod(c *gin.Context) {
	var in service.RestMethodUpdate
	if !h.bind(c, &in) {
		return
	}
	m, err := h.svc().UpdateRestMethod(c.Request.Context(), c.Param("methodId"), in)
	respond(h, c, m, err)
}

func (h *handlers) deleteRestMethod(c *gin.Context) {
	m, err := h.svc().DeleteRestMethod(c.Request.Context(), c.Param("methodId"))
	respond(h, c, m, err)
}

func (h *handlers) restMockResponsesStatus(c *gin.Context) {
	h.bulk(c, func(req idsRequest) error {
		return h.svc().SetRestMockResponsesStatus(c.Request.Context(), req.MockResponseIDs, domain.RestMockResponseStatus(req.Status))
	})
}

// Mock responses.

func (h *handlers) createRestMockResponse(c *gin.Context) {
	var in service.RestMockResponseInput
	if !h.bind(c, &in) {
		return
	}
	mr, err := h.svc().CreateRestMockResponse(c.Request.Context(), c.Param("methodId"), in)
	respond(h, c, mr, err)
}

func (h *handlers) duplicateRestMockResponses(c *gin.Context) {
	h.bulk(c, func(req idsRequest) error {
		return h.svc().DuplicateRestMockResponses(c.Request.Context(), req.MockResponseIDs)
	})
}

func (h *handlers) restMockResponse(c *gin.Context) {
	mr, err := h.svc().RestMockResponse(c.Request.Context(), c.Param("mockResponseId"))
	respond(h, c, mr, err)
}

func (h *handlers) updateRestMockResponse(c *gin.Context) {
	var in service.RestMockResponseInput
	if !h.bind(c, &in) {
		return
	}
	mr, err := h.svc().UpdateRestMockResponse(c.Request.Context(), c.Param("mockResponseId"), in)
	respond(h, c, mr, err)
}

func (h *handlers) deleteRestMockResponse(c *gin.Context) {
	mr, err := h.svc().DeleteRestMockResponse(c.Request.Context(), c.Param("mockResponseId"))
	respond(h, c, mr, err)
}

func (h *handlers) restEvent(c *gin.Context) {
	e, err := h.svc().RestEvent(c.Request.Context(), c.Param("eventId"))
	respond(h, c, e, err)
}
