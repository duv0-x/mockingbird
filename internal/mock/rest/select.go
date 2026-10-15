package rest

import (
	"context"
	"fmt"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/mock/match"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// candidates returns the enabled mock responses sorted by name
// (AlphanumComparator), narrowed to those whose Content-Type shares a token
// with the request's Accept header when any does.
func candidates(method domain.RestMethod, headers []domain.HTTPHeader) []domain.RestMockResponse {
	var enabled []domain.RestMockResponse
	for _, r := range method.MockResponses {
		if r.Status == domain.RestMockResponseEnabled {
			enabled = append(enabled, r)
		}
	}
	slices.SortStableFunc(enabled, func(a, b domain.RestMockResponse) int { return match.AlphanumCompare(a.Name, b.Name) })

	accept := headerTokens(headers, "Accept")
	var accepted []domain.RestMockResponse
	for _, r := range enabled {
		contentTypes := headerTokens(r.HTTPHeaders, "Content-Type")
		if slices.ContainsFunc(contentTypes, func(ct string) bool { return slices.Contains(accept, ct) }) {
			accepted = append(accepted, r)
		}
	}
	if len(accepted) == 0 {
		return enabled
	}
	return accepted
}

// headerTokens mirrors getHeaderValues: the first header with that name,
// split on "," and lower-cased, without trimming.
func headerTokens(headers []domain.HTTPHeader, name string) []string {
	v, ok := headerValue(headers, name)
	if !ok {
		return nil
	}
	tokens := strings.Split(v, ",")
	for i, t := range tokens {
		tokens[i] = strings.ToLower(t)
	}
	return tokens
}

// pick applies the response strategy. nil means no response was selected.
func pick(ctx context.Context, state storage.StateRepository, method domain.RestMethod, list []domain.RestMockResponse,
	req *incoming, pathParameters map[string][]string) (*domain.RestMockResponse, error) {
	switch method.ResponseStrategy {
	case domain.RestStrategySequence:
		i, err := state.SequenceIndex(ctx, method.ID)
		if err != nil {
			return nil, fmt.Errorf("read sequence index of %s: %w", method.ID, err)
		}
		if i >= len(list) || i < 0 {
			i = 0
		}
		if err := state.SetSequenceIndex(ctx, method.ID, i+1); err != nil {
			return nil, fmt.Errorf("store sequence index of %s: %w", method.ID, err)
		}
		return &list[i], nil
	case domain.RestStrategyQueryMatch:
		for i := range list {
			if parameterQueriesMatch(list[i].ParameterQueries, pathParameters) {
				return &list[i], nil
			}
		}
		return defaultResponse(method, list), nil
	case domain.RestStrategyXPath:
		// The body is always present (possibly empty) once read, as in Java.
		for i := range list {
			for _, x := range list[i].XPathExpressions {
				if match.XPathMatches(req.body, x.Expression) {
					return &list[i], nil
				}
			}
		}
		return defaultResponse(method, list), nil
	case domain.RestStrategyJSONPath:
		for i := range list {
			for _, x := range list[i].JSONPathExpressions {
				if match.JSONPathMatches(req.body, x.Expression) {
					return &list[i], nil
				}
			}
		}
		return defaultResponse(method, list), nil
	case domain.RestStrategyHeaderQueryMatch:
		for i := range list {
			if headerQueriesMatch(list[i].HeaderQueries, req.headers) {
				return &list[i], nil
			}
		}
		return defaultResponse(method, list), nil
	default: // RANDOM
		return &list[rand.IntN(len(list))], nil
	}
}

func defaultResponse(method domain.RestMethod, list []domain.RestMockResponse) *domain.RestMockResponse {
	if method.DefaultMockResponseID == "" {
		return nil
	}
	for i := range list {
		if list[i].ID == method.DefaultMockResponseID {
			return &list[i]
		}
	}
	return nil
}

// parameterQueriesMatch ports RestParameterQueryValidator.validate: every path
// parameter needs at least one value matched by a query on that parameter.
func parameterQueriesMatch(queries []domain.RestParameterQuery, pathParameters map[string][]string) bool {
	for name, values := range pathParameters {
		matched := false
		for _, v := range values {
			for _, q := range queries {
				if q.Parameter != name {
					continue
				}
				value := v
				if q.URLEncoded {
					value = javaURLEncode(v)
				}
				if valueMatches(value, q.Query, q.MatchAny, q.MatchRegex, q.MatchCase) {
					matched = true
				}
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// headerQueriesMatch ports RestHeaderQueryValidator.validate: some request
// header matches some query on the same (case-insensitive) name.
func headerQueriesMatch(queries []domain.RestHeaderQuery, headers []domain.HTTPHeader) bool {
	for _, h := range headers {
		for _, q := range queries {
			if strings.EqualFold(q.Header, h.Name) && valueMatches(h.Value, q.Query, q.MatchAny, q.MatchRegex, q.MatchCase) {
				return true
			}
		}
	}
	return false
}

func valueMatches(value, query string, matchAny, matchRegex, matchCase bool) bool {
	switch {
	case matchAny:
		return true
	case matchRegex:
		expr := "^(?:" + query + ")$"
		if !matchCase {
			expr = "(?i)" + expr
		}
		re, err := regexp.Compile(expr)
		return err == nil && re.MatchString(value)
	case matchCase:
		return value == query
	}
	return strings.EqualFold(value, query)
}

// javaURLEncode mirrors URLEncoder.encode(s, UTF_8): letters, digits and
// ".-*_" are kept, space becomes "+", everything else is %XX.
func javaURLEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', strings.IndexByte(".-*_", c) >= 0:
			b.WriteByte(c)
		case c == ' ':
			b.WriteByte('+')
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
