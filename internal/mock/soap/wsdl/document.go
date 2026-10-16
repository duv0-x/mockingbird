package wsdl

import (
	"fmt"
	"strings"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/mock/soap/xml"
)

const (
	wsdlNamespace   = "http://schemas.xmlsoap.org/wsdl/"
	soap11Namespace = "http://schemas.xmlsoap.org/wsdl/soap/"
	soap12Namespace = "http://schemas.xmlsoap.org/wsdl/soap12/"
)

// document is a parsed WSDL document (a main file or a followed import).
type document struct {
	name string
	kind domain.SoapResourceType
	raw  string
	root *xml.Element
}

// qname is a qualified attribute value (WsdlParser.getAttribute /
// Converter Attribute). The prefix is resolved against the document root
// namespace declarations, like CastleMock's WsdlNamespaceParser +
// AttributeConverter.
type qname struct {
	prefix string
	local  string
}

type binding struct {
	name       string
	typ        qname
	operations []bindingOperation
}

// bindingIO keeps the soap:body of a binding input/output; parts is the
// optional parts attribute (hasParts distinguishes absent from empty).
type bindingIO struct {
	parts    string
	hasParts bool
}

type bindingOperation struct {
	name   string
	input  *bindingIO
	output *bindingIO
}

type message struct {
	name  string
	parts []messagePart
}

type messagePart struct {
	name    string
	element *qname
}

type portType struct {
	name       string
	operations []portTypeOperation
}

type portTypeOperation struct {
	name   string
	input  *qname
	output *qname
}

type servicePort struct {
	name     string
	binding  qname
	location string
	version  domain.SoapVersion
}

// parseDocument parses raw as a WSDL document named name.
func parseDocument(name string, kind domain.SoapResourceType, raw string) (document, error) {
	root, err := xml.Parse(raw)
	if err != nil {
		return document{}, fmt.Errorf("%w: %s: %w", ErrInvalidWSDL, name, err)
	}
	return document{name: name, kind: kind, raw: raw, root: root}, nil
}

// importLocations ports SoapPortConverter.getImports: wsdl:import/@location
// values, resolved against base by the caller.
func (d document) importLocations() []string {
	var out []string
	for _, element := range d.root.Descendants(wsdlNamespace, "import") {
		if location, ok := attribute(element, "location"); ok {
			out = append(out, location)
		}
	}
	return out
}

// soapPorts ports DocumentConverter.toSoapParts: one SoapPort per
// wsdl:service/wsdl:port with a soap:address or soap12:address.
func (d document) soapPorts(projectID string, generateResponse bool) ([]domain.SoapPort, error) {
	namespaces := rootNamespaces(d.root)
	bindings, err := parseBindings(d.root)
	if err != nil {
		return nil, err
	}
	messages, err := parseMessages(d.root)
	if err != nil {
		return nil, err
	}
	portTypes, err := parsePortTypes(d.root)
	if err != nil {
		return nil, err
	}
	servicePorts, err := parseServicePorts(d.root)
	if err != nil {
		return nil, err
	}

	var ports []domain.SoapPort
	for _, sp := range servicePorts {
		port, err := sp.toSoapPort(projectID, bindings, portTypes, messages, namespaces, generateResponse)
		if err != nil {
			return nil, err
		}
		ports = append(ports, port)
	}
	return ports, nil
}

// rootNamespaces ports WsdlNamespaceParser: the xmlns declarations of the
// document root, keyed by prefix.
func rootNamespaces(root *xml.Element) map[string]string {
	namespaces := map[string]string{}
	for _, attr := range root.Attrs {
		if prefix, ok := cutPrefix(attr.Name, "xmlns:"); ok {
			namespaces[prefix] = attr.Value
		}
	}
	return namespaces
}

func cutPrefix(s, prefix string) (string, bool) {
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):], true
	}
	return "", false
}

