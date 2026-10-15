package domain

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"
)

type HTTPMethod string

const (
	MethodGET     HTTPMethod = "GET"
	MethodPOST    HTTPMethod = "POST"
	MethodPUT     HTTPMethod = "PUT"
	MethodHEAD    HTTPMethod = "HEAD"
	MethodDELETE  HTTPMethod = "DELETE"
	MethodOPTIONS HTTPMethod = "OPTIONS"
	MethodTRACE   HTTPMethod = "TRACE"
	MethodPATCH   HTTPMethod = "PATCH"
)

var HTTPMethods = []HTTPMethod{MethodGET, MethodPOST, MethodPUT, MethodHEAD, MethodDELETE, MethodOPTIONS, MethodTRACE, MethodPATCH}

func ParseHTTPMethod(s string) (HTTPMethod, error) {
	m := HTTPMethod(strings.ToUpper(s))
	if !slices.Contains(HTTPMethods, m) {
		return "", fmt.Errorf("unknown http method %q", s)
	}
	return m, nil
}

type ContentEncoding string

const (
	EncodingGZIP    ContentEncoding = "GZIP"
	EncodingDEFLATE ContentEncoding = "DEFLATE"
)

type HTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HTTPParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Time serializes like Spring Boot's default Jackson date format
// ("2006-01-02T15:04:05.000+00:00", always UTC) and accepts that format,
// RFC 3339 and epoch milliseconds on input.
type Time struct{ time.Time }

const javaTimeLayout = "2006-01-02T15:04:05.000-07:00"

func Now() Time { return Time{time.Now().UTC()} }

func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.UTC().Format(javaTimeLayout))
}

func (t *Time) UnmarshalJSON(b []byte) error {
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
		return fmt.Errorf("decode time: %w", err)
	}
	for _, layout := range []string{javaTimeLayout, time.RFC3339Nano, "2006-01-02T15:04:05.000-0700"} {
		if parsed, err := time.Parse(layout, str); err == nil {
			t.Time = parsed.UTC()
			return nil
		}
	}
	return fmt.Errorf("decode time: unsupported format %q", str)
}

const idAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// NewID mirrors IdUtility.generateId(): 6 random alphanumeric characters.
func NewID() string {
	b := make([]byte, 6)
	max := big.NewInt(int64(len(idAlphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(fmt.Errorf("generate id: %w", err))
		}
		b[i] = idAlphabet[n.Int64()]
	}
	return string(b)
}

func Ptr[T any](v T) *T { return &v }

func Deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
