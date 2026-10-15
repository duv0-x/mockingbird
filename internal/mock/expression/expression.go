// Package expression implements CastleMock's response templating
// "${NAME(arg=value, ...)}" (SPEC.md §4.4).
//
// Owner: T2. Consumers: internal/mock/rest, internal/mock/soap (T3), internal/api (T4, expression validation).
package expression

import (
	"log/slog"
	"strings"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// Input is the external data an expression may read (ExternalInputBuilder).
// Zero-value fields mean "not provided".
type Input struct {
	Body            *string
	URL             *string
	PathParameters  map[string][]string
	QueryParameters []domain.HTTPParameter
}

// Names of the external arguments added to every expression (ExternalInputBuilder).
const (
	argBody           = "BODY"
	argURL            = "URL_ARGUMENT"
	argPathParameters = "PATH_PARAMETERS"
	argQueryStrings   = "QUERY_STRINGS"
)

// Render replaces every expression in text. Unknown expressions are left untouched.
func Render(text string, in Input) string {
	return render(text, in, functions)
}

// render ports TextParser.parse: an expression runs from "${" to the first
// "}" after it, and scanning resumes one character after the start of the
// previous "${" — so text produced by an expression is scanned too.
//
// Deviation (SPEC.md §4.4): an expression that cannot be parsed or fails while
// evaluating (e.g. RANDOM_ENUM without values) is left as is; CastleMock's
// ANTLR parser recovers from some syntax errors and failing expressions make
// the whole mock request fail with HTTP 500.
func render(text string, in Input, registry map[string]function) string {
	external := in.arguments()
	out := text
	start := -1
	for {
		if start+1 >= len(out) {
			return out
		}
		next := strings.Index(out[start+1:], "${")
		if next < 0 {
			return out
		}
		start += 1 + next
		end := strings.Index(out[start+1:], "}")
		if end < 0 {
			continue
		}
		end += start + 1
		source := out[start : end+1]
		call, err := parse(source)
		if err != nil {
			slog.Debug("unable to parse expression", "expression", source, "err", err)
			continue
		}
		fn, ok := registry[call.name]
		if !ok {
			slog.Debug("unknown expression", "expression", call.name)
			continue
		}
		for name, arg := range external {
			call.args[strings.ToLower(name)] = arg
		}
		result, err := fn(call)
		if err != nil {
			slog.Warn("unable to evaluate expression", "expression", source, "err", err)
			continue
		}
		out = out[:start] + result + out[end+1:]
	}
}

func (in Input) arguments() map[string]argument {
	args := map[string]argument{}
	if in.Body != nil {
		args[argBody] = argString(*in.Body)
	}
	if in.URL != nil {
		args[argURL] = argString(*in.URL)
	}
	if in.PathParameters != nil {
		m := argMap{}
		for name, values := range in.PathParameters {
			arr := make(argArray, 0, len(values))
			for _, v := range values {
				arr = append(arr, argString(v))
			}
			m[name] = arr
		}
		args[argPathParameters] = m
	}
	if in.QueryParameters != nil {
		m := argMap{}
		for _, p := range in.QueryParameters {
			m[p.Name] = argString(p.Value)
		}
		args[argQueryStrings] = m
	}
	return args
}
