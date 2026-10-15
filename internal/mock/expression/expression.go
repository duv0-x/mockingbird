// Package expression implements CastleMock's response templating
// "${NAME(arg=value, ...)}" (SPEC.md §4.4).
//
// Owner: T2. Consumers: internal/mock/rest, internal/mock/soap (T3), internal/api (T4, expression validation).
package expression

import "github.com/duv0-x/mockingbird/internal/domain"

// Input is the external data an expression may read (ExternalInputBuilder).
// Zero-value fields mean "not provided".
type Input struct {
	Body            *string
	URL             *string
	PathParameters  map[string][]string
	QueryParameters []domain.HTTPParameter
}

// Render replaces every expression in text. Unknown expressions are left untouched.
func Render(text string, in Input) string {
	return text
}
