package match

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/ohler55/ojg/jp"
)

// jsonPath is a compiled Jayway-style path. ojg parses the expression; the
// walk is done here because Jayway differs from ojg where it matters for
// CastleMock users: a filter applied to an object tests the object itself,
// objects are iterated in document order, and paths may end in a function.
type jsonPath struct {
	frags    jp.Expr
	function string
	definite bool
}

var trailingFunction = regexp.MustCompile(`\.([A-Za-z]+)\(\)$`)

func compileJSONPath(expr string) (jsonPath, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return jsonPath{}, errors.New("empty json path")
	}
	// PathCompiler.compile: a path without root is relative to "$".
	if !strings.HasPrefix(expr, "$") && !strings.HasPrefix(expr, "@") {
		expr = "$." + expr
	}
	var p jsonPath
	if m := trailingFunction.FindStringSubmatch(expr); m != nil {
		p.function = m[1]
		expr = strings.TrimSuffix(expr, m[0])
	}
	frags, err := jp.ParseString(dotBracket(expr))
	if err != nil {
		return jsonPath{}, fmt.Errorf("parse json path %q: %w", expr, err)
	}
	p.frags = frags
	p.definite = true
	for _, f := range frags {
		switch t := f.(type) {
		case jp.Descent, jp.Wildcard, *jp.Filter, jp.Slice:
			p.definite = false
		case jp.Union:
			if len(t) > 1 {
				p.definite = false
			}
		}
	}
	return p, nil
}

