package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// openAPI serves {ctx}/doc/api/rest: an OpenAPI 3 description generated from
// Routes(), so it cannot drift from the router. CastleMock redirected to a
// Swagger UI page instead (SPEC.md §5.3).
func (h *handlers) openAPI(c *gin.Context) {
	c.PureJSON(http.StatusOK, OpenAPI(h.deps.Config.ContextPath, h.svc().Version()))
}

// OpenAPI builds the document for the given context path.
func OpenAPI(contextPath, version string) map[string]any {
	paths := map[string]map[string]any{}
	for _, r := range Routes() {
		path, params := openAPIPath(r.Path)
		op := map[string]any{
			"summary":     r.Summary,
			"tags":        []string{openAPITag(r.Path)},
			"description": accessDescription(r.Access),
			"responses":   openAPIResponses(r),
		}
		if len(params) > 0 {
			op["parameters"] = params
		}
		if r.Access == Public {
			op["security"] = []any{}
		}
		if body := openAPIRequestBody(r); body != nil {
			op["requestBody"] = body
		}
		if paths[path] == nil {
			paths[path] = map[string]any{}
		}
		paths[path][strings.ToLower(r.Method)] = op
	}
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "mockingbird admin API",
			"version":     version,
			"description": "CastleMock-compatible admin REST API. Authenticate with POST /api/rest/core/login (cookie \"token\"), a Bearer token or HTTP Basic.",
		},
		"servers": []any{map[string]any{"url": contextPath}},
		"paths":   paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"cookie": map[string]any{"type": "apiKey", "in": "cookie", "name": "token"},
				"bearer": map[string]any{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"},
				"basic":  map[string]any{"type": "http", "scheme": "basic"},
			},
		},
		"security": []any{
			map[string]any{"cookie": []string{}},
			map[string]any{"bearer": []string{}},
			map[string]any{"basic": []string{}},
		},
	}
}

// openAPIPath turns gin's /a/:id into /a/{id} and lists the path parameters.
func openAPIPath(p string) (string, []any) {
	segs := strings.Split(p, "/")
	var params []any
	for i, s := range segs {
		if name, ok := strings.CutPrefix(s, ":"); ok {
			segs[i] = "{" + name + "}"
			params = append(params, map[string]any{
				"name": name, "in": "path", "required": true, "schema": map[string]any{"type": "string"},
			})
		}
	}
	return strings.Join(segs, "/"), params
}

func openAPITag(p string) string {
	switch {
	case strings.HasPrefix(p, "/api/rest/core/"):
		return "Core"
	case strings.HasPrefix(p, "/api/rest/rest/"):
		return "REST"
	default:
		return "SOAP"
	}
}

func accessDescription(a Access) string {
	switch a {
	case Reader:
		return "Required authorization: Reader, Modifier or Admin."
	case Modifier:
		return "Required authorization: Modifier or Admin."
	case Admin:
		return "Required authorization: Admin."
	}
	return "Public."
}

func openAPIResponses(r Route) map[string]any {
	resp := map[string]any{"200": map[string]any{"description": "OK"}}
	if r.Access != Public {
		resp["401"] = map[string]any{"description": "Not authenticated"}
		resp["403"] = map[string]any{"description": "Role not allowed"}
	}
	if strings.Contains(r.Path, ":") {
		resp["404"] = map[string]any{"description": "Not found"}
	}
	if r.Method != http.MethodGet && r.Method != http.MethodDelete {
		resp["400"] = map[string]any{"description": "Invalid input"}
	}
	return resp
}

func openAPIRequestBody(r Route) map[string]any {
	if r.Method == http.MethodGet || r.Method == http.MethodDelete || r.Path == "/api/rest/core/logout" {
		return nil
	}
	if strings.HasSuffix(r.Path, "/import") || strings.HasSuffix(r.Path, "/file") {
		props := map[string]any{"file": map[string]any{"type": "string", "format": "binary"}}
		if strings.HasSuffix(r.Path, "/file") {
			props["generateResponse"] = map[string]any{"type": "boolean"}
		}
		if strings.HasSuffix(r.Path, "/definition/file") {
			props["definitionType"] = map[string]any{"type": "string", "enum": []string{"SWAGGER", "OPENAPI", "WADL", "RAML"}}
		}
		if strings.HasSuffix(r.Path, "/resource/:resourceId/import") {
			props["resourceType"] = map[string]any{"type": "string", "enum": []string{"WSDL", "SCHEMA", "WSDL_IMPORT"}}
		}
		return map[string]any{"required": true, "content": map[string]any{
			"multipart/form-data": map[string]any{"schema": map[string]any{"type": "object", "properties": props, "required": []string{"file"}}},
		}}
	}
	return map[string]any{"content": map[string]any{
		"application/json": map[string]any{"schema": map[string]any{"type": "object"}},
	}}
}
