package views

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// icons holds the inner markup of the Lucide icons (ISC licence) the UI uses
// (DESIGN.md §10). They are rendered inline so no icon font or sprite is needed.
var icons = map[string]string{
	"folder":          `<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/>`,
	"activity":        `<path d="M22 12h-2.48a2 2 0 0 0-1.93 1.46l-2.35 8.36a.25.25 0 0 1-.48 0L9.24 2.18a.25.25 0 0 0-.48 0l-2.35 8.36A2 2 0 0 1 4.49 12H2"/>`,
	"users":           `<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>`,
	"user":            `<path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>`,
	"settings":        `<path d="M20 7h-9"/><path d="M14 17H5"/><circle cx="17" cy="17" r="3"/><circle cx="7" cy="7" r="3"/>`,
	"search":          `<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>`,
	"plus":            `<path d="M5 12h14"/><path d="M12 5v14"/>`,
	"upload":          `<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" x2="12" y1="3" y2="15"/>`,
	"download":        `<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/>`,
	"copy":            `<rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>`,
	"check":           `<path d="M20 6 9 17l-5-5"/>`,
	"pencil":          `<path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/><path d="m15 5 4 4"/>`,
	"trash":           `<path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/><line x1="10" x2="10" y1="11" y2="17"/><line x1="14" x2="14" y1="11" y2="17"/>`,
	"more":            `<circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/>`,
	"chevron-right":   `<path d="m9 18 6-6-6-6"/>`,
	"chevron-down":    `<path d="m6 9 6 6 6-6"/>`,
	"chevron-left":    `<path d="m15 18-6-6 6-6"/>`,
	"arrow-right":     `<path d="M5 12h14"/><path d="m12 5 7 7-7 7"/>`,
	"x":               `<path d="M18 6 6 18"/><path d="m6 6 12 12"/>`,
	"sun":             `<circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/>`,
	"moon":            `<path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/>`,
	"monitor":         `<rect width="20" height="14" x="2" y="3" rx="2"/><line x1="8" x2="16" y1="21" y2="21"/><line x1="12" x2="12" y1="17" y2="21"/>`,
	"log-out":         `<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" x2="9" y1="12" y2="12"/>`,
	"book":            `<path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/>`,
	"help":            `<circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/>`,
	"file-code":       `<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="m10 13-2 2 2 2"/><path d="m14 17 2-2-2-2"/>`,
	"braces":          `<path d="M8 3H7a2 2 0 0 0-2 2v5a2 2 0 0 1-2 2 2 2 0 0 1 2 2v5c0 1.1.9 2 2 2h1"/><path d="M16 21h1a2 2 0 0 0 2-2v-5c0-1.1.9-2 2-2a2 2 0 0 1-2-2V5a2 2 0 0 0-2-2h-1"/>`,
	"alert":           `<circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/>`,
	"info":            `<circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/>`,
	"circle-check":    `<circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/>`,
	"layers":          `<path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z"/><path d="m22 17.65-9.17 4.16a2 2 0 0 1-1.66 0L2 17.65"/><path d="m22 12.65-9.17 4.16a2 2 0 0 1-1.66 0L2 12.65"/>`,
	"link":            `<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>`,
	"play":            `<polygon points="6 3 20 12 6 21 6 3"/>`,
	"inbox":           `<polyline points="22 12 16 12 14 15 10 15 8 12 2 12"/><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/>`,
	"wrap":            `<line x1="3" x2="21" y1="6" y2="6"/><path d="M3 12h15a3 3 0 1 1 0 6h-4"/><polyline points="16 16 14 18 16 20"/><line x1="3" x2="10" y1="18" y2="18"/>`,
	"align-left":      `<path d="M15 12H3"/><path d="M17 18H3"/><path d="M21 6H3"/>`,
	"route":           `<circle cx="6" cy="19" r="3"/><path d="M9 19h8.5a3.5 3.5 0 0 0 0-7h-11a3.5 3.5 0 0 1 0-7H15"/><circle cx="18" cy="5" r="3"/>`,
	"list":            `<line x1="8" x2="21" y1="6" y2="6"/><line x1="8" x2="21" y1="12" y2="12"/><line x1="8" x2="21" y1="18" y2="18"/><line x1="3" x2="3.01" y1="6" y2="6"/><line x1="3" x2="3.01" y1="12" y2="12"/><line x1="3" x2="3.01" y1="18" y2="18"/>`,
	"rotate":          `<path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/>`,
	"external-link":   `<path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>`,
	"lock":            `<rect width="18" height="11" x="3" y="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>`,
	"server":          `<rect width="20" height="8" x="2" y="2" rx="2" ry="2"/><rect width="20" height="8" x="2" y="14" rx="2" ry="2"/><line x1="6" x2="6.01" y1="6" y2="6"/><line x1="6" x2="6.01" y1="18" y2="18"/>`,
	"terminal":        `<polyline points="4 17 10 11 4 5"/><line x1="12" x2="20" y1="19" y2="19"/>`,
	"circle-dot":      `<circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="1"/>`,
	"shuffle":         `<path d="m18 14 4 4-4 4"/><path d="m18 2 4 4-4 4"/><path d="M2 18h1.973a4 4 0 0 0 3.3-1.7l5.454-8.6a4 4 0 0 1 3.3-1.7H22"/><path d="M2 6h1.972a4 4 0 0 1 3.6 2.2"/><path d="M22 18h-6.041a4 4 0 0 1-3.3-1.8l-.359-.45"/>`,
	"code-xml":        `<path d="m18 16 4-4-4-4"/><path d="m6 8-4 4 4 4"/><path d="m14.5 4-5 16"/>`,
	"filter":          `<polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/>`,
	"clipboard-check": `<rect width="8" height="4" x="8" y="2" rx="1" ry="1"/><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/><path d="m9 14 2 2 4-4"/>`,
}

