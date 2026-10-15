package definition

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// rawElement is a node of a non namespace aware DOM: names keep their
// prefix, as Element.getElementsByTagName sees them.
type rawElement struct {
	name     string
	attrs    map[string]string
	children []*rawElement
}

func parseRawXML(data []byte) (*rawElement, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	doc := &rawElement{}
	stack := []*rawElement{doc}
	for {
		tok, err := dec.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			el := &rawElement{name: qualified(t.Name), attrs: map[string]string{}}
			for _, a := range t.Attr {
				el.attrs[qualified(a.Name)] = a.Value
			}
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, el)
			stack = append(stack, el)
		case xml.EndElement:
			if len(stack) == 1 || stack[len(stack)-1].name != qualified(t.Name) {
				return nil, fmt.Errorf("unexpected end element %s", qualified(t.Name))
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) != 1 || len(doc.children) != 1 {
		return nil, errors.New("malformed XML document")
	}
	return doc, nil
}

func qualified(n xml.Name) string {
	if n.Space == "" {
		return n.Local
	}
	return n.Space + ":" + n.Local
}

// descendants mirrors getElementsByTagName: every element below e with that
// exact tag name, in document order.
func (e *rawElement) descendants(name string) []*rawElement {
	var out []*rawElement
	for _, c := range e.children {
		if c.name == name {
			out = append(out, c)
		}
		out = append(out, c.descendants(name)...)
	}
	return out
}

// convertWADL ports WADLRestDefinitionConverter. Quirks kept: the
// application is named after the file, every <resource> below the
// application is a resource (nested ones are not joined to their parent),
// a resource gets every <method> below it, and the URI is the path of the
// resources base URL followed by the resource path.
func convertWADL(data []byte, filename string, generate bool) ([]domain.RestApplication, error) {
	doc, err := parseRawXML(data)
	if err != nil {
		return nil, fmt.Errorf("unable parse WADL file: %w", err)
	}
	var apps []domain.RestApplication
	for _, appEl := range doc.descendants("application") {
		base, err := resourceBase(appEl)
		if err != nil {
			return nil, err
		}
		app := domain.RestApplication{Name: strings.ReplaceAll(filename, ".wadl", "")}
		for _, resEl := range appEl.descendants("resource") {
			resPath := resEl.attrs["path"]
			res := domain.RestResource{Name: resPath, URI: base + resPath}
			for _, methodEl := range resEl.descendants("method") {
				httpMethod := domain.HTTPMethod(methodEl.attrs["name"])
				if _, err := domain.ParseHTTPMethod(string(httpMethod)); err != nil || strings.ToUpper(string(httpMethod)) != string(httpMethod) {
					return nil, fmt.Errorf("unable parse WADL file: unknown method %q", methodEl.attrs["name"])
				}
				m := domain.RestMethod{
					Name:                 methodEl.attrs["id"],
					HTTPMethod:           httpMethod,
					Status:               domain.RestMethodMocked,
					ResponseStrategy:     domain.RestStrategyRandom,
					SimulateNetworkDelay: domain.Ptr(false),
					AutomaticForward:     domain.Ptr(false),
				}
				if generate {
					m.MockResponses = []domain.RestMockResponse{generatedResponse()}
				}
				res.Methods = append(res.Methods, m)
			}
			app.Resources = append(app.Resources, res)
		}
		apps = append(apps, app)
	}
	return apps, nil
}

// resourceBase is the path of <resources base="...">; a base that is not a
// URL is ignored, a relative one fails like URI.toURL.
func resourceBase(appEl *rawElement) (string, error) {
	resources := appEl.descendants("resources")
	if len(resources) == 0 {
		return "", nil
	}
	base := resources[0].attrs["base"]
	if base == "" {
		return "", nil
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", nil
	}
	if !u.IsAbs() {
		return "", fmt.Errorf("unable parse WADL file: resources base %q is not absolute", base)
	}
	return u.EscapedPath(), nil
}
