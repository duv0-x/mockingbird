package domain

type RestMethodStatus string

const (
	RestMethodMocked     RestMethodStatus = "MOCKED"
	RestMethodDisabled   RestMethodStatus = "DISABLED"
	RestMethodForwarded  RestMethodStatus = "FORWARDED"
	RestMethodRecording  RestMethodStatus = "RECORDING"
	RestMethodRecordOnce RestMethodStatus = "RECORD_ONCE"
	RestMethodEcho       RestMethodStatus = "ECHO"
)

var RestMethodStatuses = []RestMethodStatus{RestMethodMocked, RestMethodDisabled, RestMethodForwarded, RestMethodRecording, RestMethodRecordOnce, RestMethodEcho}

type RestResponseStrategy string

const (
	RestStrategyRandom           RestResponseStrategy = "RANDOM"
	RestStrategySequence         RestResponseStrategy = "SEQUENCE"
	RestStrategyQueryMatch       RestResponseStrategy = "QUERY_MATCH"
	RestStrategyXPath            RestResponseStrategy = "XPATH"
	RestStrategyJSONPath         RestResponseStrategy = "JSON_PATH"
	RestStrategyHeaderQueryMatch RestResponseStrategy = "HEADER_QUERY_MATCH"
)

type RestMockResponseStatus string

const (
	RestMockResponseEnabled  RestMockResponseStatus = "ENABLED"
	RestMockResponseDisabled RestMockResponseStatus = "DISABLED"
)

type RestDefinitionType string

const (
	RestDefinitionSwagger RestDefinitionType = "SWAGGER"
	RestDefinitionOpenAPI RestDefinitionType = "OPENAPI"
	RestDefinitionWADL    RestDefinitionType = "WADL"
	RestDefinitionRAML    RestDefinitionType = "RAML"
)

type RestProject struct {
	Project
	Applications []RestApplication `json:"applications"`
}

type RestApplication struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	ProjectID string         `json:"projectId"`
	Resources []RestResource `json:"resources"`
	// StatusCount is computed on read (count of methods per status), never persisted.
	StatusCount map[RestMethodStatus]int `json:"statusCount"`
}

type RestResource struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	URI           string `json:"uri"`
	ApplicationID string `json:"applicationId"`
	// InvokeAddress is transient (only set by importers); never persisted.
	InvokeAddress string                   `json:"invokeAddress,omitempty"`
	Methods       []RestMethod             `json:"methods"`
	StatusCount   map[RestMethodStatus]int `json:"statusCount"`
}

type RestMethod struct {
	ID                           string               `json:"id"`
	Name                         string               `json:"name"`
	ResourceID                   string               `json:"resourceId"`
	DefaultBody                  string               `json:"defaultBody,omitempty"`
	HTTPMethod                   HTTPMethod           `json:"httpMethod"`
	ForwardedEndpoint            string               `json:"forwardedEndpoint,omitempty"`
	Status                       RestMethodStatus     `json:"status"`
	ResponseStrategy             RestResponseStrategy `json:"responseStrategy"`
	CurrentResponseSequenceIndex int                  `json:"currentResponseSequenceIndex"`
	SimulateNetworkDelay         *bool                `json:"simulateNetworkDelay"`
	NetworkDelay                 *int64               `json:"networkDelay"`
	DefaultMockResponseID        string               `json:"defaultMockResponseId,omitempty"`
	MockResponses                []RestMockResponse   `json:"mockResponses"`
	// URI (the parent resource URI) and DefaultResponseName are computed on read.
	URI                 string `json:"uri"`
	DefaultResponseName string `json:"defaultResponseName,omitempty"`
	AutomaticForward    *bool  `json:"automaticForward"`
}

type RestMockResponse struct {
	ID                  string                   `json:"id"`
	Name                string                   `json:"name"`
	Body                string                   `json:"body,omitempty"`
	MethodID            string                   `json:"methodId"`
	HTTPStatusCode      int                      `json:"httpStatusCode"`
	Status              RestMockResponseStatus   `json:"status"`
	UsingExpressions    *bool                    `json:"usingExpressions"`
	HTTPHeaders         []HTTPHeader             `json:"httpHeaders"`
	ContentEncodings    []ContentEncoding        `json:"contentEncodings"`
	ParameterQueries    []RestParameterQuery     `json:"parameterQueries"`
	XPathExpressions    []RestXPathExpression    `json:"xpathExpressions"`
	JSONPathExpressions []RestJSONPathExpression `json:"jsonPathExpressions"`
	HeaderQueries       []RestHeaderQuery        `json:"headerQueries"`
}

type RestParameterQuery struct {
	Parameter  string `json:"parameter"`
	Query      string `json:"query"`
	MatchCase  bool   `json:"matchCase"`
	MatchAny   bool   `json:"matchAny"`
	MatchRegex bool   `json:"matchRegex"`
	URLEncoded bool   `json:"urlEncoded"`
}

type RestHeaderQuery struct {
	Header     string `json:"header"`
	Query      string `json:"query"`
	MatchCase  bool   `json:"matchCase"`
	MatchAny   bool   `json:"matchAny"`
	MatchRegex bool   `json:"matchRegex"`
}

type RestXPathExpression struct {
	Expression string `json:"expression"`
}

type RestJSONPathExpression struct {
	Expression string `json:"expression"`
}

type RestRequest struct {
	Body           string          `json:"body,omitempty"`
	ContentType    string          `json:"contentType,omitempty"`
	URI            string          `json:"uri"`
	HTTPMethod     HTTPMethod      `json:"httpMethod"`
	HTTPHeaders    []HTTPHeader    `json:"httpHeaders"`
	HTTPParameters []HTTPParameter `json:"httpParameters"`
}

type RestResponse struct {
	Body             string            `json:"body,omitempty"`
	MockResponseName string            `json:"mockResponseName,omitempty"`
	HTTPStatusCode   int               `json:"httpStatusCode"`
	ContentType      string            `json:"contentType,omitempty"`
	HTTPHeaders      []HTTPHeader      `json:"httpHeaders"`
	ContentEncodings []ContentEncoding `json:"contentEncodings"`
}

type RestEvent struct {
	Event
	Request       RestRequest   `json:"request"`
	Response      *RestResponse `json:"response"`
	ProjectID     string        `json:"projectId"`
	ApplicationID string        `json:"applicationId"`
	ResourceID    string        `json:"resourceId"`
	MethodID      string        `json:"methodId"`
}

func NewRestStatusCount() map[RestMethodStatus]int {
	m := make(map[RestMethodStatus]int, len(RestMethodStatuses))
	for _, s := range RestMethodStatuses {
		m[s] = 0
	}
	return m
}
