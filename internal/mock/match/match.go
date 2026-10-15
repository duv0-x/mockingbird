// Package match holds the XPath/JSONPath evaluation and ordering helpers
// shared by the REST and SOAP engines and the expression package (SPEC.md §4.5).
//
// Owner: T2. Consumers: T3.
package match

// XPathMatches mirrors XPathUtility.isValidXPathExpr: true when the expression
// selects a non-empty node set (or evaluates to any non-node value) on body.
// Invalid XML or an expression that does not compile → false.
func XPathMatches(body, expr string) bool { return xpathMatches(body, expr) }

// XPathValue mirrors XPathUtility.getXPathValue (string() of the result). ok
// is false when the body is not XML or the expression is invalid.
func XPathValue(body, expr string) (string, bool) { return xpathValue(body, expr) }

// JSONPathMatches mirrors JsonPathUtility.isValidJsonPathExpr: an indefinite
// path matches when it selects at least one value, a definite path when it
// resolves to a non-null value (D-8). Invalid JSON or expression → false.
func JSONPathMatches(body, expr string) bool { return jsonPathMatches(body, expr) }

// JSONPathValue mirrors JsonPathUtility.getValueWithJsonPathExpr with
// SUPPRESS_EXCEPTIONS: a single selected element is rendered on its own, several
// as a JSON array, scalars with Java's toString. ok is false when nothing is selected.
func JSONPathValue(body, expr string) (string, bool) { return jsonPathValue(body, expr) }

// AlphanumCompare mirrors AlphanumComparator (natural sort of mock response names).
func AlphanumCompare(a, b string) int { return alphanumCompare(a, b) }
