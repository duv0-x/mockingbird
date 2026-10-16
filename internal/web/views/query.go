package views

import (
	"net/url"
	"strconv"
)

// projectsQuery encodes the overview filter; empty values are left out.
func projectsQuery(q, typ string, page int) string {
	v := url.Values{}
	if q != "" {
		v.Set("q", q)
	}
	if typ != "" {
		v.Set("type", typ)
	}
	if page > 1 {
		v.Set("page", strconv.Itoa(page))
	}
	if len(v) == 0 {
		return ""
	}
	return "?" + v.Encode()
}
