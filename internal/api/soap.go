package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/service"
)

// Projects.

func (h *handlers) createSoapProject(c *gin.Context) {
	var in service.ProjectInput
	if !h.bind(c, &in) {
		return
	}
	p, err := h.svc().CreateSoapProject(c.Request.Context(), in)
	respond(h, c, p, err)
}

func (h *handlers) soapProject(c *gin.Context) {
	p, err := h.svc().SoapProject(c.Request.Context(), c.Param("projectId"))
	respond(h, c, p, err)
}

func (h *handlers) updateSoapProject(c *gin.Context) {
	var in service.ProjectInput
	if !h.bind(c, &in) {
		return
	}
	p, err := h.svc().UpdateSoapProject(c.Request.Context(), c.Param("projectId"), in)
	respond(h, c, p, err)
}

func (h *handlers) deleteSoapProject(c *gin.Context) {
	p, err := h.svc().DeleteSoapProject(c.Request.Context(), c.Param("projectId"))
	respond(h, c, p, err)
}

func (h *handlers) soapPortsStatus(c *gin.Context) {
	h.bulk(c, func(req idsRequest) error {
		return h.svc().SetSoapPortsStatus(c.Request.Context(), req.PortIDs, domain.SoapOperationStatus(req.Status))
	})
}

func (h *handlers) soapPortsEndpoint(c *gin.Context) {
	h.bulk(c, func(req idsRequest) error {
		return h.svc().SetSoapPortsForwardedEndpoint(c.Request.Context(), req.PortIDs, req.ForwardedEndpoint)
	})
}

// wsdlFile never follows imports: includeImports only applies to links.
func (h *handlers) wsdlFile(c *gin.Context) {
	src, ok := h.upload(c)
	if !ok {
		return
	}
	generate, err := formBool(c, "generateResponse")
	if err != nil {
		h.badRequest(c, err)
		return
	}
	h.respondEmpty(c, h.svc().ImportWSDL(c.Request.Context(), c.Param("projectId"), src, generate, false))
}

func (h *handlers) wsdlLink(c *gin.Context) {
	var req linkRequest
	if !h.bind(c, &req) {
		return
	}
	h.respondEmpty(c, h.svc().ImportWSDL(c.Request.Context(), c.Param("projectId"),
		service.Upload{URL: req.URL}, req.GenerateResponse, req.IncludeImports))
}

func (h *handlers) importSoapProject(c *gin.Context) {
	src, ok := h.upload(c)
	if !ok {
		return
	}
	p, err := h.svc().ImportSoapProject(c.Request.Context(), src)
	respond(h, c, p, err)
}

func (h *handlers) exportSoapProject(c *gin.Context) {
	exp, err := h.svc().ExportSoapProject(c.Request.Context(), c.Param("projectId"), exportFormat(c))
	h.export(c, exp, err)
}

// Ports.

func (h *handlers) soapPort(c *gin.Context) {
	p, err := h.svc().SoapPort(c.Request.Context(), c.Param("portId"))
	respond(h, c, p, err)
}

func (h *handlers) updateSoapPort(c *gin.Context) {
	var req nameRequest
	if !h.bind(c, &req) {
		return
	}
	p, err := h.svc().UpdateSoapPort(c.Request.Context(), c.Param("portId"), req.URI)
	respond(h, c, p, err)
}

func (h *handlers) deleteSoapPort(c *gin.Context) {
	p, err := h.svc().DeleteSoapPort(c.Request.Context(), c.Param("portId"))
	respond(h, c, p, err)
}

func (h *handlers) soapOperationsStatus(c *gin.Context) {
	h.bulk(c, func(req idsRequest) error {
		return h.svc().SetSoapOperationsStatus(c.Request.Context(), req.OperationIDs, domain.SoapOperationStatus(req.Status))
	})
}

func (h *handlers) soapOperationsEndpoint(c *gin.Context) {
	h.bulk(c, func(req idsRequest) error {
		return h.svc().SetSoapOperationsForwardedEndpoint(c.Request.Context(), req.OperationIDs, req.ForwardedEndpoint)
	})
}

// Operations.

func (h *handlers) soapOperation(c *gin.Context) {
	o, err := h.svc().SoapOperation(c.Request.Context(), c.Param("operationId"))
	respond(h, c, o, err)
}

func (h *handlers) updateSoapOperation(c *gin.Context) {
	var in service.SoapOperationUpdate
	if !h.bind(c, &in) {
		return
	}
	o, err := h.svc().UpdateSoapOperation(c.Request.Context(), c.Param("operationId"), in)
	respond(h, c, o, err)
}

func (h *handlers) soapMockResponsesStatus(c *gin.Context) {
	h.bulk(c, func(req idsRequest) error {
		return h.svc().SetSoapMockResponsesStatus(c.Request.Context(), req.MockResponseIDs, domain.SoapMockResponseStatus(req.Status))
	})
}

// Mock responses.

func (h *handlers) createSoapMockResponse(c *gin.Context) {
	var in service.SoapMockResponseInput
	if !h.bind(c, &in) {
		return
	}
	mr, err := h.svc().CreateSoapMockResponse(c.Request.Context(), c.Param("operationId"), in)
	respond(h, c, mr, err)
}

func (h *handlers) duplicateSoapMockResponses(c *gin.Context) {
	h.bulk(c, func(req idsRequest) error {
		return h.svc().DuplicateSoapMockResponses(c.Request.Context(), req.MockResponseIDs)
	})
}

func (h *handlers) soapMockResponse(c *gin.Context) {
	mr, err := h.svc().SoapMockResponse(c.Request.Context(), c.Param("mockResponseId"))
	respond(h, c, mr, err)
}

func (h *handlers) updateSoapMockResponse(c *gin.Context) {
	var in service.SoapMockResponseInput
	if !h.bind(c, &in) {
		return
	}
	mr, err := h.svc().UpdateSoapMockResponse(c.Request.Context(), c.Param("mockResponseId"), in)
	respond(h, c, mr, err)
}

func (h *handlers) deleteSoapMockResponse(c *gin.Context) {
	mr, err := h.svc().DeleteSoapMockResponse(c.Request.Context(), c.Param("mockResponseId"))
	respond(h, c, mr, err)
}

// Resources and events.

func (h *handlers) soapResource(c *gin.Context) {
	r, err := h.svc().SoapResource(c.Request.Context(), c.Param("resourceId"))
	respond(h, c, r, err)
}

func (h *handlers) soapResourceContent(c *gin.Context) {
	content, err := h.svc().SoapResourceContent(c.Request.Context(), c.Param("resourceId"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Data(http.StatusOK, "text/plain;charset=UTF-8", []byte(content))
}

// soapResourceRaw serves {ctx}/resource/soap/project/{p}/resource/{r}
// publicly, as CastleMock did for WSDL imports referenced by URL (SPEC.md §4.7).
func (h *handlers) soapResourceRaw(c *gin.Context) {
	content, err := h.svc().SoapResourceContent(c.Request.Context(), c.Param("resourceId"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Data(http.StatusOK, "text/xml;charset=UTF-8", []byte(content))
}

func (h *handlers) importSoapResource(c *gin.Context) {
	src, ok := h.upload(c)
	if !ok {
		return
	}
	r, err := h.svc().ImportSoapResource(c.Request.Context(), c.Param("projectId"), domain.SoapResourceType(c.PostForm("resourceType")), src)
	respond(h, c, r, err)
}

func (h *handlers) soapEvent(c *gin.Context) {
	e, err := h.svc().SoapEvent(c.Request.Context(), c.Param("eventId"))
	respond(h, c, e, err)
}
