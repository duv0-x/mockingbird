package expression

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/duv0-x/mockingbird/internal/mock/match"
)

// argument is an ExpressionArgument: argString, argNumber, argArray or argMap.
type argument interface{ text() string }

type (
	argString string
	argNumber float64
	argArray  []argument
	argMap    map[string]argument
)

func (a argString) text() string { return string(a) }

// text mirrors Double.toString, which is how CastleMock prints numeric arguments.
func (a argNumber) text() string { return match.FormatJavaDouble(float64(a)) }

func (a argArray) text() string {
	parts := make([]string, len(a))
	for i, v := range a {
		parts[i] = v.text()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func (a argMap) text() string { return fmt.Sprint(map[string]argument(a)) }

// call is a parsed expression (ExpressionInput). Argument names are stored
// lower-cased: ExpressionInput keeps them in a case-insensitive TreeMap.
type call struct {
	name string
	args map[string]argument
}

func (c call) arg(name string) argument { return c.args[strings.ToLower(name)] }

// vocabulary is the set of characters the ANTLR lexer of Expression.g4 knows.
// The generated lexer reports any other character and skips it, so
// "Bogotá" inside a string argument reaches the expression as "Bogot".
func inVocabulary(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	return strings.ContainsRune("_ \t\r\n${}(),=\\\".[]<>|?!@#€%&/+-*^:;'", r)
}

// parse ports ExpressionInputParser.parse over the Expression.g4 grammar:
//
//	expression := '${' NAME ( '(' (arg (',' arg)*)? ')' )? '}'
//	arg        := argName '=' value
//	value      := number | string | array
//	string     := '\'? '"' chars '\'? '"'
//
// Whitespace between tokens is skipped, which is what ANTLR's single-token
// deletion recovers from in the original ("min = 1", "max=2 )").
func parse(source string) (call, error) {
	var b strings.Builder
	for _, r := range source {
		if inVocabulary(r) {
			b.WriteRune(r)
		}
	}
	p := &parser{src: b.String()}
	c, err := p.expression()
	if err != nil {
		return call{}, fmt.Errorf("parse %q: %w", source, err)
	}
	return c, nil
}

type parser struct {
	src string
	pos int
}

var errSyntax = errors.New("syntax error")

func (p *parser) peek() byte {
	if p.pos < len(p.src) {
		return p.src[p.pos]
	}
	return 0
}

func (p *parser) skipSpace() {
	for p.pos < len(p.src) && strings.IndexByte(" \t\r\n", p.src[p.pos]) >= 0 {
		p.pos++
	}
}

func (p *parser) accept(s string) bool {
	p.skipSpace()
	if strings.HasPrefix(p.src[p.pos:], s) {
		p.pos += len(s)
		return true
	}
	return false
}

func (p *parser) expression() (call, error) {
	if !strings.HasPrefix(p.src, "${") {
		return call{}, errSyntax
	}
	p.pos = 2
	start := p.pos
	for p.pos < len(p.src) && (isLetter(p.peek()) || isDigit(p.peek()) || p.peek() == '_') {
		p.pos++
	}
	c := call{name: p.src[start:p.pos], args: map[string]argument{}}
	if c.name == "" {
		return call{}, errSyntax
	}
	if p.accept("(") {
		if !p.accept(")") {
			for {
				name, value, err := p.argument()
				if err != nil {
					return call{}, err
				}
				c.args[strings.ToLower(name)] = value
				if p.accept(",") {
					continue
				}
				// ANTLR conjures a missing ')' right before the closing '}'.
				p.accept(")")
				break
			}
		}
	}
	if !p.accept("}") {
		return call{}, errSyntax
	}
	p.skipSpace()
	if p.pos != len(p.src) {
		return call{}, errSyntax
	}
	return c, nil
}

func (p *parser) argument() (string, argument, error) {
	p.skipSpace()
	start := p.pos
	if !isLetter(p.peek()) {
		return "", nil, errSyntax
	}
	for p.pos < len(p.src) && (isLetter(p.peek()) || isDigit(p.peek())) {
		p.pos++
	}
	name := p.src[start:p.pos]
	if !p.accept("=") {
		return "", nil, errSyntax
	}
	value, err := p.value()
	if err != nil {
		return "", nil, err
	}
	return name, value, nil
}

func (p *parser) value() (argument, error) {
	p.skipSpace()
	switch c := p.peek(); {
	case isDigit(c):
		return p.number()
	case c == '"' || c == '\\':
		return p.string()
	case c == '[':
		return p.array()
	}
	return nil, errSyntax
}

func (p *parser) number() (argument, error) {
	start := p.pos
	for isDigit(p.peek()) {
		p.pos++
	}
	if p.peek() == '.' && p.pos+1 < len(p.src) && isDigit(p.src[p.pos+1]) {
		p.pos++
		for isDigit(p.peek()) {
			p.pos++
		}
	}
	f, err := strconv.ParseFloat(p.src[start:p.pos], 64)
	if err != nil {
		return nil, errSyntax
	}
	return argNumber(f), nil
}

func (p *parser) string() (argument, error) {
	if p.peek() == '\\' {
		p.pos++
	}
	if p.peek() != '"' {
		return nil, errSyntax
	}
	p.pos++
	start := p.pos
	for p.pos < len(p.src) && p.src[p.pos] != '"' && p.src[p.pos] != '\\' {
		// "${" is a single token that the string rule does not accept.
		if strings.HasPrefix(p.src[p.pos:], "${") {
			return nil, errSyntax
		}
		p.pos++
	}
	value := p.src[start:p.pos]
	if p.peek() == '\\' {
		p.pos++
	}
	if p.peek() != '"' {
		return nil, errSyntax
	}
	p.pos++
	return argString(value), nil
}

func (p *parser) array() (argument, error) {
	p.pos++
	arr := argArray{}
	if p.accept("]") {
		return arr, nil
	}
	for {
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		arr = append(arr, v)
		if p.accept(",") {
			continue
		}
		if !p.accept("]") {
			return nil, errSyntax
		}
		return arr, nil
	}
}

func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }

// Format renders an expression the way ExpressionInputParser.convert does; the
// REST definition importer uses it to put expressions in generated bodies.
func Format(name string, args ...FormatArg) string {
	var b strings.Builder
	b.WriteString("${")
	b.WriteString(name)
	b.WriteByte('(')
	for i, a := range args {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(a.Name)
		b.WriteByte('=')
		writeFormatValue(&b, a.Value)
	}
	b.WriteString(")}")
	return b.String()
}

// FormatArg is one argument of Format. Value is a string, a float64 or a
// []any of those.
type FormatArg struct {
	Name  string
	Value any
}

func writeFormatValue(b *strings.Builder, v any) {
	switch t := v.(type) {
	case string:
		b.WriteByte('"')
		b.WriteString(t)
		b.WriteByte('"')
	case float64:
		b.WriteString(match.FormatJavaDouble(t))
	case []any:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			writeFormatValue(b, e)
		}
		b.WriteByte(']')
	}
}
