package jsonfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// schemaVersion is the only native on-disk layout version mockingbird
// understands (SPEC.md §3.2). Decoding a file with a greater version fails
// with ErrUnsupportedSchema; the store then refuses to start.
const schemaVersion = 1

const (
	kindRestProject = "RestProject"
	kindSoapProject = "SoapProject"
)

// ErrUnsupportedSchema marks data files written by a newer mockingbird.
// Open treats it as fatal instead of skipping the file.
var ErrUnsupportedSchema = errors.New("unsupported schema version")

// fileTime is the timestamp format of the data dir: RFC 3339, UTC,
// milliseconds, "Z" suffix (SPEC.md §3.1: 2024-05-01T10:20:30.123Z).
type fileTime struct{ time.Time }

const fileTimeLayout = "2006-01-02T15:04:05.000Z07:00"

func (t fileTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.UTC().Format(fileTimeLayout))
}

func (t *fileTime) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" {
		t.Time = time.Time{}
		return nil
	}
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		t.Time = time.UnixMilli(ms).UTC()
		return nil
	}
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return fmt.Errorf("decode file time: %w", err)
	}
	for _, layout := range []string{fileTimeLayout, "2006-01-02T15:04:05.000-0700", time.RFC3339Nano} {
		if parsed, err := time.Parse(layout, str); err == nil {
			t.Time = parsed.UTC()
			return nil
		}
	}
	return fmt.Errorf("decode file time: unsupported format %q", str)
}

type httpHeaderFile struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// REST project file, SPEC.md §3.2.2. Parent ids are implied by nesting and
// currentResponseSequenceIndex lives in state/sequence.json (D-13), so
// neither is written.
type restProjectFile struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Kind          string                `json:"kind"`
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	Description   string                `json:"description,omitempty"`
	Created       fileTime              `json:"created"`
	Updated       fileTime              `json:"updated"`
	Applications  []restApplicationFile `json:"applications"`
}

type restApplicationFile struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Resources []restResourceFile `json:"resources"`
}

type restResourceFile struct {
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	URI     string           `json:"uri"`
	Methods []restMethodFile `json:"methods"`
}

type restMethodFile struct {
	ID                    string                 `json:"id"`
	Name                  string                 `json:"name"`
	HTTPMethod            string                 `json:"httpMethod"`
	Status                string                 `json:"status"`
	ResponseStrategy      string                 `json:"responseStrategy"`
	DefaultBody           string                 `json:"defaultBody,omitempty"`
	ForwardedEndpoint     string                 `json:"forwardedEndpoint,omitempty"`
	SimulateNetworkDelay  *bool                  `json:"simulateNetworkDelay,omitempty"`
	NetworkDelay          *int64                 `json:"networkDelay,omitempty"`
	AutomaticForward      *bool                  `json:"automaticForward,omitempty"`
	DefaultMockResponseID string                 `json:"defaultMockResponseId,omitempty"`
	MockResponses         []restMockResponseFile `json:"mockResponses"`
}

type restMockResponseFile struct {
	ID                  string                   `json:"id"`
	Name                string                   `json:"name"`
	Status              string                   `json:"status"`
	HTTPStatusCode      int                      `json:"httpStatusCode,omitempty"`
	UsingExpressions    *bool                    `json:"usingExpressions,omitempty"`
	Body                string                   `json:"body,omitempty"`
	HTTPHeaders         []httpHeaderFile         `json:"httpHeaders"`
	ContentEncodings    []string                 `json:"contentEncodings"`
	ParameterQueries    []restParameterQueryFile `json:"parameterQueries"`
	HeaderQueries       []restHeaderQueryFile    `json:"headerQueries"`
	XPathExpressions    expressionList           `json:"xpathExpressions"`
	JSONPathExpressions expressionList           `json:"jsonPathExpressions"`
}

type restParameterQueryFile struct {
	Parameter  string `json:"parameter"`
	Query      string `json:"query"`
	MatchCase  bool   `json:"matchCase"`
	MatchAny   bool   `json:"matchAny"`
	MatchRegex bool   `json:"matchRegex"`
	URLEncoded bool   `json:"urlEncoded"`
}

type restHeaderQueryFile struct {
	Header     string `json:"header"`
	Query      string `json:"query"`
	MatchCase  bool   `json:"matchCase"`
	MatchAny   bool   `json:"matchAny"`
	MatchRegex bool   `json:"matchRegex"`
}

// expressionList is written as an array of strings (SPEC.md §3.2.2). It also
// reads the admin API shape [{"expression": "..."}], so hand-written seeds
// may use either.
type expressionList []string

