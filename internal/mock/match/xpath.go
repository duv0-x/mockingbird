package match

import (
	"math"
	"strconv"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/antchfx/xpath"
)

// parseXML mirrors the default (non namespace aware) DocumentBuilderFactory
// used by XPathUtility. On such a DOM, Xalan matches an unprefixed name test
// against the part of the qualified name after the colon and cannot resolve
// prefixed name tests at all, so "//GetWhoIS" selects <web:GetWhoIS> while
// "//web:GetWhoIS" never matches. Dropping prefixes and namespace URIs from
// the parsed tree reproduces exactly that.
func parseXML(body string) (*xmlquery.Node, bool) {
	doc, err := xmlquery.Parse(strings.NewReader(body))
	if err != nil || !wellFormed(doc) {
		return nil, false
	}
	stripNamespaces(doc)
	return doc, true
}

// wellFormed rejects what a SAX parser would: no root element, several root
// elements or text outside the root.
func wellFormed(doc *xmlquery.Node) bool {
	roots := 0
	for n := doc.FirstChild; n != nil; n = n.NextSibling {
		switch n.Type {
		case xmlquery.ElementNode:
			roots++
		case xmlquery.TextNode, xmlquery.CharDataNode:
			if strings.TrimSpace(n.Data) != "" {
				return false
			}
		}
	}
	return roots == 1
}

func stripNamespaces(n *xmlquery.Node) {
	for ; n != nil; n = n.NextSibling {
		if n.Type == xmlquery.ElementNode {
			n.Prefix, n.NamespaceURI = "", ""
			attrs := n.Attr[:0]
			for _, a := range n.Attr {
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
					continue
				}
				a.Name.Space, a.NamespaceURI = "", ""
				attrs = append(attrs, a)
			}
			n.Attr = attrs
		}
		stripNamespaces(n.FirstChild)
	}
}

func evaluateXPath(body, expr string) (any, bool) {
	doc, ok := parseXML(body)
	if !ok {
		return nil, false
	}
	compiled, err := xpath.Compile(expr)
	if err != nil {
		return nil, false
	}
	return evaluateSafely(compiled, doc)
}

// evaluateSafely guards against panics inside the XPath library on odd
// expressions: a failed evaluation is a non-match, like a Java exception.
func evaluateSafely(compiled *xpath.Expr, doc *xmlquery.Node) (result any, ok bool) {
	defer func() {
		if recover() != nil {
			result, ok = nil, false
		}
	}()
	return compiled.Evaluate(xmlquery.CreateXPathNavigator(doc)), true
}

func xpathMatches(body, expr string) bool {
	result, ok := evaluateXPath(body, expr)
	if !ok {
		return false
	}
	if it, isNodes := result.(*xpath.NodeIterator); isNodes {
		return it.MoveNext()
	}
	return true
}

func xpathValue(body, expr string) (string, bool) {
	result, ok := evaluateXPath(body, expr)
	if !ok {
		return "", false
	}
	switch v := result.(type) {
	case *xpath.NodeIterator:
		if v.MoveNext() {
			return v.Current().Value(), true
		}
		return "", true
	case float64:
		return xpathNumber(v), true
	case bool:
		return strconv.FormatBool(v), true
	case string:
		return v, true
	}
	return "", false
}

// xpathNumber is the XPath 1.0 number-to-string conversion (Xalan XNumber.str).
func xpathNumber(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == math.Trunc(f) && math.Abs(f) < 1e18:
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
