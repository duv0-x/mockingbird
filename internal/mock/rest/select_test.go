package rest

import (
	"testing"

	"github.com/duv0-x/mockingbird/internal/domain"
)

type pq = domain.RestParameterQuery

// Cases from RestParameterQueryValidatorTest.
func TestParameterQueriesMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		queries []pq
		params  map[string][]string
		want    bool
	}{
		{"match", []pq{{Parameter: "userId", Query: "123"}, {Parameter: "country", Query: "SE"}},
			map[string][]string{"userId": {"123"}, "country": {"SE"}}, true},
		{"url encoded enabled", []pq{{Parameter: "input", Query: "%3Cxml%3Etest%3C%2Fxml%3E", URLEncoded: true}},
			map[string][]string{"input": {"<xml>test</xml>"}}, true},
		{"url encoded disabled", []pq{{Parameter: "input", Query: "%3Cxml%3Etest%3C%2Fxml%3E"}},
			map[string][]string{"input": {"<xml>test</xml>"}}, false},
		{"url encoded like URLEncoder", []pq{{Parameter: "input", Query: "a+b*~", URLEncoded: true, MatchCase: true}},
			map[string][]string{"input": {"a b*~"}}, false},
		{"url encoded tilde", []pq{{Parameter: "input", Query: "a+b*%7E", URLEncoded: true, MatchCase: true}},
			map[string][]string{"input": {"a b*~"}}, true},
		{"multiple values", []pq{{Parameter: "userId", Query: "123"}}, map[string][]string{"userId": {"123", "456"}}, true},
		{"no match", []pq{{Parameter: "userId2", Query: "345"}, {Parameter: "country", Query: "SE"}},
			map[string][]string{"userId": {"123"}, "country": {"SE"}}, false},
		{"missing", []pq{{Parameter: "userId", Query: "123"}}, map[string][]string{"userId": {"123"}, "country": {"SE"}}, false},
		{"multiple", []pq{{Parameter: "userId", Query: "123"}, {Parameter: "country", Query: "SE"}, {Parameter: "country", Query: "NO"}},
			map[string][]string{"userId": {"123"}, "country": {"SE"}}, true},
		{"not match case", []pq{{Parameter: "username", Query: "karl"}}, map[string][]string{"username": {"kArL"}}, true},
		{"match case", []pq{{Parameter: "username", Query: "karl", MatchCase: true}}, map[string][]string{"username": {"kArL"}}, false},
		{"match any", []pq{{Parameter: "username", MatchAny: true}}, map[string][]string{"username": {"username"}}, true},
		{"regex case sensitive", []pq{{Parameter: "username", Query: "a*b", MatchRegex: true, MatchCase: true}}, map[string][]string{"username": {"aaaaab"}}, true},
		{"regex case sensitive no match", []pq{{Parameter: "username", Query: "a*b", MatchRegex: true, MatchCase: true}}, map[string][]string{"username": {"AAAAB"}}, false},
		{"regex ignore case", []pq{{Parameter: "username", Query: "a*b", MatchRegex: true}}, map[string][]string{"username": {"aaaaab"}}, true},
		{"regex ignore case upper", []pq{{Parameter: "username", Query: "a*b", MatchRegex: true}}, map[string][]string{"username": {"AAAAB"}}, true},
		{"regex is a full match", []pq{{Parameter: "username", Query: "a", MatchRegex: true}}, map[string][]string{"username": {"ab"}}, false},
		{"no parameters", []pq{{Parameter: "x", Query: "1"}}, map[string][]string{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parameterQueriesMatch(tt.queries, tt.params); got != tt.want {
				t.Errorf("parameterQueriesMatch = %v, want %v", got, tt.want)
			}
		})
	}
}

// Cases from RestHeaderQueryValidatorTest.
func TestHeaderQueriesMatch(t *testing.T) {
	t.Parallel()
	header := func(v string) []domain.HTTPHeader { return []domain.HTTPHeader{{Name: "Content-Encoding", Value: v}} }
	tests := []struct {
		name  string
		query domain.RestHeaderQuery
		hdrs  []domain.HTTPHeader
		want  bool
	}{
		{"query match", domain.RestHeaderQuery{Header: "Content-Encoding", Query: "application/json", MatchCase: true}, header("application/json"), true},
		{"no match case sensitive", domain.RestHeaderQuery{Header: "Content-Encoding", Query: "application/JSON", MatchCase: true}, header("application/json"), false},
		{"match case insensitive", domain.RestHeaderQuery{Header: "Content-Encoding", Query: "application/JSON"}, header("application/json"), true},
		{"match any", domain.RestHeaderQuery{Header: "Content-Encoding", Query: "application/json", MatchAny: true}, header("application/xml"), true},
		{"regex no match", domain.RestHeaderQuery{Header: "Content-Encoding", Query: "application/json", MatchRegex: true}, header("application/xml"), false},
		{"regex match", domain.RestHeaderQuery{Header: "Content-Encoding", Query: "application/(.*)", MatchRegex: true}, header("APPLICATION/json"), true},
		{"regex case sensitive", domain.RestHeaderQuery{Header: "Content-Encoding", Query: "application/(.*)", MatchRegex: true, MatchCase: true}, header("APPLICATION/json"), false},
		{"header name case insensitive", domain.RestHeaderQuery{Header: "content-encoding", Query: "gzip"}, header("gzip"), true},
		{"other header", domain.RestHeaderQuery{Header: "X-Other", Query: "gzip"}, header("gzip"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := headerQueriesMatch([]domain.RestHeaderQuery{tt.query}, tt.hdrs); got != tt.want {
				t.Errorf("headerQueriesMatch = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNegotiateContentType(t *testing.T) {
	t.Parallel()
	tests := []struct{ accept, want string }{
		{"", "text/plain;charset=UTF-8"},
		{"*/*", "text/plain;charset=UTF-8"},
		{"application/json", "application/json"},
		{"application/json, text/plain, */*", "application/json"},
		{"application/xml", "application/xml;charset=UTF-8"},
		{"text/html;q=0.5, application/hal+json", "application/hal+json"},
		{"application/*", "text/plain;charset=UTF-8"},
	}
	for _, tt := range tests {
		if got := negotiateContentType(tt.accept); got != tt.want {
			t.Errorf("negotiateContentType(%q) = %q, want %q", tt.accept, got, tt.want)
		}
	}
}

func TestReadBody(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"", ""}, {"a", "a"}, {"a\n", "a"}, {"a\r\nb\r\n", "a\nb"}, {"a\rb", "a\nb"}, {"a\n\n", "a\n"},
	}
	for _, tt := range tests {
		got, err := readBody(stringReader(tt.in))
		if err != nil || got != tt.want {
			t.Errorf("readBody(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}