// attribute mirrors DocumentUtility.getAttribute: empty means absent, and the
// value loses everything up to its first colon ("tns:Foo" → "Foo"; an
// absolute import location "http://h/x" becomes the network-path reference
// "//h/x", which still resolves against the importing URL).
func attribute(element *xml.Element, name string) (string, bool) {
	value, _ := element.Attribute(name)
	if value == "" {
		return "", false
	}
	parts := strings.Split(value, ":")
	if len(parts) == 1 {
		return parts[0], true
	}
	return parts[1], true
}

// requiredAttribute mirrors DocumentUtility.getAttribute(...).orElseThrow.
func requiredAttribute(element *xml.Element, name, what string) (string, error) {
	value, ok := attribute(element, name)
	if !ok {
		return "", fmt.Errorf("wsdl: unable to find %s", what)
	}
	return value, nil
}

// parseQName ports WsdlParser.getAttribute: "ns:name" → {ns, name}; a missing
// or repeated colon makes the value unusable (ok == false).
func parseQName(element *xml.Element, name string) (qname, bool) {
	value, ok := element.Attribute(name)
	if !ok || value == "" {
		return qname{}, false
	}
	switch parts := strings.Split(value, ":"); len(parts) {
	case 1:
		return qname{local: parts[0]}, true
	case 2:
		return qname{prefix: parts[0], local: parts[1]}, true
	}
	return qname{}, false
}

// parseBindings ports WsdlBindingParser.
func parseBindings(root *xml.Element) ([]binding, error) {
	var out []binding
	for _, element := range root.Descendants(wsdlNamespace, "binding") {
		name, err := requiredAttribute(element, "name", "binding name")
		if err != nil {
			return nil, err
		}
		typ, ok := parseQName(element, "type")
		if !ok {
			return nil, fmt.Errorf("wsdl: unable to find type attribute")
		}
		b := binding{name: name, typ: typ}
		for _, opElement := range element.Descendants(wsdlNamespace, "operation") {
			opName, err := requiredAttribute(opElement, "name", "operation name")
			if err != nil {
				return nil, err
			}
			op := bindingOperation{name: opName}
			if input := firstChild(opElement, wsdlNamespace, "input"); input != nil {
				op.input = parseBody(input)
			}
			if output := firstChild(opElement, wsdlNamespace, "output"); output != nil {
				op.output = parseBody(output)
			}
			b.operations = append(b.operations, op)
		}
		out = append(out, b)
	}
	return out, nil
}

// parseBody extracts the soap:body (SOAP 1.1 first, then SOAP 1.2) of a
// binding input/output element. Absent body → nil; body without parts →
// hasParts false.
func parseBody(element *xml.Element) *bindingIO {
	body := firstChild(element, soap11Namespace, "body")
	if body == nil {
		body = firstChild(element, soap12Namespace, "body")
	}
	if body == nil {
		return nil
	}
	parts, hasParts := attribute(body, "parts")
	return &bindingIO{parts: parts, hasParts: hasParts}
}

// parseMessages ports WsdlMessageParser.
func parseMessages(root *xml.Element) ([]message, error) {
	var out []message
	for _, element := range root.Descendants(wsdlNamespace, "message") {
		name, err := requiredAttribute(element, "name", "message name")
		if err != nil {
			return nil, err
		}
		m := message{name: name}
		for _, partElement := range element.Descendants(wsdlNamespace, "part") {
			partName, ok := attribute(partElement, "name")
			if !ok {
				continue
			}
			part := messagePart{name: partName}
			if element, ok := parseQName(partElement, "element"); ok {
				part.element = &element
			}
			m.parts = append(m.parts, part)
		}
		out = append(out, m)
	}
	return out, nil
}

// parsePortTypes ports WsdlPortTypeParser.
func parsePortTypes(root *xml.Element) ([]portType, error) {
	var out []portType
	for _, element := range root.Descendants(wsdlNamespace, "portType") {
		name, err := requiredAttribute(element, "name", "port type name")
		if err != nil {
			return nil, err
		}
		pt := portType{name: name}
		for _, opElement := range element.Descendants(wsdlNamespace, "operation") {
			opName, err := requiredAttribute(opElement, "name", "operation name")
			if err != nil {
				return nil, err
			}
			op := portTypeOperation{name: opName}
			if input := firstChild(opElement, wsdlNamespace, "input"); input != nil {
				if message, ok := parseQName(input, "message"); ok {
					op.input = &message
				}
			}
			if output := firstChild(opElement, wsdlNamespace, "output"); output != nil {
				if message, ok := parseQName(output, "message"); ok {
					op.output = &message
				}
			}
			pt.operations = append(pt.operations, op)
		}
		out = append(out, pt)
	}
	return out, nil
}