func (l *expressionList) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("decode expressions: %w", err)
	}
	out := make(expressionList, 0, len(raw))
	for _, r := range raw {
		var s string
		if err := json.Unmarshal(r, &s); err == nil {
			out = append(out, s)
			continue
		}
		var obj struct {
			Expression string `json:"expression"`
		}
		if err := json.Unmarshal(r, &obj); err != nil {
			return fmt.Errorf("decode expression: %w", err)
		}
		out = append(out, obj.Expression)
	}
	*l = out
	return nil
}

// SOAP project file, SPEC.md §3.2.3. Resource content lives in separate
// files; "content" is only written for seeds and JSON exports.
type soapProjectFile struct {
	SchemaVersion int                `json:"schemaVersion"`
	Kind          string             `json:"kind"`
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Description   string             `json:"description,omitempty"`
	Created       fileTime           `json:"created"`
	Updated       fileTime           `json:"updated"`
	Resources     []soapResourceFile `json:"resources"`
	Ports         []soapPortFile     `json:"ports"`
}

type soapResourceFile struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
}

type soapPortFile struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	URI        string              `json:"uri"`
	Operations []soapOperationFile `json:"operations"`
}

type soapOperationFile struct {
	ID                    string                 `json:"id"`
	Name                  string                 `json:"name"`
	OperationIdentifier   soapOperationIDFile    `json:"operationIdentifier"`
	HTTPMethod            string                 `json:"httpMethod"`
	SoapVersion           string                 `json:"soapVersion"`
	Status                string                 `json:"status"`
	ResponseStrategy      string                 `json:"responseStrategy"`
	IdentifyStrategy      string                 `json:"identifyStrategy"`
	DefaultBody           string                 `json:"defaultBody,omitempty"`
	ForwardedEndpoint     string                 `json:"forwardedEndpoint,omitempty"`
	OriginalEndpoint      string                 `json:"originalEndpoint,omitempty"`
	SimulateNetworkDelay  *bool                  `json:"simulateNetworkDelay,omitempty"`
	NetworkDelay          *int64                 `json:"networkDelay,omitempty"`
	MockOnFailure         *bool                  `json:"mockOnFailure,omitempty"`
	AutomaticForward      *bool                  `json:"automaticForward,omitempty"`
	DefaultMockResponseID string                 `json:"defaultMockResponseId,omitempty"`
	MockResponses         []soapMockResponseFile `json:"mockResponses"`
}

type soapOperationIDFile struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

type soapMockResponseFile struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Status           string           `json:"status"`
	HTTPStatusCode   int              `json:"httpStatusCode,omitempty"`
	UsingExpressions *bool            `json:"usingExpressions,omitempty"`
	Body             string           `json:"body,omitempty"`
	HTTPHeaders      []httpHeaderFile `json:"httpHeaders"`
	ContentEncodings []string         `json:"contentEncodings"`
	XPathExpressions expressionList   `json:"xpathExpressions"`
}

type usersFile struct {
	SchemaVersion int        `json:"schemaVersion"`
	Users         []userFile `json:"users"`
}

// userFile field order follows the SPEC.md §3.2.4 example; email and
// fullName are plain fields there ("" when unset), matching UserFile's
// nullable strings.
type userFile struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	Email    string   `json:"email"`
	FullName string   `json:"fullName"`
	Status   string   `json:"status"`
	Role     string   `json:"role"`
	Created  fileTime `json:"created"`
	Updated  fileTime `json:"updated"`
}

// Event files embed the admin API JSON of the event (SPEC.md §3.2.5), so
// they reuse the domain wire format (including the "+00:00" time layout).
type restEventFile struct {
	SchemaVersion int              `json:"schemaVersion"`
	Kind          string           `json:"kind"`
	Event         domain.RestEvent `json:"event"`
}

type soapEventFile struct {
	SchemaVersion int              `json:"schemaVersion"`
	Kind          string           `json:"kind"`
	Event         domain.SoapEvent `json:"event"`
}

type stateFile struct {
	SchemaVersion int            `json:"schemaVersion"`
	SequenceIndex map[string]int `json:"sequenceIndex"`
}

// metaFile is the data dir marker (SPEC.md §3.1).
type metaFile struct {
	SchemaVersion int    `json:"schemaVersion"`
	CreatedBy     string `json:"createdBy"`
}

func checkSchemaVersion(got int, what string) error {
	if got > schemaVersion {
		return fmt.Errorf("%w: %s has schemaVersion %d, supported is %d",
			ErrUnsupportedSchema, what, got, schemaVersion)
	}
	return nil
}
