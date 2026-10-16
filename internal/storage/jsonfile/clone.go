package jsonfile

import (
	"slices"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// Deep copies: repository reads hand out snapshots so callers can never race
// a concurrent Update. Computed fields (StatusCount, RestMethod.URI,
// DefaultResponseName) are nil/zero in stored trees and stay that way.

func cloneRestProject(p domain.RestProject) domain.RestProject {
	c := p
	c.Applications = make([]domain.RestApplication, len(p.Applications))
	for i, a := range p.Applications {
		a.Resources = make([]domain.RestResource, len(p.Applications[i].Resources))
		for j, r := range p.Applications[i].Resources {
			r.Methods = make([]domain.RestMethod, len(p.Applications[i].Resources[j].Methods))
			for k, m := range p.Applications[i].Resources[j].Methods {
				m.SimulateNetworkDelay = clonePtr(m.SimulateNetworkDelay)
				m.NetworkDelay = clonePtr(m.NetworkDelay)
				m.AutomaticForward = clonePtr(m.AutomaticForward)
				m.MockResponses = cloneRestMockResponses(m.MockResponses)
				r.Methods[k] = m
			}
			r.StatusCount = nil
			a.Resources[j] = r
		}
		a.StatusCount = nil
		c.Applications[i] = a
	}
	return c
}

func cloneRestMockResponses(rs []domain.RestMockResponse) []domain.RestMockResponse {
	out := make([]domain.RestMockResponse, len(rs))
	for i := range rs {
		r := rs[i]
		r.UsingExpressions = clonePtr(r.UsingExpressions)
		r.HTTPHeaders = cloneHTTPHeaders(rs[i].HTTPHeaders)
		r.ContentEncodings = cloneEncodings(rs[i].ContentEncodings)
		r.ParameterQueries = slices.Clone(rs[i].ParameterQueries)
		r.XPathExpressions = slices.Clone(rs[i].XPathExpressions)
		r.JSONPathExpressions = slices.Clone(rs[i].JSONPathExpressions)
		r.HeaderQueries = slices.Clone(rs[i].HeaderQueries)
		out[i] = r
	}
	return out
}

func cloneSoapProject(p domain.SoapProject) domain.SoapProject {
	c := p
	c.StatusCount = nil
	c.Resources = slices.Clone(p.Resources)
	c.Ports = make([]domain.SoapPort, len(p.Ports))
	for i, port := range p.Ports {
		port.StatusCount = nil
		port.Operations = make([]domain.SoapOperation, len(p.Ports[i].Operations))
		for j, op := range p.Ports[i].Operations {
			op.SimulateNetworkDelay = clonePtr(op.SimulateNetworkDelay)
			op.NetworkDelay = clonePtr(op.NetworkDelay)
			op.MockOnFailure = clonePtr(op.MockOnFailure)
			op.AutomaticForward = clonePtr(op.AutomaticForward)
			op.MockResponses = cloneSoapMockResponses(op.MockResponses)
			port.Operations[j] = op
		}
		c.Ports[i] = port
	}
	return c
}

func cloneSoapMockResponses(rs []domain.SoapMockResponse) []domain.SoapMockResponse {
	out := make([]domain.SoapMockResponse, len(rs))
	for i := range rs {
		r := rs[i]
		r.UsingExpressions = clonePtr(r.UsingExpressions)
		r.HTTPHeaders = cloneHTTPHeaders(rs[i].HTTPHeaders)
		r.ContentEncodings = cloneEncodings(rs[i].ContentEncodings)
		r.XPathExpressions = slices.Clone(rs[i].XPathExpressions)
		out[i] = r
	}
	return out
}

func cloneHTTPHeaders(hs []domain.HTTPHeader) []domain.HTTPHeader {
	return slices.Clone(hs)
}

func cloneEncodings(es []domain.ContentEncoding) []domain.ContentEncoding {
	return slices.Clone(es)
}

func cloneRestEvent(e domain.RestEvent) domain.RestEvent {
	c := e
	c.Request.HTTPHeaders = cloneHTTPHeaders(e.Request.HTTPHeaders)
	c.Request.HTTPParameters = slices.Clone(e.Request.HTTPParameters)
	if e.Response != nil {
		resp := *e.Response
		resp.HTTPHeaders = cloneHTTPHeaders(e.Response.HTTPHeaders)
		resp.ContentEncodings = cloneEncodings(e.Response.ContentEncodings)
		c.Response = &resp
	}
	return c
}

func cloneSoapEvent(e domain.SoapEvent) domain.SoapEvent {
	c := e
	c.Request.HTTPHeaders = cloneHTTPHeaders(e.Request.HTTPHeaders)
	if e.Response != nil {
		resp := *e.Response
		resp.HTTPHeaders = cloneHTTPHeaders(e.Response.HTTPHeaders)
		resp.ContentEncodings = cloneEncodings(e.Response.ContentEncodings)
		c.Response = &resp
	}
	return c
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
