package definition

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// ramlMethods are the method keys of a RAML resource.
var ramlMethods = []string{"get", "patch", "put", "post", "delete", "options", "head", "trace", "connect"}

// convertRAML ports RAMLRestDefinitionConverter with RAML08Parser and
// RAML10Parser. Only the structure CastleMock reads is supported: nested
// resources, methods, responses with body examples and header defaults.
// Resource types, traits and !include are not expanded.
func convertRAML(data []byte, _ string, generate bool) ([]domain.RestApplication, error) {
	text := strings.TrimPrefix(string(data), "\ufeff")
	header, _, _ := strings.Cut(text, "\n")
	header = strings.TrimSpace(header)
	var v10 bool
	switch {
	case strings.HasPrefix(header, "#%RAML 0.8"):
	case strings.HasPrefix(header, "#%RAML 1.0"):
		v10 = true
	default:
		return nil, errors.New("unable to parse the RAML file")
	}
	root, err := parseTree([]byte(text))
	if err != nil {
		return nil, errors.Join(errors.New("unable to parse the RAML file"), err)
	}
	title, ok := str(get(root, "title"))
	if !ok {
		title, ok = str(lookup(root, "title", "value"))
	}
	if !ok {
		return nil, errors.New("unable to parse the RAML file: missing title")
	}
	app := domain.RestApplication{Name: title}
	var walkErr error
	ramlResources(root, "", v10, generate, &app.Resources, &walkErr)
	if walkErr != nil {
		return nil, walkErr
	}
	return []domain.RestApplication{app}, nil
}

func ramlResources(parent *yaml.Node, base string, v10, generate bool, out *[]domain.RestResource, walkErr *error) {
	for _, e := range entries(parent) {
		if !strings.HasPrefix(e.key, "/") {
			continue
		}
		uri := base + e.key
		var methods []domain.RestMethod
		for _, m := range entries(e.value) {
			if !slices.Contains(ramlMethods, m.key) {
				continue
			}
			httpMethod, err := domain.ParseHTTPMethod(m.key)
			if err != nil {
				*walkErr = errors.Join(errors.New("unsupported REST method "+m.key), err)
				return
			}
			method := domain.RestMethod{
				Name:             string(httpMethod),
				HTTPMethod:       httpMethod,
				Status:           domain.RestMethodMocked,
				ResponseStrategy: domain.RestStrategyRandom,
			}
			if v10 {
				method.Name = strOr(get(m.value, "displayName"), m.key)
			}
			if generate {
				method.MockResponses = ramlResponses(get(m.value, "responses"), v10)
			}
			methods = append(methods, method)
		}
		if len(methods) > 0 {
			name := uri
			if v10 {
				name = strOr(get(e.value, "displayName"), e.key)
			}
			*out = append(*out, domain.RestResource{Name: name, URI: uri, Methods: methods})
		}
		ramlResources(e.value, uri, v10, generate, out, walkErr)
		if *walkErr != nil {
			return
		}
	}
}

func ramlResponses(responses *yaml.Node, v10 bool) []domain.RestMockResponse {
	var out []domain.RestMockResponse
	for i, r := range entries(responses) {
		code := statusCode(r.key)
		headers := []domain.HTTPHeader{}
		for _, h := range entries(get(r.value, "headers")) {
			headers = append(headers, domain.HTTPHeader{Name: h.key, Value: strOr(get(h.value, "default"), "")})
		}
		out = append(out, domain.RestMockResponse{
			Name:           "Response " + strconv.Itoa(i+1),
			HTTPStatusCode: code,
			Status:         responseStatus(code),
			Body:           ramlExample(get(r.value, "body"), v10),
			HTTPHeaders:    headers,
		})
	}
	return out
}

// ramlExample returns the example of the first body. A RAML 1.0 body can be
// declared without media type; structured examples are rendered as JSON.
func ramlExample(body *yaml.Node, v10 bool) string {
	bodies := entries(body)
	if len(bodies) == 0 {
		return ""
	}
	decl := bodies[0].value
	if v10 && !strings.Contains(bodies[0].key, "/") {
		decl = body
	}
	example := get(decl, "example")
	if example == nil {
		return ""
	}
	if v10 {
		if v := get(example, "value"); v != nil && example.Kind == yaml.MappingNode {
			example = v
		}
	}
	if s, ok := str(example); ok {
		return s
	}
	return toJSON(example)
}
