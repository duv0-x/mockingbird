package forward

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/duv0-x/mockingbird/internal/domain"
)

func gzipped(s string) []byte {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, _ = w.Write([]byte(s))
	_ = w.Close()
	return b.Bytes()
}

func deflated(s string) []byte {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	_, _ = w.Write([]byte(s))
	_ = w.Close()
	return b.Bytes()
}

func TestDo(t *testing.T) {
	t.Parallel()
	var got struct {
		method, uri, body, custom, host string
		hasConnection                   bool
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.method, got.uri, got.body = r.Method, r.RequestURI, string(b)
		got.custom, got.host = r.Header.Get("X-Custom"), r.Host
		_, got.hasConnection = r.Header["Keep-Alive"]
		switch r.URL.Path {
		case "/gzip":
			w.Header().Set("Content-Encoding", "gzip")
			_, _ = w.Write(gzipped("hello gzip\r\n"))
		case "/deflate":
			w.Header().Set("Content-Encoding", "deflate")
			_, _ = w.Write(deflated("hello deflate"))
		case "/missing":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("nope"))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.Header().Add("X-Multi", "a")
			w.Header().Add("X-Multi", "b")
			_, _ = w.Write([]byte("{\"a\":1}\r\n"))
		}
	}))
	t.Cleanup(srv.Close)
	client := NewClient(Options{Timeout: 5 * time.Second, TLSVerify: true})
	body := "payload"

	tests := []struct {
		name     string
		req      Request
		status   int
		body     string
		enc      []domain.ContentEncoding
		wantHdrs []domain.HTTPHeader
	}{
		{
			name: "plain with headers and body",
			req: Request{Method: domain.MethodPOST, URL: srv.URL + "/plain?a=1", Body: &body,
				Headers: []domain.HTTPHeader{{Name: "X-Custom", Value: "v"}, {Name: "Host", Value: "evil"}, {Name: "Keep-Alive", Value: "1"}, {Name: "Content-Length", Value: "99"}}},
			status: 200, body: "{\"a\":1}\r\n", enc: []domain.ContentEncoding{},
			wantHdrs: []domain.HTTPHeader{{Name: "Content-Type", Value: "application/json"}, {Name: "X-Multi", Value: "a"}, {Name: "X-Multi", Value: "b"}},
		},
		{name: "gzip decoded", req: Request{Method: domain.MethodGET, URL: srv.URL + "/gzip"}, status: 200, body: "hello gzip\r\n", enc: []domain.ContentEncoding{domain.EncodingGZIP}},
		{name: "deflate decoded", req: Request{Method: domain.MethodGET, URL: srv.URL + "/deflate"}, status: 200, body: "hello deflate", enc: []domain.ContentEncoding{domain.EncodingDEFLATE}},
		{name: "4xx is a response", req: Request{Method: domain.MethodGET, URL: srv.URL + "/missing"}, status: 404, body: "nope", enc: []domain.ContentEncoding{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.Do(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			if resp.StatusCode != tt.status || resp.Body != tt.body || !slices.Equal(resp.Encodings, tt.enc) {
				t.Errorf("got %d %q %v, want %d %q %v", resp.StatusCode, resp.Body, resp.Encodings, tt.status, tt.body, tt.enc)
			}
			for _, h := range resp.Headers {
				if h.Name == "Content-Length" || h.Name == "Transfer-Encoding" {
					t.Errorf("header %s must be dropped", h.Name)
				}
			}
			for _, want := range tt.wantHdrs {
				if !slices.Contains(resp.Headers, want) {
					t.Errorf("missing header %v in %v", want, resp.Headers)
				}
			}
		})
	}
	if got.method != "GET" {
		t.Errorf("last method = %s", got.method)
	}
	resp, err := client.Do(context.Background(), tests[0].req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp
	if got.uri != "/plain?a=1" || got.body != "payload" || got.custom != "v" || got.hasConnection || got.host == "evil" {
		t.Errorf("forwarded request = %+v", got)
	}
}

func TestDoTransportError(t *testing.T) {
	t.Parallel()
	client := NewClient(Options{Timeout: time.Second, TLSVerify: true})
	if _, err := client.Do(context.Background(), Request{Method: domain.MethodGET, URL: "http://127.0.0.1:1/x"}); err == nil {
		t.Fatal("want error")
	}
	if _, err := client.Do(context.Background(), Request{Method: domain.MethodGET, URL: "://bad"}); err == nil {
		t.Fatal("want error for bad URL")
	}
}
