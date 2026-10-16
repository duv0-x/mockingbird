package castlemock

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// node is a parsed XML element. Both CastleMock XML surfaces (the JAXB data
// dir, SPEC.md §3.6.3, and the Jackson export files, §6.3) are
// order-insensitive containers of named child elements where unknown elements
// must be ignored, so a generic tree fits better than struct tags.
type node struct {
	name     string
	text     string
	children []*node
}

// parseXML reads a single-root XML document into a node tree. Namespaces are
// irrelevant in CastleMock files; only local names are kept.
func parseXML(data []byte) (*node, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var root *node
	var stack []*node
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &node{name: t.Name.Local}
			switch {
			case len(stack) > 0:
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, n)
			case root == nil:
				root = n
			default:
				return nil, errors.New("multiple root elements")
			}
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(t)
			}
		}
	}
	if root == nil {
		return nil, errors.New("empty XML document")
	}
	return root, nil
}

// child returns the first child element with the given name, or nil.
func (n *node) child(name string) *node {
	for _, c := range n.children {
		if c.name == name {
			return c
		}
	}
	return nil
}

// childrenNamed returns all child elements with the given name.
func (n *node) childrenNamed(name string) []*node {
	var out []*node
	for _, c := range n.children {
		if c.name == name {
			out = append(out, c)
		}
	}
	return out
}

// str returns the trimmed text of the named child ("" when absent).
func (n *node) str(name string) string {
	if c := n.child(name); c != nil {
		return strings.TrimSpace(c.text)
	}
	return ""
}

// raw returns the verbatim text of the named child ("" when absent). Used for
// payload fields (body, envelope, resource content) whose whitespace matters.
func (n *node) raw(name string) string {
	if c := n.child(name); c != nil {
		return c.text
	}
	return ""
}

// has reports whether a child with the given name exists, even if empty.
func (n *node) has(name string) bool {
	return n.child(name) != nil
}

func (n *node) intVal(name string) int {
	if s := n.str(name); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
	}
	return 0
}

// boolPtr maps an optional JAXB Boolean: absent element or empty text → nil.
func (n *node) boolPtr(name string) *bool {
	c := n.child(name)
	if c == nil {
		return nil
	}
	s := strings.TrimSpace(c.text)
	if s == "" {
		return nil
	}
	v := s == "true"
	return &v
}

// intPtr maps an optional JAXB Long: absent element or empty text → nil.
func (n *node) intPtr(name string) *int64 {
	c := n.child(name)
	if c == nil {
		return nil
	}
	s := strings.TrimSpace(c.text)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

// jaxbTime parses the xsd:dateTime used by the data-dir files: server local
// time with a numeric offset and an optional fraction
// (2023-02-01T10:15:30.123+01:00). Go's parser accepts the fractional second
// even though the layout does not mention it.
func jaxbTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02T15:04:05Z07:00", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse xsd:dateTime %q: %w", s, err)
	}
	return t.UTC(), nil
}

// exportTime parses the dates of export files: epoch milliseconds (Jackson's
// WRITE_DATES_AS_TIMESTAMPS default) or an ISO/xsd:dateTime string.
func exportTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.UnixMilli(ms).UTC(), nil
	}
	return jaxbTime(s)
}

// timeField reads an xsd:dateTime child element; absent or empty → zero time.
func (n *node) timeField(name string) (time.Time, error) {
	return jaxbTime(n.str(name))
}

// exportTimeField reads a millis-or-ISO child element; absent or empty → zero.
func (n *node) exportTimeField(name string) (time.Time, error) {
	return exportTime(n.str(name))
}

// listItems returns the item elements of a list property, accepting
// double-wrapped Jackson XML (<xs><xs>…</xs></xs>), a JAXB-style wrapper
// with distinct item names (<xs><x>…</x></xs>) and a wrapper that is itself
// a single item.
func listItems(parent *node, name string) []*node {
	var out []*node
	for _, w := range parent.childrenNamed(name) {
		if inner := w.childrenNamed(name); len(inner) > 0 {
			out = append(out, inner...)
			continue
		}
		out = append(out, w.children...)
	}
	return out
}

// stringList reads a list of scalar values (contentEncodings), accepting the
// same shapes as listItems plus repeated bare elements.
func stringList(parent *node, name string) []string {
	out := []string{}
	for _, w := range parent.childrenNamed(name) {
		if inner := w.childrenNamed(name); len(inner) > 0 {
			for _, i := range inner {
				out = append(out, strings.TrimSpace(i.text))
			}
			continue
		}
		if len(w.children) > 0 {
			for _, c := range w.children {
				out = append(out, strings.TrimSpace(c.text))
			}
			continue
		}
		if s := strings.TrimSpace(w.text); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// headers reads httpHeader{name,value} items out of a wrapper element.
func headers(parent *node, wrapper string) []headerValue {
	items := listItems(parent, wrapper)
	out := make([]headerValue, 0, len(items))
	for _, item := range items {
		out = append(out, headerValue{name: item.str("name"), value: item.str("value")})
	}
	return out
}

type headerValue struct {
	name  string
	value string
}
