// Package xml is the minimal XML element tree used by the SOAP mock engine and
// the WSDL importer. Names keep their raw prefix exactly as written (Java
// DocumentBuilderFactory defaults), while namespace resolution follows standard
// XML scoping: the innermost declaration wins (SPEC.md D-10).
package xml

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Attr is an attribute with its qualified name as written ("xmlns:soapenv",
// "location").
type Attr struct {
	Name  string
	Value string
}

// Element is an XML element. Text content is not kept; the engine and the WSDL
// importer only need structure and attributes.
type Element struct {
	Name     string // qualified name as written ("soapenv:Body")
	Attrs    []Attr
	Children []*Element
	Parent   *Element
}

// Parse reads a single-rooted, well-formed XML document and returns its root
// element. Any malformed input is an error (Java DOM parse semantics).
func Parse(data string) (*Element, error) {
	decoder := xml.NewDecoder(strings.NewReader(data))
	// The request body / uploaded file was already decoded to Go text; accept
	// any declared charset instead of failing on e.g. ISO-8859-1 envelopes.
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }

	var root, current *Element
	for {
		token, err := decoder.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("xml: parse: %w", err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			if current == nil && root != nil {
				return nil, fmt.Errorf("xml: parse: multiple root elements")
			}
			element := &Element{Name: qname(t.Name), Parent: current}
			for _, attr := range t.Attr {
				element.Attrs = append(element.Attrs, Attr{Name: qname(attr.Name), Value: attr.Value})
			}
			if current != nil {
				current.Children = append(current.Children, element)
			} else {
				root = element
			}
			current = element
		case xml.EndElement:
			if current == nil || current.Name != qname(t.Name) {
				return nil, fmt.Errorf("xml: parse: unexpected end element %s", qname(t.Name))
			}
			current = current.Parent
		case xml.CharData:
			if current == nil && strings.TrimSpace(string(t)) != "" {
				return nil, fmt.Errorf("xml: parse: content outside the root element")
			}
		}
		// Directives, comments and processing instructions are ignored.
	}
	if root == nil {
		return nil, fmt.Errorf("xml: parse: no root element")
	}
	if current != nil {
		return nil, fmt.Errorf("xml: parse: unclosed element %s", current.Name)
	}
	return root, nil
}

// qname reconstructs the qualified name from a RawToken name, whose Space field
// holds the unresolved prefix.
func qname(name xml.Name) string {
	if name.Space != "" {
		return name.Space + ":" + name.Local
	}
	return name.Local
}

// SplitQName splits a qualified name into prefix and local name.
func SplitQName(name string) (prefix, local string) {
	if i := strings.IndexByte(name, ':'); i >= 0 {
		return name[:i], name[i+1:]
	}
	return "", name
}

// LocalName returns the element name without its prefix.
func (e *Element) LocalName() string {
	_, local := SplitQName(e.Name)
	return local
}

// Prefix returns the namespace prefix of the element name, "" if unprefixed.
func (e *Element) Prefix() string {
	prefix, _ := SplitQName(e.Name)
	return prefix
}

// Attribute returns the value of the attribute whose qualified name equals
// name (plain string match, like DOM's getAttribute).
func (e *Element) Attribute(name string) (string, bool) {
	for _, attr := range e.Attrs {
		if attr.Name == name {
			return attr.Value, true
		}
	}
	return "", false
}

// Namespace resolves the namespace URI bound to the element's prefix (or the
// default namespace for unprefixed elements) with standard XML scoping: the
// innermost declaration along the ancestor chain wins (SPEC.md D-10). An
// unbound prefix or an absent default namespace yields "".
func (e *Element) Namespace() string {
	want := "xmlns"
	if prefix := e.Prefix(); prefix != "" {
		want = "xmlns:" + prefix
	}
	for node := e; node != nil; node = node.Parent {
		if value, ok := node.Attribute(want); ok {
			return value
		}
	}
	return ""
}

// Descendants returns the descendant elements (document order, self excluded)
// whose resolved namespace URI equals ns and whose local name equals local
// (getElementsByTagNameNS semantics).
func (e *Element) Descendants(ns, local string) []*Element {
	var out []*Element
	var walk func(node *Element)
	walk = func(node *Element) {
		for _, child := range node.Children {
			if child.Namespace() == ns && child.LocalName() == local {
				out = append(out, child)
			}
			walk(child)
		}
	}
	walk(e)
	return out
}