// IconSVG returns the markup of a Lucide icon, sized by the CSS class.
func IconSVG(name, class string) string {
	body, ok := icons[name]
	if !ok {
		body = icons["info"]
	}
	return `<svg class="icon ` + class + `" viewBox="0 0 24 24" aria-hidden="true" focusable="false">` + body + `</svg>`
}

// MethodClass maps an HTTP method to its badge modifier.
func MethodClass(m domain.HTTPMethod) string {
	switch m {
	case domain.MethodGET, domain.MethodPOST, domain.MethodPUT, domain.MethodPATCH, domain.MethodDELETE:
		return "badge-method--" + strings.ToLower(string(m))
	}
	return "badge-method--other"
}

// StatusClass maps an HTTP status code to its badge modifier (1xx…5xx).
func StatusClass(code int) string {
	c := code / 100
	if c < 1 || c > 5 {
		c = 5
	}
	return "badge-status--" + strconv.Itoa(c) + "xx"
}

// ModeSlug turns MOCKED / RECORD_ONCE into mocked / record-once.
func ModeSlug(mode string) string {
	return strings.ReplaceAll(strings.ToLower(mode), "_", "-")
}

// EnumLabel turns RECORD_ONCE into "Record once".
func EnumLabel(v string) string {
	if v == "" {
		return ""
	}
	s := strings.ToLower(strings.ReplaceAll(v, "_", " "))
	return strings.ToUpper(s[:1]) + s[1:]
}

// ModeOrder is the column order of status counts (SPEC §8.2).
var ModeOrder = []string{"MOCKED", "DISABLED", "FORWARDED", "RECORDING", "RECORD_ONCE", "ECHO"}

// ModeCount is one segment of a mode summary bar.
type ModeCount struct {
	Mode  string
	Count int
}

// ModeCounts orders the non-zero counts of a statusCount map.
func ModeCounts(counts map[string]int) []ModeCount {
	var out []ModeCount
	for _, m := range ModeOrder {
		if n := counts[m]; n > 0 {
			out = append(out, ModeCount{Mode: m, Count: n})
		}
	}
	return out
}

