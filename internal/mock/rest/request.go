package rest

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/duv0-x/mockingbird/internal/domain"
)

type parameter = domain.HTTPParameter

// incoming is the RestRequest built by AbstractRestServiceController.prepareRequest.
type incoming struct {
	projectID     string
	applicationID string
	method        domain.HTTPMethod
	uri           string // resource URI: raw path after the application prefix
	path          string // raw request path, for error bodies
	url           string // request URL without query (HttpServletRequest.getRequestURL)
	body          string
	contentType   string
	headers       []domain.HTTPHeader
	params        []parameter
}

// parseRequest splits {ctx}/mock/rest/project/{projectId}/application/{applicationId}/**.
func parseRequest(r *http.Request, contextPath string) (*incoming, error) {
	rawPath := rawRequestPath(r)
	prefix := contextPath + "/mock/rest/project/"
	rest, ok := strings.CutPrefix(rawPath, prefix)
	if !ok {
		return nil, fmt.Errorf("path %q is not under %s", rawPath, prefix)
	}
	parts := strings.SplitN(rest, "/", 4)
	if len(parts) < 3 || parts[1] != "application" || parts[0] == "" || parts[2] == "" {
		return nil, fmt.Errorf("path %q has no project and application", rawPath)
	}
	projectID, err := url.PathUnescape(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode project id: %w", err)
	}
	applicationID, err := url.PathUnescape(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode application id: %w", err)
	}
	method, err := domain.ParseHTTPMethod(r.Method)
	if err != nil {
		return nil, err
	}
	body, err := readBody(r.Body)
	if err != nil {
		return nil, err
	}
	uri := ""
	if len(parts) == 4 {
		uri = "/" + parts[3]
	}
	return &incoming{
		projectID:     projectID,
		applicationID: applicationID,
		method:        method,
		uri:           uri,
		path:          rawPath,
		url:           requestURL(r, rawPath),
		body:          body,
		contentType:   r.Header.Get("Content-Type"),
		headers:       requestHeaders(r),
		params:        queryParams(r.URL.RawQuery),
	}, nil
}

// rawRequestPath is HttpServletRequest.getRequestURI: the path as sent,
// still percent-encoded.
func rawRequestPath(r *http.Request) string {
	if r.RequestURI != "" && !strings.Contains(r.RequestURI, "://") {
		path, _, _ := strings.Cut(r.RequestURI, "?")
		return path
	}
	return r.URL.EscapedPath()
}

// requestURL is HttpServletRequest.getRequestURL: scheme://host[:port]/path.
func requestURL(r *http.Request, rawPath string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host + rawPath
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
// first value. Go moves Host out of the header map; it is put back because
// CastleMock sees it like any other header.
func requestHeaders(r *http.Request) []domain.HTTPHeader {
	names := make([]string, 0, len(r.Header)+1)
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

// queryParams decodes the query string keeping request order; identical
// name/value pairs collapse like the Java Set<HttpParameter>.
func queryParams(rawQuery string) []parameter {
	params := []parameter{}
	for _, pair := range strings.Split(rawQuery, "&") {
		if pair == "" {
			continue
		}
		name, value, _ := strings.Cut(pair, "=")
		n, err := url.QueryUnescape(name)
		if err != nil {
			continue
		}
		v, err := url.QueryUnescape(value)
		if err != nil {
			continue
		}
		p := parameter{Name: n, Value: v}
		if !slices.Contains(params, p) {
			params = append(params, p)
		}
	}
	return params
}

func headerValue(headers []domain.HTTPHeader, name string) (string, bool) {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value, true
		}
	}
	return "", false
}
