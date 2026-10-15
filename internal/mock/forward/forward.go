// Package forward is the outbound HTTP client used by FORWARDED, RECORDING,
// RECORD_ONCE and automatic-forward modes (SPEC.md §4.3, §4.6).
//
// Owner: T2. Consumers: T3.
package forward

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/duv0-x/mockingbird/internal/domain"
)

type Options struct {
	Timeout   time.Duration
	TLSVerify bool
}

type Request struct {
	Method  domain.HTTPMethod
	URL     string
	Headers []domain.HTTPHeader
	Body    *string
}

type Response struct {
	StatusCode int
	Headers    []domain.HTTPHeader
	Body       string
	Encodings  []domain.ContentEncoding
}

type Client struct {
	opts Options
	http *http.Client
}

func NewClient(opts Options) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// The body is decoded here (HttpMessageSupport.extractHttpBody), and the
	// caller's Accept-Encoding is forwarded as is.
	transport.DisableCompression = true
	if !opts.TLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // MOCKINGBIRD_TLS_VERIFY=false, like -Dhttp.sslverify=false
	}
	return &Client{opts: opts, http: &http.Client{Transport: transport, Timeout: opts.Timeout}}
}

// hopByHop headers are connection specific (RFC 9110 §7.6.1) and are never
// forwarded; Host and Content-Length are recomputed for the new request.
var hopByHop = []string{
	"connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "proxy-connection",
	"te", "trailer", "transfer-encoding", "upgrade", "host", "content-length",
}

// Do sends req and returns the decoded (gunzipped/inflated) response. A
// transport failure is an error; any HTTP status is a Response.
//
// CastleMock (HttpURLConnection) fails on 4xx responses because it reads them
// through getInputStream; here they are returned like any other status.
func (c *Client) Do(ctx context.Context, req Request) (Response, error) {
	var body io.Reader
	if req.Body != nil {
		body = strings.NewReader(*req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, string(req.Method), req.URL, body)
	if err != nil {
		return Response{}, fmt.Errorf("forward: build request to %s: %w", req.URL, err)
	}
	for _, h := range req.Headers {
		if slices.Contains(hopByHop, strings.ToLower(h.Name)) {
			continue
		}
		httpReq.Header.Add(h.Name, h.Value)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("forward: %s %s: %w", req.Method, req.URL, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("forward: read response of %s: %w", req.URL, err)
	}
	encodings := contentEncodings(resp.Header.Get("Content-Encoding"))
	decoded, err := decode(raw, encodings)
	if err != nil {
		return Response{}, fmt.Errorf("forward: decode response of %s: %w", req.URL, err)
	}
	return Response{
		StatusCode: resp.StatusCode,
		Headers:    responseHeaders(resp.Header),
		Body:       string(decoded),
		Encodings:  encodings,
	}, nil
}

// contentEncodings mirrors HttpMessageSupport.extractContentEncoding: every
// known encoding whose name appears in the header value.
func contentEncodings(header string) []domain.ContentEncoding {
	upper := strings.ToUpper(header)
	encodings := []domain.ContentEncoding{}
	for _, e := range []domain.ContentEncoding{domain.EncodingGZIP, domain.EncodingDEFLATE} {
		if strings.Contains(upper, string(e)) {
			encodings = append(encodings, e)
		}
	}
	return encodings
}

// decode undoes GZIP (preferred, like the Java code) or DEFLATE (zlib stream,
// as InflaterInputStream expects). The bytes are otherwise passed through
// unchanged (D-6).
func decode(raw []byte, encodings []domain.ContentEncoding) ([]byte, error) {
	var r io.ReadCloser
	var err error
	switch {
	case slices.Contains(encodings, domain.EncodingGZIP):
		r, err = gzip.NewReader(bytes.NewReader(raw))
	case slices.Contains(encodings, domain.EncodingDEFLATE):
		r, err = zlib.NewReader(bytes.NewReader(raw))
	default:
		return raw, nil
	}
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return out, nil
}

// responseHeaders mirrors HttpMessageSupport.extractHttpHeaders(connection):
// every value of every header except Transfer-Encoding and Content-Length.
// Names are sorted because Go does not keep the wire order.
func responseHeaders(h http.Header) []domain.HTTPHeader {
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	slices.Sort(names)
	headers := []domain.HTTPHeader{}
	for _, name := range names {
		if strings.EqualFold(name, "Transfer-Encoding") || strings.EqualFold(name, "Content-Length") {
			continue
		}
		for _, v := range h[name] {
			headers = append(headers, domain.HTTPHeader{Name: name, Value: v})
		}
	}
	return headers
}
