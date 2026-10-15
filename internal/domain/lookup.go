package domain

// Tree lookups shared by the API (T4) and the mock engines (T2/T3).
// They return pointers into the receiver so callers can mutate in place
// inside storage Update callbacks.

func (p *RestProject) Application(id string) *RestApplication {
	for i := range p.Applications {
		if p.Applications[i].ID == id {
			return &p.Applications[i]
		}
	}
	return nil
}

func (a *RestApplication) Resource(id string) *RestResource {
	for i := range a.Resources {
		if a.Resources[i].ID == id {
			return &a.Resources[i]
		}
	}
	return nil
}

func (r *RestResource) Method(id string) *RestMethod {
	for i := range r.Methods {
		if r.Methods[i].ID == id {
			return &r.Methods[i]
		}
	}
	return nil
}

func (m *RestMethod) MockResponse(id string) *RestMockResponse {
	for i := range m.MockResponses {
		if m.MockResponses[i].ID == id {
			return &m.MockResponses[i]
		}
	}
	return nil
}

func (p *SoapProject) Port(id string) *SoapPort {
	for i := range p.Ports {
		if p.Ports[i].ID == id {
			return &p.Ports[i]
		}
	}
	return nil
}

func (p *SoapProject) Resource(id string) *SoapResource {
	for i := range p.Resources {
		if p.Resources[i].ID == id {
			return &p.Resources[i]
		}
	}
	return nil
}

func (p *SoapPort) Operation(id string) *SoapOperation {
	for i := range p.Operations {
		if p.Operations[i].ID == id {
			return &p.Operations[i]
		}
	}
	return nil
}

func (o *SoapOperation) MockResponse(id string) *SoapMockResponse {
	for i := range o.MockResponses {
		if o.MockResponses[i].ID == id {
			return &o.MockResponses[i]
		}
	}
	return nil
}
