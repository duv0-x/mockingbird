package service

import "github.com/duv0-x/mockingbird/internal/domain"

// Inputs mirror CastleMock's *Request models; JSON tags are the admin API
// request bodies (SPEC.md §5). Pointers mark optional fields.

type ProjectInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// ProjectType is sent by the CastleMock UI on create and ignored.
	ProjectType string `json:"projectType,omitempty"`
}

type RestMethodCreate struct {
	Name       string            `json:"name"`
	HTTPMethod domain.HTTPMethod `json:"httpMethod"`
}

type RestMethodUpdate struct {
	Name                  string                      `json:"name"`
	HTTPMethod            domain.HTTPMethod           `json:"httpMethod"`
	Status                domain.RestMethodStatus     `json:"status"`
	ResponseStrategy      domain.RestResponseStrategy `json:"responseStrategy"`
	ForwardedEndpoint     *string                     `json:"forwardedEndpoint"`
	SimulateNetworkDelay  *bool                       `json:"simulateNetworkDelay"`
	NetworkDelay          *int64                      `json:"networkDelay"`
	DefaultMockResponseID *string                     `json:"defaultMockResponseId"`
	AutomaticForward      *bool                       `json:"automaticForward"`
}

type RestMockResponseInput struct {
	Name                string                          `json:"name"`
	Body                *string                         `json:"body"`
	HTTPStatusCode      *int                            `json:"httpStatusCode"`
	Status              *domain.RestMockResponseStatus  `json:"status"`
	UsingExpressions    *bool                           `json:"usingExpressions"`
	HTTPHeaders         []domain.HTTPHeader             `json:"httpHeaders"`
	ContentEncodings    []domain.ContentEncoding        `json:"contentEncodings"`
	ParameterQueries    []domain.RestParameterQuery     `json:"parameterQueries"`
	XPathExpressions    []domain.RestXPathExpression    `json:"xpathExpressions"`
	JSONPathExpressions []domain.RestJSONPathExpression `json:"jsonPathExpressions"`
	HeaderQueries       []domain.RestHeaderQuery        `json:"headerQueries"`
}

type SoapOperationUpdate struct {
	ResponseStrategy      domain.SoapResponseStrategy          `json:"responseStrategy"`
	Status                domain.SoapOperationStatus           `json:"status"`
	IdentifyStrategy      domain.SoapOperationIdentifyStrategy `json:"identifyStrategy"`
	ForwardedEndpoint     *string                              `json:"forwardedEndpoint"`
	SimulateNetworkDelay  *bool                                `json:"simulateNetworkDelay"`
	NetworkDelay          *int64                               `json:"networkDelay"`
	DefaultMockResponseID *string                              `json:"defaultMockResponseId"`
	MockOnFailure         *bool                                `json:"mockOnFailure"`
	AutomaticForward      *bool                                `json:"automaticForward"`
}

type SoapMockResponseInput struct {
	Name             string                        `json:"name"`
	Body             string                        `json:"body"`
	Status           domain.SoapMockResponseStatus `json:"status"`
	HTTPStatusCode   int                           `json:"httpStatusCode"`
	UsingExpressions *bool                         `json:"usingExpressions"`
	HTTPHeaders      []domain.HTTPHeader           `json:"httpHeaders"`
	XPathExpressions []domain.SoapXPathExpression  `json:"xpathExpressions"`
}

type UserInput struct {
	Username string            `json:"username"`
	Password string            `json:"password"`
	Email    string            `json:"email"`
	FullName string            `json:"fullName"`
	Status   domain.UserStatus `json:"status"`
	Role     domain.Role       `json:"role"`
}

type ProfileInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	FullName string `json:"fullName"`
}

// Upload is a definition/WSDL/project source: uploaded bytes or a URL.
type Upload struct {
	Filename string
	Data     []byte
	URL      string
}

type ExportFormat string

const (
	ExportCastleMockXML ExportFormat = "xml"
	ExportNativeJSON    ExportFormat = "json"
)

type Export struct {
	Filename    string
	ContentType string
	Data        []byte
}
