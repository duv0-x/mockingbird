package rest

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// identified is the output of IdentifyRestMethodService.
type identified struct {
	projectID      string // project that owns the application (store lookup)
	resource       domain.RestResource
	method         domain.RestMethod
	pathParameters map[string][]string
}

// identify ports IdentifyRestMethodService. The application is resolved by
// its own id; the project id of the URL is only recorded in events.
func identify(ctx context.Context, repo storage.RestRepository, req *incoming) (*identified, error) {
	projectID, err := repo.ProjectOf(ctx, req.applicationID)
	if err != nil {
		return nil, fmt.Errorf("resolve application %s: %w", req.applicationID, err)
	}
	project, err := repo.Get(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("load project %s: %w", projectID, err)
	}
	app := project.Application(req.applicationID)
	if app == nil {
		return nil, fmt.Errorf("application %s not found in project %s", req.applicationID, projectID)
	}

	type candidate struct {
		resource  *domain.RestResource
		vars      map[string]string
		variables int
		literals  int
	}
	var candidates []candidate
	for i := range app.Resources {
		res := &app.Resources[i]
		pattern, err := compilePattern(res.URI)
		if err != nil {
			continue
		}
		vars, ok := pattern.match(req.uri)
		if !ok {
			continue
		}
		v, l := pattern.specificity()
		candidates = append(candidates, candidate{resource: res, vars: vars, variables: v, literals: l})
	}
	// D-4: CastleMock picks an arbitrary candidate (HashMap order); the most
	// specific pattern wins here, then storage order.
	slices.SortStableFunc(candidates, func(a, b candidate) int {
		if a.variables != b.variables {
			return a.variables - b.variables
		}
		return b.literals - a.literals
	})
	for _, c := range candidates {
		for _, m := range c.resource.Methods {
			if m.HTTPMethod != req.method {
				continue
			}
			params := map[string][]string{}
			for name, v := range c.vars {
				params[name] = []string{v}
			}
			maps.Copy(params, queryParameters(c.resource.URI, req.params))
			return &identified{projectID: projectID, resource: *c.resource, method: m, pathParameters: params}, nil
		}
	}
	return nil, fmt.Errorf("unable to identify REST method: %s (%s)", req.uri, req.method)
}
