package rest

import (
	"maps"
	"slices"
	"testing"
)

// Cases from UrlUtilityTest plus Spring PathPattern behaviour.
func TestPatternMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern string
		path    string
		want    bool
		vars    map[string]string
	}{
		{"/user", "/user", true, map[string]string{}},
		{"/user/id", "/user/id", true, map[string]string{}},
		{"/user/id.json", "/user/id.json", true, map[string]string{}},
		{"/user/{id}", "/user/1", true, map[string]string{"id": "1"}},
		{"/user/{id}.json", "/user/1.json", true, map[string]string{"id": "1"}},
		{"/user/1.{format}", "/user/1.json", true, map[string]string{"format": "json"}},
		{"/user/1.{format}", "/user/1.xml", true, map[string]string{"format": "xml"}},
		{"/user/1.{format}", "/user/2.xml", false, nil},
		{"/user/{id}.{format}", "/user/1.json", true, map[string]string{"id": "1", "format": "json"}},
		{"/user/{id}.{format}", "/user/2.xml", true, map[string]string{"id": "2", "format": "xml"}},
		{"/company/{company}/user/{id}.{format}", "/company/Castle Mock/user/1.json", true,
			map[string]string{"company": "Castle Mock", "id": "1", "format": "json"}},
		{"/user/{id", "/user/1", false, nil},
		{"/user/id?test={hej}", "/user/id", true, map[string]string{}},
		{"/resource/{id}...{type}", "/resource/test...xml", true, map[string]string{"id": "test", "type": "xml"}},
		{"/user/{id}", "/user/a%20b", true, map[string]string{"id": "a b"}},
		{"/user/{id}", "/user/", false, nil},
		{"/user/{id}", "/user", false, nil},
		{"/user", "/user/", false, nil},
		{"/user", "/User", false, nil},
		{"/user/{id:\\d+}", "/user/12", true, map[string]string{"id": "12"}},
		{"/user/{id:\\d+}", "/user/ab", false, nil},
		{"/user/{id:\\d{3}}", "/user/123", true, map[string]string{"id": "123"}},
		{"/user/*", "/user/x", true, map[string]string{}},
		{"/user/*", "/user/x/y", false, nil},
		{"/user/*.json", "/user/x.json", true, map[string]string{}},
		{"/user/**", "/user", true, map[string]string{}},
		{"/user/**", "/user/x/y", true, map[string]string{}},
		{"/user/{*rest}", "/user/x/y", true, map[string]string{"rest": "/x/y"}},
		{"/user/{*rest}", "/user", true, map[string]string{"rest": ""}},
		{"/**/user", "/a/user", false, nil},
		{"/user;v=1", "/user", false, nil},
		{"/user", "/user;jsessionid=1", true, map[string]string{}},
		{"", "", true, map[string]string{}},
		{"/", "/", true, map[string]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.pattern+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			p, err := compilePattern(tt.pattern)
			if err != nil {
				if tt.want {
					t.Fatalf("compile: %v", err)
				}
				return
			}
			vars, ok := p.match(tt.path)
			if ok != tt.want {
				t.Fatalf("match = %v, want %v", ok, tt.want)
			}
			if ok && !maps.Equal(vars, tt.vars) {
				t.Errorf("vars = %v, want %v", vars, tt.vars)
			}
		})
	}
}

func TestPatternSpecificity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern            string
		variables, literal int
	}{
		{"/user", 0, 1},
		{"/user/{id}.json", 1, 1},
		{"/user/{id}/{x}", 2, 1},
		{"/user/**", 1, 1},
	}
	for _, tt := range tests {
		p, err := compilePattern(tt.pattern)
		if err != nil {
			t.Fatal(err)
		}
		if v, l := p.specificity(); v != tt.variables || l != tt.literal {
			t.Errorf("%s: specificity = %d,%d want %d,%d", tt.pattern, v, l, tt.variables, tt.literal)
		}
	}
}

func TestQueryParameters(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		uri    string
		params []parameter
		want   map[string][]string
	}{
		{"single", "/rest/api/user/?userId={userId}", []parameter{{Name: "userId", Value: "1"}}, map[string][]string{"userId": {"1"}}},
		{"different variable name", "/rest/api/user/?userId={id}", []parameter{{Name: "userId", Value: "1"}}, map[string][]string{"userId": {"1"}}},
		{"multiple", "/rest/api/user/?userId={userId}&username={username}",
			[]parameter{{Name: "userId", Value: "1"}, {Name: "username", Value: "Karl"}}, map[string][]string{"userId": {"1"}, "username": {"Karl"}}},
		{"multiple values", "/rest/api/user/?userId={userId}&userId=1",
			[]parameter{{Name: "userId", Value: "1"}, {Name: "userId", Value: "2"}}, map[string][]string{"userId": {"1", "2"}}},
		{"no template", "/rest/api/user", []parameter{{Name: "userId", Value: "1"}}, map[string][]string{}},
		{"missing parameter", "/a?x={x}", nil, map[string][]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := queryParameters(tt.uri, tt.params)
			if !maps.EqualFunc(got, tt.want, slices.Equal[[]string]) {
				t.Errorf("queryParameters = %v, want %v", got, tt.want)
			}
		})
	}
}