func ModeTotal(counts map[string]int) int {
	t := 0
	for _, n := range counts {
		t += n
	}
	return t
}

// ModeLegend renders "28 mocked · 2 forwarded".
func ModeLegend(counts map[string]int) string {
	parts := []string{}
	for _, mc := range ModeCounts(counts) {
		parts = append(parts, fmt.Sprintf("%d %s", mc.Count, strings.ToLower(EnumLabel(mc.Mode))))
	}
	if len(parts) == 0 {
		return "no endpoints"
	}
	return strings.Join(parts, " · ")
}

func segWidth(n, total int) string {
	if total == 0 {
		return "width:0"
	}
	return fmt.Sprintf("width:%.2f%%", float64(n)*100/float64(total))
}

// RestCounts converts a typed statusCount map for ModeBar.
func RestCounts(m map[domain.RestMethodStatus]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[string(k)] = v
	}
	return out
}

// SoapCounts converts a typed statusCount map for ModeBar.
func SoapCounts(m map[domain.SoapOperationStatus]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[string(k)] = v
	}
	return out
}

// Initials of a user for the avatar.
func Initials(u *domain.User) string {
	if u == nil {
		return "?"
	}
	name := strings.TrimSpace(u.FullName)
	if name == "" {
		name = u.Username
	}
	fields := strings.Fields(name)
	switch {
	case len(fields) >= 2:
		return string([]rune(fields[0])[:1]) + string([]rune(fields[len(fields)-1])[:1])
	case len(name) >= 2:
		return string([]rune(name)[:2])
	}
	return name
}

// FormatTime renders a timestamp in UTC, mono-friendly.
func FormatTime(t domain.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}

// Ago renders a relative time ("2m ago").
func Ago(t domain.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t.Time)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return t.UTC().Format("2006-01-02")
}

var uriParam = regexp.MustCompile(`\{[^}]*\}`)

// URIPart is a literal or a {param} segment of a resource URI.
type URIPart struct {
	Text  string
	Param bool
}

// SplitURI separates the {params} of a URI so they can be highlighted.
func SplitURI(uri string) []URIPart {
	var parts []URIPart
	last := 0
	for _, loc := range uriParam.FindAllStringIndex(uri, -1) {
		if loc[0] > last {
			parts = append(parts, URIPart{Text: uri[last:loc[0]]})
		}
		parts = append(parts, URIPart{Text: uri[loc[0]:loc[1]], Param: true})
		last = loc[1]
	}
	if last < len(uri) {
		parts = append(parts, URIPart{Text: uri[last:]})
	}
	return parts
}

// Itoa is strconv.Itoa for templates.
func Itoa(n int) string { return strconv.Itoa(n) }

// Plural renders "1 resource" / "3 resources".
func Plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// BoolLabel renders an optional flag as Yes / No.
func BoolLabel(b *bool) string {
	if b != nil && *b {
		return "Yes"
	}
	return "No"
}

// OrDash renders "—" for empty values.
func OrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// HeaderValue returns the first value of a header, case-insensitively.
func HeaderValue(headers []domain.HTTPHeader, name string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

// StatusText is the reason phrase used in status badge tooltips.
func StatusText(code int) string {
	if t := statusTexts[code]; t != "" {
		return t
	}
	return "HTTP " + strconv.Itoa(code)
}

var statusTexts = map[int]string{
	100: "Continue", 200: "OK", 201: "Created", 202: "Accepted", 204: "No Content",
	301: "Moved Permanently", 302: "Found", 304: "Not Modified",
	400: "Bad Request", 401: "Unauthorized", 403: "Forbidden", 404: "Not Found", 405: "Method Not Allowed",
	409: "Conflict", 415: "Unsupported Media Type", 422: "Unprocessable Entity", 429: "Too Many Requests",
	500: "Internal Server Error", 501: "Not Implemented", 502: "Bad Gateway", 503: "Service Unavailable", 504: "Gateway Timeout",
}

func joinDot(parts []string) string { return strings.Join(parts, " · ") }

func containsAny(s string, subs ...string) bool {
	s = strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
