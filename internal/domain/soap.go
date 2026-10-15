package domain

import "strings"

type SoapOperationStatus string

const (
	SoapOperationMocked     SoapOperationStatus = "MOCKED"
	SoapOperationDisabled   SoapOperationStatus = "DISABLED"
	SoapOperationForwarded  SoapOperationStatus = "FORWARDED"
	SoapOperationRecording  SoapOperationStatus = "RECORDING"
	SoapOperationRecordOnce SoapOperationStatus = "RECORD_ONCE"
	SoapOperationEcho       SoapOperationStatus = "ECHO"
)

var SoapOperationStatuses = []SoapOperationStatus{SoapOperationMocked, SoapOperationDisabled, SoapOperationForwarded, SoapOperationRecording, SoapOperationRecordOnce, SoapOperationEcho}

type SoapResponseStrategy string

const (
	SoapStrategyRandom     SoapResponseStrategy = "RANDOM"
	SoapStrategySequence   SoapResponseStrategy = "SEQUENCE"
	SoapStrategyXPathInput SoapResponseStrategy = "XPATH_INPUT"
)

type SoapMockResponseStatus string

const (
	SoapMockResponseEnabled  SoapMockResponseStatus = "ENABLED"
	SoapMockResponseDisabled SoapMockResponseStatus = "DISABLED"
)

type SoapOperationIdentifyStrategy string

const (
	IdentifyByElement          SoapOperationIdentifyStrategy = "ELEMENT"
	IdentifyByElementNamespace SoapOperationIdentifyStrategy = "ELEMENT_NAMESPACE"
)

type SoapResourceType string

const (
	SoapResourceWSDL       SoapResourceType = "WSDL"
	SoapResourceSchema     SoapResourceType = "SCHEMA"
	SoapResourceWSDLImport SoapResourceType = "WSDL_IMPORT"
)

type SoapVersion string

const (
	SOAP11 SoapVersion = "SOAP11"
	SOAP12 SoapVersion = "SOAP12"
)

// ContentType mirrors SoapVersion.getContextType().
func (v SoapVersion) ContentType() string {
	if v == SOAP12 {
		return "application/soap+xml"
	}
	return "text/xml"
}

// SoapVersionFromContentType mirrors SoapVersion.convert(): SOAP12 when the
// content type contains "application/soap+xml", SOAP11 otherwise.
func SoapVersionFromContentType(contentType string) SoapVersion {
	if strings.Contains(contentType, SOAP12.ContentType()) {
		return SOAP12
	}
	return SOAP11
}

type SoapProject struct {
	Project
	Ports     []SoapPort     `json:"ports"`
	Resources []SoapResource `json:"resources"`
	// StatusCount exists in the Java DTO but is never filled at project level.
	StatusCount map[SoapOperationStatus]int `json:"statusCount"`
}

type SoapPort struct {
	ID            string                      `json:"id"`
	Name          string                      `json:"name"`
	URI           string                      `json:"uri"`
	ProjectID     string                      `json:"projectId"`
	InvokeAddress string                      `json:"invokeAddress,omitempty"`
	Operations    []SoapOperation             `json:"operations"`
	StatusCount   map[SoapOperationStatus]int `json:"statusCount"`
}

type SoapOperationIdentifier struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

type SoapOperation struct {
	ID                           string                        `json:"id"`
	Name                         string                        `json:"name"`
	OperationIdentifier          SoapOperationIdentifier       `json:"operationIdentifier"`
	ResponseStrategy             SoapResponseStrategy          `json:"responseStrategy"`
	Status                       SoapOperationStatus           `json:"status"`
	HTTPMethod                   HTTPMethod                    `json:"httpMethod"`
	SoapVersion                  SoapVersion                   `json:"soapVersion"`
	DefaultBody                  string                        `json:"defaultBody,omitempty"`
	CurrentResponseSequenceIndex int                           `json:"currentResponseSequenceIndex"`
	ForwardedEndpoint            string                        `json:"forwardedEndpoint,omitempty"`
	OriginalEndpoint             string                        `json:"originalEndpoint,omitempty"`
	SimulateNetworkDelay         *bool                         `json:"simulateNetworkDelay"`
	NetworkDelay                 *int64                        `json:"networkDelay"`
	DefaultMockResponseID        string                        `json:"defaultMockResponseId,omitempty"`
	PortID                       string                        `json:"portId"`
	MockOnFailure                *bool                         `json:"mockOnFailure"`
	IdentifyStrategy             SoapOperationIdentifyStrategy `json:"identifyStrategy"`
	MockResponses                []SoapMockResponse            `json:"mockResponses"`
	InvokeAddress                string                        `json:"invokeAddress,omitempty"`
	DefaultResponseName          string                        `json:"defaultResponseName,omitempty"`
	AutomaticForward             *bool                         `json:"automaticForward"`
}

type SoapMockResponse struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Body             string                 `json:"body"`
	OperationID      string                 `json:"operationId"`
	Status           SoapMockResponseStatus `json:"status"`
	HTTPStatusCode   int                    `json:"httpStatusCode"`
	UsingExpressions *bool                  `json:"usingExpressions"`
	HTTPHeaders      []HTTPHeader           `json:"httpHeaders"`
	ContentEncodings []ContentEncoding      `json:"contentEncodings"`
	XPathExpressions []SoapXPathExpression  `json:"xpathExpressions"`
}

type SoapXPathExpression struct {
	Expression string `json:"expression"`
}

// SoapResource is a WSDL or XSD attached to a SOAP project. Content is only
// populated by the /content endpoint, exports and imports.
type SoapResource struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	ProjectID string           `json:"projectId"`
	Content   string           `json:"content,omitempty"`
	Type      SoapResourceType `json:"type"`
}

type SoapRequest struct {
	Body                string                  `json:"body"`
	Envelope            string                  `json:"envelope,omitempty"`
	ContentType         string                  `json:"contentType,omitempty"`
	URI                 string                  `json:"uri"`
	HTTPMethod          HTTPMethod              `json:"httpMethod"`
	OperationName       string                  `json:"operationName,omitempty"`
	SoapVersion         SoapVersion             `json:"soapVersion"`
	HTTPHeaders         []HTTPHeader            `json:"httpHeaders"`
	OperationIdentifier SoapOperationIdentifier `json:"operationIdentifier"`
}

type SoapResponse struct {
	Body             string            `json:"body"`
	MockResponseName string            `json:"mockResponseName"`
	HTTPStatusCode   int               `json:"httpStatusCode"`
	ContentType      string            `json:"contentType,omitempty"`
	HTTPHeaders      []HTTPHeader      `json:"httpHeaders"`
	ContentEncodings []ContentEncoding `json:"contentEncodings"`
}

type SoapEvent struct {
	Event
	Request     SoapRequest   `json:"request"`
	Response    *SoapResponse `json:"response"`
	ProjectID   string        `json:"projectId"`
	PortID      string        `json:"portId"`
	OperationID string        `json:"operationId"`
}

func NewSoapStatusCount() map[SoapOperationStatus]int {
	m := make(map[SoapOperationStatus]int, len(SoapOperationStatuses))
	for _, s := range SoapOperationStatuses {
		m[s] = 0
	}
	return m
}
