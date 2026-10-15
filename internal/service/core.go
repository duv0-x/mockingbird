package service

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"runtime/debug"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/mock/expression"
)

// Projects returns REST and SOAP projects (GET /core/project).
func (s *Service) Projects(ctx context.Context) ([]domain.OverviewProject, error) {
	restProjects, err := s.rest().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list rest projects: %w", err)
	}
	soapProjects, err := s.soap().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list soap projects: %w", err)
	}
	out := make([]domain.OverviewProject, 0, len(restProjects)+len(soapProjects))
	for _, p := range restProjects {
		out = append(out, domain.OverviewProject{Project: p.Project, Type: domain.ProjectTypeREST})
	}
	for _, p := range soapProjects {
		out = append(out, domain.OverviewProject{Project: p.Project, Type: domain.ProjectTypeSOAP})
	}
	return out, nil
}

// Events lists all events, REST then SOAP (GET /core/event).
func (s *Service) Events(ctx context.Context) ([]domain.OverviewEvent, error) {
	restEvents, err := s.deps.Store.Events().ListRest(ctx)
	if err != nil {
		return nil, fmt.Errorf("list rest events: %w", err)
	}
	soapEvents, err := s.deps.Store.Events().ListSoap(ctx)
	if err != nil {
		return nil, fmt.Errorf("list soap events: %w", err)
	}
	out := make([]domain.OverviewEvent, 0, len(restEvents)+len(soapEvents))
	for _, e := range restEvents {
		out = append(out, domain.OverviewEvent{Event: e.Event, Type: domain.ProjectTypeREST})
	}
	for _, e := range soapEvents {
		out = append(out, domain.OverviewEvent{Event: e.Event, Type: domain.ProjectTypeSOAP})
	}
	return out, nil
}

// ClearEvents deletes every REST and SOAP event.
func (s *Service) ClearEvents(ctx context.Context) error {
	if err := s.deps.Store.Events().ClearRest(ctx); err != nil {
		return fmt.Errorf("clear rest events: %w", err)
	}
	if err := s.deps.Store.Events().ClearSoap(ctx); err != nil {
		return fmt.Errorf("clear soap events: %w", err)
	}
	return nil
}

func (s *Service) RestEvent(ctx context.Context, id string) (domain.RestEvent, error) {
	e, err := s.deps.Store.Events().GetRest(ctx, id)
	if err != nil {
		return domain.RestEvent{}, fmt.Errorf("load rest event %s: %w", id, err)
	}
	e.Request.HTTPHeaders = emptyIfNil(e.Request.HTTPHeaders)
	e.Request.HTTPParameters = emptyIfNil(e.Request.HTTPParameters)
	if e.Response != nil {
		e.Response.HTTPHeaders = emptyIfNil(e.Response.HTTPHeaders)
		e.Response.ContentEncodings = emptyIfNil(e.Response.ContentEncodings)
	}
	return e, nil
}

func (s *Service) SoapEvent(ctx context.Context, id string) (domain.SoapEvent, error) {
	e, err := s.deps.Store.Events().GetSoap(ctx, id)
	if err != nil {
		return domain.SoapEvent{}, fmt.Errorf("load soap event %s: %w", id, err)
	}
	e.Request.HTTPHeaders = emptyIfNil(e.Request.HTTPHeaders)
	if e.Response != nil {
		e.Response.HTTPHeaders = emptyIfNil(e.Response.HTTPHeaders)
		e.Response.ContentEncodings = emptyIfNil(e.Response.ContentEncodings)
	}
	return e, nil
}

// ValidateExpression renders responseBody with requestBody as BODY input.
func (s *Service) ValidateExpression(ctx context.Context, requestBody, responseBody string) (string, error) {
	return expression.Render(responseBody, expression.Input{Body: &requestBody}), nil
}

// SystemInformation maps CastleMock's Java/Tomcat fields to Go runtime
// equivalents (SPEC.md §5.3). Memory values are in MB (bytes / 1 000 000).
func (s *Service) SystemInformation(ctx context.Context) (domain.SystemInformation, error) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	maxBytes := ms.Sys
	if limit := debug.SetMemoryLimit(-1); limit > 0 && limit != math.MaxInt64 {
		maxBytes = uint64(limit)
	}
	const mb = 1_000_000
	return domain.SystemInformation{
		OperatingSystemName:         runtime.GOOS,
		JavaVersion:                 runtime.Version(),
		JavaVendor:                  "Go",
		TomcatBuilt:                 buildTime(),
		TomcatInfo:                  "mockingbird",
		TomcatVersion:               s.deps.Version,
		TotalMemory:                 int64(ms.Sys / mb),
		MaxMemory:                   int64(maxBytes / mb),
		FreeMemory:                  int64((ms.Sys - ms.HeapAlloc) / mb),
		AvailableProcessors:         runtime.NumCPU(),
		CastleMockHomeDirectory:     s.deps.Config.DataDir,
		ShowCastleMockHomeDirectory: s.deps.Config.ShowDataDir,
	}, nil
}

func buildTime() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.time" {
			return setting.Value
		}
	}
	return ""
}

func (s *Service) Version() string {
	return s.deps.Version
}

// ContextPath is the configured context path ("" at root).
func (s *Service) ContextPath() string {
	return s.deps.Config.ContextPath
}
