package definition

import (
	"errors"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// operationMethods are the operations both Swagger converters read, in
// their order. PATCH and TRACE operations are ignored, like in CastleMock.
var operationMethods = []domain.HTTPMethod{
	domain.MethodGET, domain.MethodPOST, domain.MethodPUT, domain.MethodDELETE, domain.MethodHEAD, domain.MethodOPTIONS,
}

// openAPI is the subset of a parsed (and fully resolved) OpenAPI document
// the converter reads. A Swagger 2.0 document is accepted too, as
// swagger-parser's OpenAPIParser converts it to OpenAPI 3.
type openAPI struct {
	root    *yaml.Node
	servers []server
}

type server struct {
	url         string
	description *string
}

// convertOpenAPI ports OpenApiRestDefinitionConverter.
func convertOpenAPI(data []byte, _ string, generate bool) ([]domain.RestApplication, error) {
	root, err := parseTree(data)
	if err != nil {
		return nil, errors.Join(errors.New("unable to parse the OpenApi content"), err)
	}
	doc := openAPI{root: root}
	if _, isV2 := str(get(root, "swagger")); isV2 {
		doc.servers = swagger2Servers(root)
	} else if _, isV3 := str(get(root, "openapi")); isV3 {
		for _, s := range items(get(root, "servers")) {
			srv := server{url: strOr(get(s, "url"), "")}
			if d, ok := str(get(s, "description")); ok {
				srv.description = &d
			}
			doc.servers = append(doc.servers, srv)
		}
	} else {
		return nil, errors.New("unable to parse the OpenApi content")
	}
	// swagger-parser adds a "/" server when the document declares none.
	if len(doc.servers) == 0 {
		doc.servers = []server{{url: "/"}}
	}
	paths := get(root, "paths")
	if paths == nil {
		return nil, errors.New("the OpenApi content has no paths")
	}
	name, err := doc.applicationName()
	if err != nil {
		return nil, err
	}
	forward := doc.servers[0].url

	app := domain.RestApplication{Name: name}
	for _, p := range entries(paths) {
		item := deref(root, p.value)
		res := domain.RestResource{Name: p.key, URI: p.key}
		for _, httpMethod := range operationMethods {
			op := get(item, strings.ToLower(string(httpMethod)))
			if op == nil {
				continue
			}
			res.Methods = append(res.Methods, doc.method(op, httpMethod, forward, generate))
		}
		app.Resources = append(app.Resources, res)
	}
	return []domain.RestApplication{app}, nil
}

// swagger2Servers mirrors swagger-parser's v2 → v3 conversion of
// schemes/host/basePath into servers.
func swagger2Servers(root *yaml.Node) []server {
	host, hasHost := str(get(root, "host"))
	basePath := strOr(get(root, "basePath"), "")
	if !hasHost {
		if basePath == "" {
			return nil
		}
		return []server{{url: basePath}}
	}
	var schemes []string
	for _, s := range items(get(root, "schemes")) {
		if v, ok := str(s); ok {
			schemes = append(schemes, v)
		}
	}
	if len(schemes) == 0 {
		schemes = []string{"http"}
	}
	servers := make([]server, 0, len(schemes))
	for _, scheme := range schemes {
		servers = append(servers, server{url: scheme + "://" + host + basePath})
	}
	return servers
}

func (d openAPI) applicationName() (string, error) {
	info := get(d.root, "info")
	if title, ok := str(get(info, "title")); ok {
		return title, nil
	}
	if desc, ok := str(get(info, "description")); ok {
		return desc, nil
	}
	if s := d.servers[0]; s.description != nil {
		return *s.description, nil
	}
	if d.servers[0].url != "" {
		return d.servers[0].url, nil
	}
	return "", errors.New("unable to extract application name from the OpenApi content")
}

func (d openAPI) method(op *yaml.Node, httpMethod domain.HTTPMethod, forward string, generate bool) domain.RestMethod {
	m := domain.RestMethod{
		Name:                 operationName(op, httpMethod),
		HTTPMethod:           httpMethod,
		Status:               domain.RestMethodMocked,
		ResponseStrategy:     domain.RestStrategySequence,
		ForwardedEndpoint:    forward,
		SimulateNetworkDelay: domain.Ptr(false),
		NetworkDelay:         domain.Ptr(int64(0)),
		AutomaticForward:     domain.Ptr(false),
	}
	if !generate {
		return m
	}
	responses := entries(get(op, "responses"))
	if len(responses) == 0 {
		m.MockResponses = []domain.RestMockResponse{generatedResponse()}
		return m
	}
	for _, r := range responses {
		resp := deref(d.root, r.value)
		code := statusCode(r.key)
		headers := []domain.HTTPHeader{}
		for _, h := range entries(get(resp, "headers")) {
			headers = append(headers, domain.HTTPHeader{Name: h.key, Value: ""})
		}
		m.MockResponses = append(m.MockResponses, domain.RestMockResponse{
			Name:             strOr(get(resp, "description"), ""),
			HTTPStatusCode:   code,
			Status:           responseStatus(code),
			UsingExpressions: domain.Ptr(true),
			HTTPHeaders:      headers,
		})
	}
	return m
}

// operationName: operationId, then summary, then the HTTP method.
func operationName(op *yaml.Node, httpMethod domain.HTTPMethod) string {
	if id, ok := str(get(op, "operationId")); ok {
		return id
	}
	if summary, ok := str(get(op, "summary")); ok {
		return summary
	}
	return string(httpMethod)
}
