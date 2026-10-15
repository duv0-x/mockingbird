package domain

// EnsureSlices replaces nil slices/maps with empty ones so the API encodes
// [] / {} instead of null, matching the Java builders' List.of()/Map.of() defaults.
func (p *RestProject) EnsureSlices() {
	if p.Applications == nil {
		p.Applications = []RestApplication{}
	}
	for i := range p.Applications {
		p.Applications[i].EnsureSlices()
	}
}

func (a *RestApplication) EnsureSlices() {
	if a.Resources == nil {
		a.Resources = []RestResource{}
	}
	if a.StatusCount == nil {
		a.StatusCount = map[RestMethodStatus]int{}
	}
	for i := range a.Resources {
		a.Resources[i].EnsureSlices()
	}
}

func (r *RestResource) EnsureSlices() {
	if r.Methods == nil {
		r.Methods = []RestMethod{}
	}
	if r.StatusCount == nil {
		r.StatusCount = map[RestMethodStatus]int{}
	}
	for i := range r.Methods {
		r.Methods[i].EnsureSlices()
	}
}

func (m *RestMethod) EnsureSlices() {
	if m.MockResponses == nil {
		m.MockResponses = []RestMockResponse{}
	}
	for i := range m.MockResponses {
		m.MockResponses[i].EnsureSlices()
	}
}

func (r *RestMockResponse) EnsureSlices() {
	if r.HTTPHeaders == nil {
		r.HTTPHeaders = []HTTPHeader{}
	}
	if r.ContentEncodings == nil {
		r.ContentEncodings = []ContentEncoding{}
	}
	if r.ParameterQueries == nil {
		r.ParameterQueries = []RestParameterQuery{}
	}
	if r.XPathExpressions == nil {
		r.XPathExpressions = []RestXPathExpression{}
	}
	if r.JSONPathExpressions == nil {
		r.JSONPathExpressions = []RestJSONPathExpression{}
	}
	if r.HeaderQueries == nil {
		r.HeaderQueries = []RestHeaderQuery{}
	}
}

func (p *SoapProject) EnsureSlices() {
	if p.Ports == nil {
		p.Ports = []SoapPort{}
	}
	if p.Resources == nil {
		p.Resources = []SoapResource{}
	}
	if p.StatusCount == nil {
		p.StatusCount = map[SoapOperationStatus]int{}
	}
	for i := range p.Ports {
		p.Ports[i].EnsureSlices()
	}
}

func (p *SoapPort) EnsureSlices() {
	if p.Operations == nil {
		p.Operations = []SoapOperation{}
	}
	if p.StatusCount == nil {
		p.StatusCount = map[SoapOperationStatus]int{}
	}
	for i := range p.Operations {
		p.Operations[i].EnsureSlices()
	}
}

func (o *SoapOperation) EnsureSlices() {
	if o.MockResponses == nil {
		o.MockResponses = []SoapMockResponse{}
	}
	for i := range o.MockResponses {
		o.MockResponses[i].EnsureSlices()
	}
}

func (r *SoapMockResponse) EnsureSlices() {
	if r.HTTPHeaders == nil {
		r.HTTPHeaders = []HTTPHeader{}
	}
	if r.ContentEncodings == nil {
		r.ContentEncodings = []ContentEncoding{}
	}
	if r.XPathExpressions == nil {
		r.XPathExpressions = []SoapXPathExpression{}
	}
}