// dotBracket rewrites Jayway's tolerated "a.[0]" into "a[0]" (ojg rejects it),
// leaving ".." and quoted text alone.
func dotBracket(expr string) string {
	var b strings.Builder
	var quote byte
	for i := 0; i < len(expr); i++ {
		c := expr[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '.' && i+1 < len(expr) && expr[i+1] == '[' && (i == 0 || expr[i-1] != '.'):
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// eval returns the selected values. For a definite path, found reports
// whether the single location exists.
func (p jsonPath) eval(d *jsonDoc) (values []any, found bool) {
	current := []any{d.root}
	for _, f := range p.frags {
		var next []any
		switch t := f.(type) {
		case jp.Root, jp.At, jp.Bracket:
			continue
		case jp.Child:
			for _, v := range current {
				if m, ok := v.(map[string]any); ok {
					if e, ok := m[string(t)]; ok {
						next = append(next, e)
					}
				}
			}
		case jp.Nth:
			for _, v := range current {
				if e, ok := nth(v, int(t)); ok {
					next = append(next, e)
				}
			}
		case jp.Union:
			for _, v := range current {
				for _, key := range t {
					switch k := key.(type) {
					case string:
						if m, ok := v.(map[string]any); ok {
							if e, ok := m[k]; ok {
								next = append(next, e)
							}
						}
					case int64:
						if e, ok := nth(v, int(k)); ok {
							next = append(next, e)
						}
					}
				}
			}
		case jp.Wildcard:
			for _, v := range current {
				next = append(next, d.children(v)...)
			}
		case jp.Slice:
			for _, v := range current {
				if list, ok := v.([]any); ok {
					next = append(next, slice(list, t)...)
				}
			}
		case *jp.Filter:
			for _, v := range current {
				switch c := v.(type) {
				case []any:
					for _, e := range c {
						if filterMatch(t, e) {
							next = append(next, e)
						}
					}
				case map[string]any:
					if filterMatch(t, c) {
						next = append(next, c)
					}
				}
			}
		case jp.Descent:
			for _, v := range current {
				next = d.containers(v, next)
			}
		default:
			return nil, false
		}
		current = next
	}
	return current, len(current) > 0
}

func filterMatch(f *jp.Filter, v any) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return f.Match(v)
}

func nth(v any, i int) (any, bool) {
	list, ok := v.([]any)
	if !ok {
		return nil, false
	}
	if i < 0 {
		i += len(list)
	}
	if i < 0 || i >= len(list) {
		return nil, false
	}
	return list[i], true
}

// slice follows Jayway's ArraySliceOperation: [from:to] with "to" exclusive
// and negative indexes counted from the end.
func slice(list []any, s jp.Slice) []any {
	n := len(list)
	bound := func(v, def int) int {
		if v == jp.SliceNotSet {
			return def
		}
		if v < 0 {
			v += n
		}
		return min(max(v, 0), n)
	}
	from, to := bound(s[0], 0), bound(s[1], n)
	step := 1
	if s[2] != jp.SliceNotSet && s[2] > 0 {
		step = s[2]
	}
	var out []any
	for i := from; i < to; i += step {
		out = append(out, list[i])
	}
	return out
}

func (d *jsonDoc) children(v any) []any {
	switch c := v.(type) {
	case map[string]any:
		out := make([]any, 0, len(c))
		for _, k := range d.keys(c) {
			out = append(out, c[k])
		}
		return out
	case []any:
		return c
	}
	return nil
}

// containers appends v and every object/array below it, in document order.
func (d *jsonDoc) containers(v any, out []any) []any {
	switch v.(type) {
	case map[string]any, []any:
		out = append(out, v)
		for _, c := range d.children(v) {
			out = d.containers(c, out)
		}
	}
	return out
}

// result is what JsonPath.read returns: the single value of a definite path,
// the list of an indefinite one, or a function's value.
func (p jsonPath) result(d *jsonDoc) (any, bool) {
	values, found := p.eval(d)
	var v any
	if p.definite {
		if !found {
			return nil, false
		}
		v = values[0]
	} else {
		if values == nil {
			values = []any{}
		}
		v = values
	}
	if p.function != "" {
		return applyFunction(p.function, v)
	}
	return v, true
}

// applyFunction supports the Jayway path functions that make sense on a
// selection; the numeric ones return doubles like Jayway.
func applyFunction(name string, v any) (any, bool) {
	list, isList := v.([]any)
	switch name {
	case "length", "size":
		switch c := v.(type) {
		case []any:
			return int64(len(c)), true
		case map[string]any:
			return int64(len(c)), true
		}
		return nil, false
	case "first", "last":
		if !isList || len(list) == 0 {
			return nil, false
		}
		if name == "first" {
			return list[0], true
		}
		return list[len(list)-1], true
	case "min", "max", "sum", "avg", "stddev":
		if !isList {
			return nil, false
		}
		var nums []float64
		for _, e := range list {
			switch n := e.(type) {
			case int64:
				nums = append(nums, float64(n))
			case float64:
				nums = append(nums, n)
			}
		}
		if len(nums) == 0 {
			return nil, false
		}
		return aggregate(name, nums), true
	}
	return nil, false
}

func aggregate(name string, nums []float64) float64 {
	sum, lo, hi := 0.0, math.Inf(1), math.Inf(-1)
	for _, n := range nums {
		sum += n
		lo, hi = math.Min(lo, n), math.Max(hi, n)
	}
	avg := sum / float64(len(nums))
	switch name {
	case "min":
		return lo
	case "max":
		return hi
	case "sum":
		return sum
	case "avg":
		return avg
	}
	variance := 0.0
	for _, n := range nums {
		variance += (n - avg) * (n - avg)
	}
	return math.Sqrt(variance / float64(len(nums)))
}

func jsonPathMatches(body, expr string) bool {
	d, err := decodeJSON(body)
	if err != nil {
		return false
	}
	p, err := compileJSONPath(expr)
	if err != nil {
		return false
	}
	v, ok := p.result(d)
	if !ok {
		return false
	}
	if list, isList := v.([]any); isList {
		return len(list) > 0
	}
	return v != nil
}

func jsonPathValue(body, expr string) (string, bool) {
	d, err := decodeJSON(body)
	if err != nil {
		return "", false
	}
	p, err := compileJSONPath(expr)
	if err != nil {
		return "", false
	}
	v, ok := p.result(d)
	if !ok || v == nil {
		return "", false
	}
	if list, isList := v.([]any); isList {
		switch len(list) {
		case 0:
			return "", false
		case 1:
			if list[0] == nil {
				return "", false
			}
			return d.toString(list[0]), true
		}
	}
	return d.toString(v), true
}
