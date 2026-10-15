package rest

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// errorBody is Spring Boot's default error JSON (DefaultErrorAttributes).
type errorBody struct {
	Timestamp domain.Time `json:"timestamp"`
	Status    int         `json:"status"`
	Error     string      `json:"error"`
	Path      string      `json:"path"`
}

func writeError(w http.ResponseWriter, status int, path string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Timestamp: domain.Now(), Status: status, Error: http.StatusText(status), Path: path})
}

// writeResponse writes a mock, forwarded or echo response. Headers are set in
// order so a repeated name keeps its last value (Spring HttpHeaders.put);
// Content-Encoding is never copied because the body is not encoded, and the
// framing headers are recomputed for the body actually written.
func writeResponse(w http.ResponseWriter, resp *domain.RestResponse, accept string) {
	for _, h := range resp.HTTPHeaders {
		if strings.EqualFold(h.Name, "Content-Encoding") || strings.EqualFold(h.Name, "Content-Length") ||
			strings.EqualFold(h.Name, "Transfer-Encoding") {
			continue
		}
		w.Header().Set(h.Name, h.Value)
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", negotiateContentType(accept))
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(resp.Body)))
	w.WriteHeader(resp.HTTPStatusCode)
	_, _ = w.Write([]byte(resp.Body))
}

const defaultContentType = "text/plain;charset=UTF-8" // D-9

type mediaRange struct {
	value    string // type/subtype, lower-cased, without parameters
	quality  float64
	wildcard int // 0 concrete, 1 type/*, 2 */*
}

// negotiateContentType reproduces what Spring's StringHttpMessageConverter
// does for a ResponseEntity<String> without Content-Type: the most acceptable
// concrete media type of the Accept header (text/plain for "*/*"), with the
// UTF-8 charset except for JSON (D-9 sets the default charset to UTF-8).
func negotiateContentType(accept string) string {
	if strings.TrimSpace(accept) == "" {
		return defaultContentType
	}
	var ranges []mediaRange
	for _, part := range strings.Split(accept, ",") {
		params := strings.Split(part, ";")
		value := strings.ToLower(strings.TrimSpace(params[0]))
		if !strings.Contains(value, "/") {
			continue
		}
		q := 1.0
		for _, p := range params[1:] {
			if k, v, ok := strings.Cut(strings.TrimSpace(p), "="); ok && strings.EqualFold(k, "q") {
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					q = f
				}
			}
		}
		if q <= 0 {
			continue
		}
		wildcard := 0
		switch {
		case value == "*/*":
			wildcard = 2
		case strings.HasSuffix(value, "/*"):
			wildcard = 1
		}
		ranges = append(ranges, mediaRange{value: value, quality: q, wildcard: wildcard})
	}
	slices.SortStableFunc(ranges, func(a, b mediaRange) int {
		if a.quality != b.quality {
			if a.quality > b.quality {
				return -1
			}
			return 1
		}
		return a.wildcard - b.wildcard
	})
	for _, r := range ranges {
		switch {
		case r.wildcard == 0:
			if r.value == "application/json" || strings.HasSuffix(r.value, "+json") {
				return r.value
			}
			return r.value + ";charset=UTF-8"
		case r.value == "*/*" || r.value == "text/*":
			return defaultContentType
		}
	}
	return defaultContentType
}
