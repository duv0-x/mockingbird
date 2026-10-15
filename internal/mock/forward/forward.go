// Package forward is the outbound HTTP client used by FORWARDED, RECORDING,
// RECORD_ONCE and automatic-forward modes (SPEC.md §4.3, §4.6).
//
// Owner: T2. Consumers: T3.
package forward

import (
	"context"
	"errors"
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
}

func NewClient(opts Options) *Client { return &Client{opts: opts} }

// Do sends req and returns the decoded (gunzipped/inflated) response. A
// transport failure is an error; any HTTP status is a Response.
func (c *Client) Do(ctx context.Context, req Request) (Response, error) {
	return Response{}, errors.New("forward: not implemented (T2)")
}
