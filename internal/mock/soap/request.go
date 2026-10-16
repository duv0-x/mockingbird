package soap

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"slices"
	"strings"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/mock/soap/xml"
)

// parseRequest ports HttpServletRequestConverter.toSoapRequest.
func parseRequest(r *http.Request, projectID, path, contextPath string) (domain.SoapRequest, error) {
	method, err := domain.ParseHTTPMethod(r.Method)
	if err != nil {
		return domain.SoapRequest{}, err
	}
	body, err := readBody(r.Body)
	if err != nil {
		return domain.SoapRequest{}, err
	}
	contentType := r.Header.Get("Content-Type")
	envelope := body
	if strings.HasPrefix(strings.ToLower(contentType), "multipart/") {
		envelope, err = mtomEnvelope(body, contentType)
		if err != nil {
			return domain.SoapRequest{}, err
		}
	}
	identifier, err := operationIdentifier(envelope)
	if err != nil {
		return domain.SoapRequest{}, err
	}
	// String.replace of "{ctx}/mock/soap/project/{id}/": without anything
	// after the project id the URI stays the full path, which matches no port.
	uri := strings.ReplaceAll(path, contextPath+projectPrefix+projectID+"/", "")
	return domain.SoapRequest{
		Body:                body,
		Envelope:            envelope,
		ContentType:         contentType,
		URI:                 uri,
		HTTPMethod:          method,
		SoapVersion:         domain.SoapVersionFromContentType(contentType),
		HTTPHeaders:         requestHeaders(r),
		OperationIdentifier: identifier,
	}, nil
}

// readBody mirrors HttpMessageSupport.getBody: lines read with readLine and
// joined with "\n", so line terminators become "\n" and one final terminator
// is dropped.
func readBody(body io.Reader) (string, error) {
	if body == nil {
		return "", nil
	}
	raw, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("read request body: %w", err)
	}
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSuffix(s, "\n"), nil
}

// requestHeaders mirrors extractHttpHeaders: one entry per name with its
// first value; Host is put back because Go moves it out of the header map.
func requestHeaders(r *http.Request) []domain.HTTPHeader {
	names := make([]string, 0, len(r.Header))
	for name := range r.Header {
		names = append(names, name)
	}
	slices.Sort(names)
	headers := make([]domain.HTTPHeader, 0, len(names)+1)
	if r.Host != "" {
		headers = append(headers, domain.HTTPHeader{Name: "Host", Value: r.Host})
	}
	for _, name := range names {
		if v := r.Header[name]; len(v) > 0 {
			headers = append(headers, domain.HTTPHeader{Name: name, Value: v[0]})
		}
	}
	return headers
}

// mtomEnvelope ports MtomUtility.extractMtomBody (CXF AttachmentDeserializer):
// the root part is the one whose Content-ID equals the "start" parameter,
// else the first part. Like CXF it tolerates parts whose headers are not
// followed by a blank line, and the content runs up to the next boundary.
func mtomEnvelope(body, contentType string) (string, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", fmt.Errorf("unable to extract the request body in the MTOM request: %w", err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		return "", errors.New("unable to extract the request body in the MTOM request: no boundary")
	}
	start := strings.Trim(strings.TrimSpace(params["start"]), "<>")
	delimiter := "--" + boundary
	chunks := strings.Split(body, delimiter)
	var first *string
	for _, chunk := range chunks[1:] {
		if strings.HasPrefix(chunk, "--") {
			break
		}
		chunk = strings.TrimPrefix(strings.TrimPrefix(chunk, "\r"), "\n")
		contentID, content := splitPart(chunk)
		if first == nil {
			first = &content
		}
		if start != "" && strings.Trim(contentID, "<> ") == start {
			return content, nil
		}
	}
	if first == nil {
		return "", errors.New("unable to extract the request body in the MTOM request: no parts")
	}
	return *first, nil
}

// splitPart separates the MIME headers of a part from its content. Headers
// end at a blank line or at the first line that is not a "Name: value" header.
func splitPart(part string) (contentID, content string) {
	rest := part
	for {
		line, after, found := strings.Cut(rest, "\n")
		trimmed := strings.TrimSuffix(line, "\r")
		if trimmed == "" {
			return contentID, after
		}
		name, value, isHeader := strings.Cut(trimmed, ":")
		if !isHeader || strings.ContainsAny(name, " <\t") {
			return contentID, rest
		}
		if strings.EqualFold(name, "Content-ID") {
			contentID = strings.TrimSpace(value)
		}
		if !found {
			return contentID, ""
		}
		rest = after
	}
}

// operationIdentifier ports SoapUtility.extractSoapRequestName: the first
// element child of the document element's child named Body (any prefix,
// case-insensitive); name is its local name and namespace the one bound to
// its prefix, or the default namespace when unprefixed, with standard XML
// scoping (D-10).
func operationIdentifier(envelope string) (domain.SoapOperationIdentifier, error) {
	root, err := xml.Parse(envelope)
	if err != nil {
		return domain.SoapOperationIdentifier{}, fmt.Errorf("unable to extract the SOAP request name: %w", err)
	}
	var body *xml.Element
	for _, child := range root.Children {
		if strings.EqualFold(child.LocalName(), "Body") {
			body = child
			break
		}
	}
	if body == nil {
		return domain.SoapOperationIdentifier{}, errors.New("unable to extract the SOAP body")
	}
	if len(body.Children) == 0 {
		return domain.SoapOperationIdentifier{}, errors.New("unable to extract the service name")
	}
	message := body.Children[0]
	return domain.SoapOperationIdentifier{Name: message.LocalName(), Namespace: message.Namespace()}, nil
}
