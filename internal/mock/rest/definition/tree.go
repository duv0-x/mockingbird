package definition

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// parseTree parses JSON or YAML into a yaml.Node tree, which keeps the
// document order of keys (Swagger/OpenAPI parsers use LinkedHashMaps, so
// paths, responses and properties are converted in file order). JSON is
// decoded separately because yaml.v3 rejects JSON indented with tabs.
func parseTree(data []byte) (*yaml.Node, error) {
	text := strings.TrimSpace(strings.TrimPrefix(string(data), "\ufeff"))
	if text == "" {
		return nil, errors.New("empty document")
	}
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		dec := json.NewDecoder(strings.NewReader(text))
		dec.UseNumber()
		n, err := jsonNode(dec)
		if err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}
		return n, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, errors.New("empty document")
	}
	return resolve(doc.Content[0]), nil
}

func jsonNode(dec *json.Decoder) (*yaml.Node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		if t == '[' {
			n = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		}
		for dec.More() {
			if t == '{' {
				key, err := dec.Token()
				if err != nil {
					return nil, err
				}
				n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key.(string)})
			}
			child, err := jsonNode(dec)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, child)
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return n, nil
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: t}, nil
	case json.Number:
		tag := "!!int"
		if strings.ContainsAny(string(t), ".eE") {
			tag = "!!float"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: string(t)}, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(t)}, nil
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	}
	return nil, io.ErrUnexpectedEOF
}

func resolve(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

// get returns the value of key in a mapping node, or nil.
func get(n *yaml.Node, key string) *yaml.Node {
	n = resolve(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return resolve(n.Content[i+1])
		}
	}
	return nil
}

// lookup follows nested keys.
func lookup(n *yaml.Node, keys ...string) *yaml.Node {
	for _, k := range keys {
		n = get(n, k)
	}
	return n
}

type entry struct {
	key   string
	value *yaml.Node
}

// entries lists a mapping in document order.
func entries(n *yaml.Node) []entry {
	n = resolve(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	out := make([]entry, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		out = append(out, entry{key: n.Content[i].Value, value: resolve(n.Content[i+1])})
	}
	return out
}

func items(n *yaml.Node) []*yaml.Node {
	n = resolve(n)
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]*yaml.Node, len(n.Content))
	for i, c := range n.Content {
		out[i] = resolve(c)
	}
	return out
}

// str returns a scalar value; ok is false for missing or null values.
func str(n *yaml.Node) (string, bool) {
	n = resolve(n)
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return "", false
	}
	return n.Value, true
}

func strOr(n *yaml.Node, def string) string {
	if s, ok := str(n); ok {
		return s
	}
	return def
}

func intValue(n *yaml.Node) (int, bool) {
	s, ok := str(n)
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(s)
	return i, err == nil
}

// localRef follows a "#/a/b" JSON reference inside the same document.
func localRef(root *yaml.Node, ref string) *yaml.Node {
	ref, ok := strings.CutPrefix(ref, "#/")
	if !ok {
		return nil
	}
	n := root
	for _, part := range strings.Split(ref, "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		n = get(n, part)
	}
	return n
}

// deref resolves chained "$ref" objects (swagger-parser resolveFully).
func deref(root, n *yaml.Node) *yaml.Node {
	for range 32 {
		ref, ok := str(get(n, "$ref"))
		if !ok {
			return n
		}
		target := localRef(root, ref)
		if target == nil {
			return n
		}
		n = target
	}
	return n
}

// toJSON renders a node as compact JSON in document order (used for
// structured RAML examples).
func toJSON(n *yaml.Node) string {
	var b strings.Builder
	writeNodeJSON(&b, resolve(n))
	return b.String()
}

func writeNodeJSON(b *strings.Builder, n *yaml.Node) {
	switch {
	case n == nil:
		b.WriteString("null")
	case n.Kind == yaml.MappingNode:
		b.WriteByte('{')
		for i, e := range entries(n) {
			if i > 0 {
				b.WriteByte(',')
			}
			k, _ := json.Marshal(e.key)
			b.Write(k)
			b.WriteByte(':')
			writeNodeJSON(b, e.value)
		}
		b.WriteByte('}')
	case n.Kind == yaml.SequenceNode:
		b.WriteByte('[')
		for i, c := range items(n) {
			if i > 0 {
				b.WriteByte(',')
			}
			writeNodeJSON(b, c)
		}
		b.WriteByte(']')
	default:
		switch n.Tag {
		case "!!int", "!!float", "!!bool":
			b.WriteString(n.Value)
		case "!!null":
			b.WriteString("null")
		default:
			v, _ := json.Marshal(n.Value)
			b.Write(v)
		}
	}
}
