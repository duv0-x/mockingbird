package jsonfile

import (
	"fmt"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// --- REST project: file -> domain ---

func restProjectToDomain(f restProjectFile) domain.RestProject {
	p := domain.RestProject{
		Project: domain.Project{
			ID:          f.ID,
			Name:        f.Name,
			Description: f.Description,
			Created:     domain.Time{Time: f.Created.Time},
			Updated:     domain.Time{Time: f.Updated.Time},
		},
		Applications: make([]domain.RestApplication, 0, len(f.Applications)),
	}
	for _, a := range f.Applications {
		app := domain.RestApplication{
			ID:        a.ID,
			Name:      a.Name,
			ProjectID: f.ID,
			Resources: make([]domain.RestResource, 0, len(a.Resources)),
		}
		for _, r := range a.Resources {
			res := domain.RestResource{
				ID:            r.ID,
				Name:          r.Name,
				URI:           r.URI,
				ApplicationID: a.ID,
				Methods:       make([]domain.RestMethod, 0, len(r.Methods)),
			}
			for _, m := range r.Methods {
				method := domain.RestMethod{
					ID:                    m.ID,
					Name:                  m.Name,
					ResourceID:            r.ID,
					DefaultBody:           m.DefaultBody,
					HTTPMethod:            defaultHTTPMethod(m.HTTPMethod),
					ForwardedEndpoint:     m.ForwardedEndpoint,
					Status:                defaultRestMethodStatus(m.Status),
					ResponseStrategy:      defaultRestStrategy(m.ResponseStrategy),
					SimulateNetworkDelay:  m.SimulateNetworkDelay,
					NetworkDelay:          m.NetworkDelay,
					DefaultMockResponseID: m.DefaultMockResponseID,
					AutomaticForward:      m.AutomaticForward,
					MockResponses:         make([]domain.RestMockResponse, 0, len(m.MockResponses)),
				}
				for _, mr := range m.MockResponses {
					method.MockResponses = append(method.MockResponses, restMockResponseToDomain(mr, m.ID))
				}
				res.Methods = append(res.Methods, method)
			}
			app.Resources = append(app.Resources, res)
		}
		p.Applications = append(p.Applications, app)
	}
	return p
}

func restMockResponseToDomain(mr restMockResponseFile, methodID string) domain.RestMockResponse {
	r := domain.RestMockResponse{
		ID:                  mr.ID,
		Name:                mr.Name,
		Body:                mr.Body,
		MethodID:            methodID,
		HTTPStatusCode:      defaultStatusCode(mr.HTTPStatusCode),
		Status:              defaultRestMockResponseStatus(mr.Status),
		UsingExpressions:    mr.UsingExpressions,
		HTTPHeaders:         make([]domain.HTTPHeader, 0, len(mr.HTTPHeaders)),
		ContentEncodings:    make([]domain.ContentEncoding, 0, len(mr.ContentEncodings)),
		ParameterQueries:    make([]domain.RestParameterQuery, 0, len(mr.ParameterQueries)),
		XPathExpressions:    make([]domain.RestXPathExpression, 0, len(mr.XPathExpressions)),
		JSONPathExpressions: make([]domain.RestJSONPathExpression, 0, len(mr.JSONPathExpressions)),
		HeaderQueries:       make([]domain.RestHeaderQuery, 0, len(mr.HeaderQueries)),
	}
	for _, h := range mr.HTTPHeaders {
		r.HTTPHeaders = append(r.HTTPHeaders, domain.HTTPHeader{Name: h.Name, Value: h.Value})
	}
	for _, e := range mr.ContentEncodings {
		r.ContentEncodings = append(r.ContentEncodings, domain.ContentEncoding(e))
	}
	for _, q := range mr.ParameterQueries {
		r.ParameterQueries = append(r.ParameterQueries, domain.RestParameterQuery{
			Parameter: q.Parameter, Query: q.Query,
			MatchCase: q.MatchCase, MatchAny: q.MatchAny,
			MatchRegex: q.MatchRegex, URLEncoded: q.URLEncoded,
		})
	}
	for _, x := range mr.XPathExpressions {
		r.XPathExpressions = append(r.XPathExpressions, domain.RestXPathExpression{Expression: x})
	}
	for _, x := range mr.JSONPathExpressions {
		r.JSONPathExpressions = append(r.JSONPathExpressions, domain.RestJSONPathExpression{Expression: x})
	}
	for _, hq := range mr.HeaderQueries {
		r.HeaderQueries = append(r.HeaderQueries, domain.RestHeaderQuery{
			Header: hq.Header, Query: hq.Query,
			MatchCase: hq.MatchCase, MatchAny: hq.MatchAny, MatchRegex: hq.MatchRegex,
		})
	}
	return r
}

// --- REST project: domain -> file ---

func restProjectFromDomain(p domain.RestProject) restProjectFile {
	f := restProjectFile{
		SchemaVersion: schemaVersion,
		Kind:          kindRestProject,
		ID:            p.ID,
		Name:          p.Name,
		Description:   p.Description,
		Created:       fileTime{Time: p.Created.Time},
		Updated:       fileTime{Time: p.Updated.Time},
		Applications:  make([]restApplicationFile, 0, len(p.Applications)),
	}
	for _, a := range p.Applications {
		af := restApplicationFile{
			ID:        a.ID,
			Name:      a.Name,
			Resources: make([]restResourceFile, 0, len(a.Resources)),
		}
		for _, r := range a.Resources {
			rf := restResourceFile{
				ID:      r.ID,
				Name:    r.Name,
				URI:     r.URI,
				Methods: make([]restMethodFile, 0, len(r.Methods)),
			}
			for _, m := range r.Methods {
				mf := restMethodFile{
					ID:                    m.ID,
					Name:                  m.Name,
					HTTPMethod:            string(m.HTTPMethod),
					Status:                string(m.Status),
					ResponseStrategy:      string(m.ResponseStrategy),
					DefaultBody:           m.DefaultBody,
					ForwardedEndpoint:     m.ForwardedEndpoint,
					SimulateNetworkDelay:  m.SimulateNetworkDelay,
					NetworkDelay:          m.NetworkDelay,
					AutomaticForward:      m.AutomaticForward,
					DefaultMockResponseID: m.DefaultMockResponseID,
					MockResponses:         make([]restMockResponseFile, 0, len(m.MockResponses)),
				}
				for _, mr := range m.MockResponses {
					mf.MockResponses = append(mf.MockResponses, restMockResponseFromDomain(mr))
				}
				rf.Methods = append(rf.Methods, mf)
			}
			af.Resources = append(af.Resources, rf)
		}
		f.Applications = append(f.Applications, af)
	}
	return f
}

func restMockResponseFromDomain(mr domain.RestMockResponse) restMockResponseFile {
	f := restMockResponseFile{
		ID:                  mr.ID,
		Name:                mr.Name,
		Body:                mr.Body,
		HTTPStatusCode:      mr.HTTPStatusCode,
		Status:              string(mr.Status),
		UsingExpressions:    mr.UsingExpressions,
		HTTPHeaders:         make([]httpHeaderFile, 0, len(mr.HTTPHeaders)),
		ContentEncodings:    make([]string, 0, len(mr.ContentEncodings)),
		ParameterQueries:    make([]restParameterQueryFile, 0, len(mr.ParameterQueries)),
		XPathExpressions:    make(expressionList, 0, len(mr.XPathExpressions)),
		JSONPathExpressions: make(expressionList, 0, len(mr.JSONPathExpressions)),
		HeaderQueries:       make([]restHeaderQueryFile, 0, len(mr.HeaderQueries)),
	}
	for _, h := range mr.HTTPHeaders {
		f.HTTPHeaders = append(f.HTTPHeaders, httpHeaderFile{Name: h.Name, Value: h.Value})
	}
	for _, e := range mr.ContentEncodings {
		f.ContentEncodings = append(f.ContentEncodings, string(e))
	}
	for _, q := range mr.ParameterQueries {
		f.ParameterQueries = append(f.ParameterQueries, restParameterQueryFile{
			Parameter: q.Parameter, Query: q.Query,
			MatchCase: q.MatchCase, MatchAny: q.MatchAny,
			MatchRegex: q.MatchRegex, URLEncoded: q.URLEncoded,
		})
	}
	for _, x := range mr.XPathExpressions {
		f.XPathExpressions = append(f.XPathExpressions, x.Expression)
	}
	for _, x := range mr.JSONPathExpressions {
		f.JSONPathExpressions = append(f.JSONPathExpressions, x.Expression)
	}
	for _, hq := range mr.HeaderQueries {
		f.HeaderQueries = append(f.HeaderQueries, restHeaderQueryFile{
			Header: hq.Header, Query: hq.Query,
			MatchCase: hq.MatchCase, MatchAny: hq.MatchAny, MatchRegex: hq.MatchRegex,
		})
	}
	return f
}

// --- SOAP project: file -> domain ---

func soapProjectToDomain(f soapProjectFile) domain.SoapProject {
	p := domain.SoapProject{
		Project: domain.Project{
			ID:          f.ID,
			Name:        f.Name,
			Description: f.Description,
			Created:     domain.Time{Time: f.Created.Time},
			Updated:     domain.Time{Time: f.Updated.Time},
		},
		Resources: make([]domain.SoapResource, 0, len(f.Resources)),
		Ports:     make([]domain.SoapPort, 0, len(f.Ports)),
	}
	for _, r := range f.Resources {
		p.Resources = append(p.Resources, domain.SoapResource{
			ID:        r.ID,
			Name:      r.Name,
			ProjectID: f.ID,
			Type:      domain.SoapResourceType(defaultString(string(r.Type), string(domain.SoapResourceWSDL))),
		})
	}
	for _, po := range f.Ports {
		port := domain.SoapPort{
			ID:         po.ID,
			Name:       po.Name,
			URI:        po.URI,
			ProjectID:  f.ID,
			Operations: make([]domain.SoapOperation, 0, len(po.Operations)),
		}
		for _, op := range po.Operations {
			o := domain.SoapOperation{
				ID:   op.ID,
				Name: op.Name,
				OperationIdentifier: domain.SoapOperationIdentifier{
					Name:      op.OperationIdentifier.Name,
					Namespace: op.OperationIdentifier.Namespace,
				},
				HTTPMethod:            defaultSOAPHTTPMethod(op.HTTPMethod),
				SoapVersion:           defaultSoapVersion(op.SoapVersion),
				Status:                defaultSoapOperationStatus(op.Status),
				ResponseStrategy:      defaultSoapStrategy(op.ResponseStrategy),
				IdentifyStrategy:      defaultIdentifyStrategy(op.IdentifyStrategy),
				DefaultBody:           op.DefaultBody,
				ForwardedEndpoint:     op.ForwardedEndpoint,
				OriginalEndpoint:      op.OriginalEndpoint,
				SimulateNetworkDelay:  op.SimulateNetworkDelay,
				NetworkDelay:          op.NetworkDelay,
				MockOnFailure:         op.MockOnFailure,
				AutomaticForward:      op.AutomaticForward,
				DefaultMockResponseID: op.DefaultMockResponseID,
				PortID:                po.ID,
				MockResponses:         make([]domain.SoapMockResponse, 0, len(op.MockResponses)),
			}
			for _, mr := range op.MockResponses {
				o.MockResponses = append(o.MockResponses, soapMockResponseToDomain(mr, op.ID))
			}
			port.Operations = append(port.Operations, o)
		}
		p.Ports = append(p.Ports, port)
	}
	return p
}

func soapMockResponseToDomain(mr soapMockResponseFile, operationID string) domain.SoapMockResponse {
	r := domain.SoapMockResponse{
		ID:               mr.ID,
		Name:             mr.Name,
		Body:             mr.Body,
		OperationID:      operationID,
		HTTPStatusCode:   defaultStatusCode(mr.HTTPStatusCode),
		Status:           domain.SoapMockResponseStatus(defaultString(string(mr.Status), string(domain.SoapMockResponseEnabled))),
		UsingExpressions: mr.UsingExpressions,
		HTTPHeaders:      make([]domain.HTTPHeader, 0, len(mr.HTTPHeaders)),
		ContentEncodings: make([]domain.ContentEncoding, 0, len(mr.ContentEncodings)),
		XPathExpressions: make([]domain.SoapXPathExpression, 0, len(mr.XPathExpressions)),
	}
	for _, h := range mr.HTTPHeaders {
		r.HTTPHeaders = append(r.HTTPHeaders, domain.HTTPHeader{Name: h.Name, Value: h.Value})
	}
	for _, e := range mr.ContentEncodings {
		r.ContentEncodings = append(r.ContentEncodings, domain.ContentEncoding(e))
	}
	for _, x := range mr.XPathExpressions {
		r.XPathExpressions = append(r.XPathExpressions, domain.SoapXPathExpression{Expression: x})
	}
	return r
}

// --- SOAP project: domain -> file ---

func soapProjectFromDomain(p domain.SoapProject) soapProjectFile {
	f := soapProjectFile{
		SchemaVersion: schemaVersion,
		Kind:          kindSoapProject,
		ID:            p.ID,
		Name:          p.Name,
		Description:   p.Description,
		Created:       fileTime{Time: p.Created.Time},
		Updated:       fileTime{Time: p.Updated.Time},
		Resources:     make([]soapResourceFile, 0, len(p.Resources)),
		Ports:         make([]soapPortFile, 0, len(p.Ports)),
	}
	for _, r := range p.Resources {
		f.Resources = append(f.Resources, soapResourceFile{
			ID:   r.ID,
			Name: r.Name,
			Type: string(r.Type),
		})
	}
	for _, po := range p.Ports {
		pf := soapPortFile{
			ID:         po.ID,
			Name:       po.Name,
			URI:        po.URI,
			Operations: make([]soapOperationFile, 0, len(po.Operations)),
		}
		for _, op := range po.Operations {
			of := soapOperationFile{
				ID:   op.ID,
				Name: op.Name,
				OperationIdentifier: soapOperationIDFile{
					Name:      op.OperationIdentifier.Name,
					Namespace: op.OperationIdentifier.Namespace,
				},
				HTTPMethod:            string(op.HTTPMethod),
				SoapVersion:           string(op.SoapVersion),
				Status:                string(op.Status),
				ResponseStrategy:      string(op.ResponseStrategy),
				IdentifyStrategy:      string(op.IdentifyStrategy),
				DefaultBody:           op.DefaultBody,
				ForwardedEndpoint:     op.ForwardedEndpoint,
				OriginalEndpoint:      op.OriginalEndpoint,
				SimulateNetworkDelay:  op.SimulateNetworkDelay,
				NetworkDelay:          op.NetworkDelay,
				MockOnFailure:         op.MockOnFailure,
				AutomaticForward:      op.AutomaticForward,
				DefaultMockResponseID: op.DefaultMockResponseID,
				MockResponses:         make([]soapMockResponseFile, 0, len(op.MockResponses)),
			}
			for _, mr := range op.MockResponses {
				of.MockResponses = append(of.MockResponses, soapMockResponseFromDomain(mr))
			}
			pf.Operations = append(pf.Operations, of)
		}
		f.Ports = append(f.Ports, pf)
	}
	return f
}

func soapMockResponseFromDomain(mr domain.SoapMockResponse) soapMockResponseFile {
	f := soapMockResponseFile{
		ID:               mr.ID,
		Name:             mr.Name,
		Body:             mr.Body,
		HTTPStatusCode:   mr.HTTPStatusCode,
		Status:           string(mr.Status),
		UsingExpressions: mr.UsingExpressions,
		HTTPHeaders:      make([]httpHeaderFile, 0, len(mr.HTTPHeaders)),
		ContentEncodings: make([]string, 0, len(mr.ContentEncodings)),
		XPathExpressions: make(expressionList, 0, len(mr.XPathExpressions)),
	}
	for _, h := range mr.HTTPHeaders {
		f.HTTPHeaders = append(f.HTTPHeaders, httpHeaderFile{Name: h.Name, Value: h.Value})
	}
	for _, e := range mr.ContentEncodings {
		f.ContentEncodings = append(f.ContentEncodings, string(e))
	}
	for _, x := range mr.XPathExpressions {
		f.XPathExpressions = append(f.XPathExpressions, x.Expression)
	}
	return f
}

// --- users ---

func userToDomain(f userFile) domain.User {
	return domain.User{
		ID:       f.ID,
		Username: f.Username,
		Password: f.Password,
		Email:    f.Email,
		FullName: f.FullName,
		Created:  domain.Time{Time: f.Created.Time},
		Updated:  domain.Time{Time: f.Updated.Time},
		Status:   domain.UserStatus(defaultString(f.Status, string(domain.UserActive))),
		Role:     domain.Role(defaultString(f.Role, string(domain.RoleReader))),
	}
}

func userFromDomain(u domain.User) userFile {
	return userFile{
		ID:       u.ID,
		Username: u.Username,
		Password: u.Password,
		Email:    u.Email,
		FullName: u.FullName,
		Status:   string(u.Status),
		Role:     string(u.Role),
		Created:  fileTime{Time: u.Created.Time},
		Updated:  fileTime{Time: u.Updated.Time},
	}
}

// --- defaults for hand-written seeds and files missing optional enum fields ---

func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func defaultHTTPMethod(m string) domain.HTTPMethod {
	return domain.HTTPMethod(defaultString(m, string(domain.MethodGET)))
}

func defaultSOAPHTTPMethod(m string) domain.HTTPMethod {
	return domain.HTTPMethod(defaultString(m, string(domain.MethodPOST)))
}

func defaultRestMethodStatus(s string) domain.RestMethodStatus {
	return domain.RestMethodStatus(defaultString(s, string(domain.RestMethodMocked)))
}

func defaultRestMockResponseStatus(s string) domain.RestMockResponseStatus {
	return domain.RestMockResponseStatus(defaultString(s, string(domain.RestMockResponseEnabled)))
}

func defaultRestStrategy(s string) domain.RestResponseStrategy {
	return domain.RestResponseStrategy(defaultString(s, string(domain.RestStrategyRandom)))
}

func defaultSoapOperationStatus(s string) domain.SoapOperationStatus {
	return domain.SoapOperationStatus(defaultString(s, string(domain.SoapOperationMocked)))
}

func defaultSoapStrategy(s string) domain.SoapResponseStrategy {
	return domain.SoapResponseStrategy(defaultString(s, string(domain.SoapStrategyRandom)))
}

func defaultIdentifyStrategy(s string) domain.SoapOperationIdentifyStrategy {
	return domain.SoapOperationIdentifyStrategy(defaultString(s, string(domain.IdentifyByElementNamespace)))
}

func defaultSoapVersion(s string) domain.SoapVersion {
	return domain.SoapVersion(defaultString(s, string(domain.SOAP11)))
}

func defaultStatusCode(code int) int {
	if code == 0 {
		return 200
	}
	return code
}

// --- strict decoding (data dir; schema checked, no invented ids) ---

func decodeRestProjectFile(data []byte) (restProjectFile, error) {
	var f restProjectFile
	if err := unmarshalFile(data, &f); err != nil {
		return f, fmt.Errorf("decode rest project file: %w", err)
	}
	if err := checkSchemaVersion(f.SchemaVersion, "rest project file"); err != nil {
		return f, err
	}
	if f.Kind != "" && f.Kind != kindRestProject {
		return f, fmt.Errorf("decode rest project file: kind is %q, want %q", f.Kind, kindRestProject)
	}
	return f, nil
}

func decodeSoapProjectFile(data []byte) (soapProjectFile, error) {
	var f soapProjectFile
	if err := unmarshalFile(data, &f); err != nil {
		return f, fmt.Errorf("decode soap project file: %w", err)
	}
	if err := checkSchemaVersion(f.SchemaVersion, "soap project file"); err != nil {
		return f, err
	}
	if f.Kind != "" && f.Kind != kindSoapProject {
		return f, fmt.Errorf("decode soap project file: kind is %q, want %q", f.Kind, kindSoapProject)
	}
	return f, nil
}

func decodeUsersFile(data []byte) (usersFile, error) {
	var f usersFile
	if err := unmarshalFile(data, &f); err != nil {
		return f, fmt.Errorf("decode users file: %w", err)
	}
	if err := checkSchemaVersion(f.SchemaVersion, "users file"); err != nil {
		return f, err
	}
	return f, nil
}

func decodeStateFile(data []byte) (stateFile, error) {
	var f stateFile
	if err := unmarshalFile(data, &f); err != nil {
		return f, fmt.Errorf("decode state file: %w", err)
	}
	if err := checkSchemaVersion(f.SchemaVersion, "state file"); err != nil {
		return f, err
	}
	return f, nil
}

func decodeRestEventFile(data []byte) (restEventFile, error) {
	var f restEventFile
	if err := unmarshalFile(data, &f); err != nil {
		return f, fmt.Errorf("decode rest event file: %w", err)
	}
	if err := checkSchemaVersion(f.SchemaVersion, "rest event file"); err != nil {
		return f, err
	}
	return f, nil
}

func decodeSoapEventFile(data []byte) (soapEventFile, error) {
	var f soapEventFile
	if err := unmarshalFile(data, &f); err != nil {
		return f, fmt.Errorf("decode soap event file: %w", err)
	}
	if err := checkSchemaVersion(f.SchemaVersion, "soap event file"); err != nil {
		return f, err
	}
	return f, nil
}