// parseServicePorts ports WsdlServiceParser: ports without a SOAP address are
// skipped.
func parseServicePorts(root *xml.Element) ([]servicePort, error) {
	var out []servicePort
	for _, service := range root.Descendants(wsdlNamespace, "service") {
		if _, err := requiredAttribute(service, "name", "service name"); err != nil {
			return nil, err
		}
		for _, portElement := range service.Descendants(wsdlNamespace, "port") {
			location, version, found := soapAddress(portElement)
			if !found {
				continue
			}
			name, err := requiredAttribute(portElement, "name", "service port name")
			if err != nil {
				return nil, err
			}
			bindingRef, ok := parseQName(portElement, "binding")
			if !ok {
				return nil, fmt.Errorf("wsdl: unable to find binding attribute")
			}
			out = append(out, servicePort{name: name, binding: bindingRef, location: location, version: version})
		}
	}
	return out, nil
}

// soapAddress ports SoapUtility.extractSoapAddress as used by
// WsdlServiceParser: SOAP 1.1 address first, SOAP 1.2 otherwise.
func soapAddress(portElement *xml.Element) (location string, version domain.SoapVersion, found bool) {
	for _, ns := range []struct {
		ns      string
		version domain.SoapVersion
	}{{soap11Namespace, domain.SOAP11}, {soap12Namespace, domain.SOAP12}} {
		// element.getAttribute: a missing location is "".
		if address := firstChild(portElement, ns.ns, "address"); address != nil {
			location, _ := address.Attribute("location")
			return location, ns.version, true
		}
	}
	return "", "", false
}

// firstChild is the first descendant (document order) with the given namespace
// and local name (DocumentUtility.getElement(element, ns, name)).
func firstChild(element *xml.Element, ns, local string) *xml.Element {
	elements := element.Descendants(ns, local)
	if len(elements) == 0 {
		return nil
	}
	return elements[0]
}

// toSoapPort ports ServicePortConverter.toSoapPort.
func (sp servicePort) toSoapPort(projectID string, bindings []binding, portTypes []portType, messages []message, namespaces map[string]string, generateResponse bool) (domain.SoapPort, error) {
	b := findBinding(bindings, sp.binding.local)
	if b == nil {
		return domain.SoapPort{}, fmt.Errorf("wsdl: unable to find the binding")
	}
	pt := findPortType(portTypes, b.typ.local)
	if pt == nil {
		return domain.SoapPort{}, fmt.Errorf("wsdl: unable to find the port type")
	}

	portID := domain.NewID()
	port := domain.SoapPort{
		ID:            portID,
		Name:          sp.name,
		URI:           sp.name,
		InvokeAddress: sp.name,
		ProjectID:     projectID,
	}
	for _, op := range b.operations {
		operation, err := op.toSoapOperation(portID, *pt, messages, namespaces, sp, generateResponse)
		if err != nil {
			return domain.SoapPort{}, err
		}
		port.Operations = append(port.Operations, operation)
	}
	return port, nil
}

func findBinding(bindings []binding, name string) *binding {
	for i := range bindings {
		if bindings[i].name == name {
			return &bindings[i]
		}
	}
	return nil
}

func findPortType(portTypes []portType, name string) *portType {
	for i := range portTypes {
		if portTypes[i].name == name {
			return &portTypes[i]
		}
	}
	return nil
}

func findMessage(messages []message, name string) *message {
	for i := range messages {
		if messages[i].name == name {
			return &messages[i]
		}
	}
	return nil
}

