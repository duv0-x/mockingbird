// Package match holds the XPath/JSONPath evaluation and ordering helpers
// shared by the REST and SOAP engines and the expression package (SPEC.md §4.5).
//
// Owner: T2. Consumers: T3.
package match

// XPathMatches mirrors XPathUtility.isValidXPathExpr: true when the expression
// selects a non-empty node set (or evaluates to any non-node value) on body.
func XPathMatches(body, expr string) bool { return false }

// XPathValue mirrors XPathUtility.getXPathValue (string() of the result).
func XPathValue(body, expr string) (string, bool) { return "", false }

// JSONPathMatches mirrors JsonPathUtility.isValidJsonPathExpr.
func JSONPathMatches(body, expr string) bool { return false }

// JSONPathValue mirrors JsonPathUtility.getValueWithJsonPathExpr.
func JSONPathValue(body, expr string) (string, bool) { return "", false }

// AlphanumCompare mirrors AlphanumComparator (natural sort of mock response names).
func AlphanumCompare(a, b string) int { return 0 }
