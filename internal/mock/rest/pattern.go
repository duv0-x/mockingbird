package rest

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// pathPattern is a resource URI compiled with Spring 6 PathPattern semantics
// (UrlUtility.isPatternMatch): segments are matched case-sensitively against
// the percent-decoded request segments, "{name}" captures one non-empty
// segment, "{name:regex}" constrains it, "*" matches one non-empty segment,
// "**" and "{*name}" the rest of the path, and a segment mixing literals and
// variables ("{id}.{format}") becomes a regular expression. The query
// template after "?" is not part of the pattern. There is no optional
// trailing slash.
type pathPattern struct {
	segments    []segment
	restCapture string // name of a trailing {*name}
	restAny     bool   // trailing **
}

type segmentKind int

const (
	literalSegment segmentKind = iota
	captureSegment
	wildcardSegment
	regexSegment
)

type segment struct {
	kind    segmentKind
	literal string
	names   []string
	re      *regexp.Regexp
}

var errPattern = errors.New("invalid path pattern")

func compilePattern(uri string) (*pathPattern, error) {
	if i := strings.IndexByte(uri, '?'); i >= 0 {
		uri = uri[:i]
	}
	parts := strings.Split(uri, "/")
	p := &pathPattern{}
	for i, part := range parts {
		last := i == len(parts)-1
		switch {
		case part == "**":
			if !last {
				return nil, fmt.Errorf("%w: ** must be the last element of %q", errPattern, uri)
			}
			p.restAny = true
		case strings.HasPrefix(part, "{*") && strings.HasSuffix(part, "}"):
			if !last {
				return nil, fmt.Errorf("%w: {*...} must be the last element of %q", errPattern, uri)
			}
			p.restCapture = part[2 : len(part)-1]
		case part == "*":
			p.segments = append(p.segments, segment{kind: wildcardSegment})
		case !strings.ContainsAny(part, "{}*?"):
			p.segments = append(p.segments, segment{kind: literalSegment, literal: part})
		default:
			s, err := compileSegment(part)
			if err != nil {
				return nil, fmt.Errorf("compile %q: %w", uri, err)
			}
			p.segments = append(p.segments, s)
		}
	}
	return p, nil
}

// compileSegment handles "{name}", "{name:regex}" and segments mixing
// literals, "*", "?" and variables (Spring's RegexPathElement, where a plain
// variable is "(.*)").
func compileSegment(part string) (segment, error) {
	var re strings.Builder
	var names []string
	re.WriteString("^")
	for i := 0; i < len(part); {
		switch c := part[i]; c {
		case '{':
			end, err := closingBrace(part, i)
			if err != nil {
				return segment{}, err
			}
			name, constraint, hasConstraint := strings.Cut(part[i+1:end], ":")
			if name == "" {
				return segment{}, fmt.Errorf("%w: empty variable name", errPattern)
			}
			names = append(names, name)
			if hasConstraint {
				re.WriteString("(" + constraint + ")")
			} else {
				re.WriteString("(.*)")
			}
			i = end + 1
		case '}':
			return segment{}, fmt.Errorf("%w: unexpected }", errPattern)
		case '*':
			re.WriteString(".*")
			i++
		case '?':
			re.WriteString(".")
			i++
		default:
			j := i
			for j < len(part) && !strings.ContainsRune("{}*?", rune(part[j])) {
				j++
			}
			re.WriteString(regexp.QuoteMeta(part[i:j]))
			i = j
		}
	}
	re.WriteString("$")
	compiled, err := regexp.Compile(re.String())
	if err != nil {
		return segment{}, fmt.Errorf("%w: %w", errPattern, err)
	}
	// A whole-segment variable must capture a non-empty value
	// (CaptureVariablePathElement); a variable inside a segment may be empty.
	kind := regexSegment
	if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && len(names) == 1 {
		if end, _ := closingBrace(part, 0); end == len(part)-1 {
			kind = captureSegment
		}
	}
	return segment{kind: kind, names: names, re: compiled}, nil
}

// closingBrace finds the "}" closing the "{" at open, allowing nested braces
// in regex constraints such as {id:\d{3}}.
func closingBrace(s string, open int) (int, error) {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, fmt.Errorf("%w: unclosed {", errPattern)
}

// match matches a raw (percent-encoded) path and returns the decoded variables.
func (p *pathPattern) match(rawPath string) (map[string]string, bool) {
	parts := strings.Split(rawPath, "/")
	values := make([]string, len(parts))
	for i, part := range parts {
		// PathContainer drops ";matrix=params" before matching.
		if j := strings.IndexByte(part, ';'); j >= 0 {
			part = part[:j]
		}
		v, err := url.PathUnescape(part)
		if err != nil {
			return nil, false
		}
		values[i] = v
	}
	rest := p.restAny || p.restCapture != ""
	if len(values) < len(p.segments) || (!rest && len(values) != len(p.segments)) {
		return nil, false
	}
	vars := map[string]string{}
	for i, s := range p.segments {
		v := values[i]
		switch s.kind {
		case literalSegment:
			if v != s.literal {
				return nil, false
			}
		case wildcardSegment:
			if v == "" {
				return nil, false
			}
		case captureSegment:
			if v == "" || !s.re.MatchString(v) {
				return nil, false
			}
			vars[s.names[0]] = v
		case regexSegment:
			m := s.re.FindStringSubmatch(v)
			if m == nil {
				return nil, false
			}
			for k, name := range s.names {
				vars[name] = m[k+1]
			}
		}
	}
	if p.restCapture != "" {
		captured := ""
		if remaining := values[len(p.segments):]; len(remaining) > 0 {
			captured = "/" + strings.Join(remaining, "/")
		}
		vars[p.restCapture] = captured
	}
	return vars, true
}

// specificity orders candidate resources (D-4): fewer variables and
// wildcards first, then more literal segments.
func (p *pathPattern) specificity() (variables, literals int) {
	for _, s := range p.segments {
		switch s.kind {
		case literalSegment:
			if s.literal != "" {
				literals++
			}
		case wildcardSegment:
			variables++
		default:
			variables += max(len(s.names), 1)
		}
	}
	if p.restAny || p.restCapture != "" {
		variables++
	}
	return variables, literals
}

// queryParameters mirrors UrlUtility.getQueryStringParameters: for every
// "name={variable}" pair of the resource URI query template, the request
// values of query parameter "name", keyed by "name".
func queryParameters(uri string, params []parameter) map[string][]string {
	out := map[string][]string{}
	i := strings.IndexByte(uri, '?')
	if i <= 0 {
		return out
	}
	query := strings.Split(uri, "?")[1]
	for _, pair := range strings.Split(query, "&") {
		parts := strings.Split(pair, "=")
		if len(parts) != 2 {
			continue
		}
		name, value := parts[0], parts[1]
		if !strings.HasPrefix(value, "{") || !strings.HasSuffix(value, "}") {
			continue
		}
		for _, p := range params {
			if p.Name == name && !contains(out[name], p.Value) {
				out[name] = append(out[name], p.Value)
			}
		}
	}
	return out
}

func contains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}