// toSoapOperation ports ServicePortConverter.toSoapOperation with
// BindingOperationConverter, MessagePartConverter and AttributeConverter.
func (op bindingOperation) toSoapOperation(portID string, pt portType, messages []message, namespaces map[string]string, sp servicePort, generateResponse bool) (domain.SoapOperation, error) {
	var ptOperation *portTypeOperation
	for i := range pt.operations {
		if pt.operations[i].name == op.name {
			ptOperation = &pt.operations[i]
			break
		}
	}
	if ptOperation == nil {
		return domain.SoapOperation{}, fmt.Errorf("wsdl: unable to find port type with the following name: %s", op.name)
	}

	var inputMessage, outputMessage *message
	if ptOperation.input != nil {
		inputMessage = findMessage(messages, ptOperation.input.local)
	}
	if ptOperation.output != nil {
		outputMessage = findMessage(messages, ptOperation.output.local)
	}

	requestIdentifier := operationIdentifier(op.input, inputMessage, op.name, namespaces)
	responseIdentifier := operationIdentifier(op.output, outputMessage, op.name, namespaces)
	body := defaultBody(responseIdentifier)

	operationID := domain.NewID()
	operation := domain.SoapOperation{
		ID:                           operationID,
		Name:                         op.name,
		PortID:                       portID,
		OperationIdentifier:          requestIdentifier,
		ResponseStrategy:             domain.SoapStrategyRandom,
		Status:                       domain.SoapOperationMocked,
		HTTPMethod:                   domain.MethodPOST,
		SoapVersion:                  sp.version,
		DefaultBody:                  body,
		ForwardedEndpoint:            sp.location,
		OriginalEndpoint:             sp.location,
		SimulateNetworkDelay:         domain.Ptr(false),
		MockOnFailure:                domain.Ptr(false),
		AutomaticForward:             domain.Ptr(false),
		IdentifyStrategy:             domain.IdentifyByElementNamespace,
		CurrentResponseSequenceIndex: 0,
	}
	if generateResponse {
		operation.MockResponses = []domain.SoapMockResponse{{
			ID:               domain.NewID(),
			Name:             "Auto-generated mocked response",
			Body:             body,
			OperationID:      operationID,
			Status:           domain.SoapMockResponseEnabled,
			HTTPStatusCode:   200,
			UsingExpressions: domain.Ptr(false),
		}}
	}
	return operation, nil
}

// operationIdentifier ports BindingOperationConverter.toSoapOperationIdentifier*:
// the binding body part resolved through the message, else the operation name
// without namespace.
func operationIdentifier(io *bindingIO, msg *message, operationName string, namespaces map[string]string) domain.SoapOperationIdentifier {
	if io != nil {
		if part := selectPart(io, msg); part != nil {
			if part.element != nil {
				return domain.SoapOperationIdentifier{
					Name:      part.element.local,
					Namespace: namespaces[part.element.prefix],
				}
			}
			return domain.SoapOperationIdentifier{Name: part.name}
		}
	}
	return domain.SoapOperationIdentifier{Name: operationName}
}

// selectPart ports BindingOperationConverter.find*Message: with an explicit
// parts attribute the named part; without it, the single part of the message.
func selectPart(io *bindingIO, msg *message) *messagePart {
	if msg == nil {
		return nil
	}
	if io.hasParts {
		for i := range msg.parts {
			if msg.parts[i].name == io.parts {
				return &msg.parts[i]
			}
		}
		return nil
	}
	if len(msg.parts) == 1 {
		return &msg.parts[0]
	}
	return nil
}

// defaultBody ports SoapOperationIdentifierConverter.toDefaultBody.
func defaultBody(identifier domain.SoapOperationIdentifier) string {
	return `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:web="` + identifier.Namespace + `">` + "\n" +
		`   <soapenv:Header/>` + "\n" +
		`   <soapenv:Body>` + "\n" +
		`      <web:` + identifier.Name + `>?</web:` + identifier.Name + `>` + "\n" +
		`   </soapenv:Body>` + "\n" +
		`</soapenv:Envelope>`
}
