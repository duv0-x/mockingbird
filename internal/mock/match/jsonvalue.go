package match

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
)

// jsonDoc is a decoded JSON document. Objects are map[string]any so ojg
// filters can evaluate them, and their key order is kept on the side because
// Jayway (json-smart, ordered mode) renders and iterates objects in document order.
type jsonDoc struct {
	root  any
	order map[unsafe.Pointer][]string
}

func decodeJSON(body string) (*jsonDoc, error) {
	dec := json.NewDecoder(strings.NewReader(body))
	dec.UseNumber()
	d := &jsonDoc{order: map[unsafe.Pointer][]string{}}
	root, err := d.value(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("decode json: trailing data")
	}
	d.root = root
	return d, nil
}

func (d *jsonDoc) value(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			m := map[string]any{}
			var keys []string
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, fmt.Errorf("decode json: %w", err)
				}
				k := kt.(string)
				v, err := d.value(dec)
				if err != nil {
					return nil, err
				}
				if _, dup := m[k]; !dup {
					keys = append(keys, k)
				}
				m[k] = v
			}
			if _, err := dec.Token(); err != nil {
				return nil, fmt.Errorf("decode json: %w", err)
			}
			d.order[mapKey(m)] = keys
			return m, nil
		case '[':
			list := []any{}
			for dec.More() {
				v, err := d.value(dec)
				if err != nil {
					return nil, err
				}
				list = append(list, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, fmt.Errorf("decode json: %w", err)
			}
			return list, nil
		}
		return nil, fmt.Errorf("decode json: unexpected %v", t)
	case json.Number:
		return parseNumber(string(t)), nil
	default:
		return t, nil
	}
}

// parseNumber follows json-smart: integral literals stay integers, anything
// with a fraction or exponent is a double.
func parseNumber(s string) any {
	if !strings.ContainsAny(s, ".eE") {
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func mapKey(m map[string]any) unsafe.Pointer { return reflect.ValueOf(m).UnsafePointer() }

func (d *jsonDoc) keys(m map[string]any) []string {
	if keys, ok := d.order[mapKey(m)]; ok && len(keys) == len(m) {
		return keys
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// toString mirrors Object.toString on json-smart values.
func (d *jsonDoc) toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any, []any:
		var b strings.Builder
		d.writeJSON(&b, v)
		return b.String()
	}
	return scalarString(v)
}

func scalarString(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	case float64:
		return FormatJavaDouble(t)
	case string:
		return t
	}
	return fmt.Sprint(v)
}

// writeJSON renders like json-smart's JSONValue.toJSONString with the default
// (NO_COMPRESS) style: compact, every string quoted, "/" escaped.
func (d *jsonDoc) writeJSON(b *strings.Builder, v any) {
	switch t := v.(type) {
	case map[string]any:
		b.WriteByte('{')
		for i, k := range d.keys(t) {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSONString(b, k)
			b.WriteByte(':')
			d.writeJSON(b, t[k])
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			d.writeJSON(b, e)
		}
		b.WriteByte(']')
	case string:
		writeJSONString(b, t)
	default:
		b.WriteString(scalarString(v))
	}
}

func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '/':
			b.WriteString(`\/`)
		default:
			if r <= 0x1F || (r >= 0x7F && r <= 0x9F) || (r >= 0x2000 && r <= 0x20FF) {
				fmt.Fprintf(b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}
