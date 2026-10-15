package expression

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// sequence returns a function producing values in order, like the Mockito
// mocks of TextParseMockConfig.
func sequence(values ...string) function {
	i := 0
	return func(call) (string, error) {
		v := values[min(i, len(values)-1)]
		i++
		return v, nil
	}
}

// mockedRegistry mirrors TextParseMockConfig.
func mockedRegistry() map[string]function {
	return map[string]function{
		"RANDOM_INTEGER":   sequence("1", "2", "3"),
		"RANDOM_DOUBLE":    sequence("9.07231552E8", "1.971414777E9"),
		"RANDOM_LONG":      sequence("8345852683487585923", "4396544029006714971"),
		"RANDOM_FLOAT":     sequence("0.77921385", "0.52562904"),
		"RANDOM_BOOLEAN":   sequence("false", "true"),
		"RANDOM_DATE":      sequence("2019-01-07", "2020-10-15"),
		"RANDOM_STRING":    sequence("XfDbd6sk90hxH0L", "s80ho4iJTlda"),
		"RANDOM_UUID":      sequence("bf60cb57-cfd8-43bb-83f7-62b1584c29d7", "af88f1d8-c8f6-4407-903c-18ffe6ac06c5"),
		"RANDOM_EMAIL":     sequence("Cng73JRbJT@RliQvDuVVN8.com", "C0GlhHZ4vL@6ZNPRPmX.com"),
		"RANDOM_PASSWORD":  sequence("j5HC7UGSVp", "p70ggj9409ia"),
		"RANDOM_DATE_TIME": sequence("2008-02-09T21:42:10", "2048-09-26T06:39:22"),
		"RANDOM_ENUM":      sequence("available", "Z"),
		"FAKER":            sequence("Peter", "Paul"),
		"PATH_PARAMETER":   pathParameter,
		"QUERY_STRING":     queryString,
		"BODY_JSON_PATH":   bodyJSONPath,
		"URL_HOST":         urlHost,
		"BODY_XPATH":       bodyXPath,
	}
}

// Cases from TextParserTest that use the mocked expressions.
func TestRenderMocked(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"integer", "a: ${RANDOM_INTEGER}, b: ${RANDOM_INTEGER()}.", "a: 1, b: 2."},
		{"double", "a: ${RANDOM_DOUBLE}, b: ${RANDOM_DOUBLE}.", "a: 9.07231552E8, b: 1.971414777E9."},
		{"long", "a: ${RANDOM_LONG}, b: ${RANDOM_LONG}.", "a: 8345852683487585923, b: 4396544029006714971."},
		{"float", "a: ${RANDOM_FLOAT}, b: ${RANDOM_FLOAT}.", "a: 0.77921385, b: 0.52562904."},
		{"boolean", "a: ${RANDOM_BOOLEAN}, b: ${RANDOM_BOOLEAN}.", "a: false, b: true."},
		{"date", "a: ${RANDOM_DATE}, b: ${RANDOM_DATE}.", "a: 2019-01-07, b: 2020-10-15."},
		{"string", "a: ${RANDOM_STRING}, b: ${RANDOM_STRING}.", "a: XfDbd6sk90hxH0L, b: s80ho4iJTlda."},
		{"uuid", "a: ${RANDOM_UUID}, b: ${RANDOM_UUID}.", "a: bf60cb57-cfd8-43bb-83f7-62b1584c29d7, b: af88f1d8-c8f6-4407-903c-18ffe6ac06c5."},
		{"email", "a: ${RANDOM_EMAIL}, b: ${RANDOM_EMAIL}.", "a: Cng73JRbJT@RliQvDuVVN8.com, b: C0GlhHZ4vL@6ZNPRPmX.com."},
		{"password", "a: ${RANDOM_PASSWORD}, b: ${RANDOM_PASSWORD}.", "a: j5HC7UGSVp, b: p70ggj9409ia."},
		{"date time", "a: ${RANDOM_DATE_TIME}, b: ${RANDOM_DATE_TIME}.", "a: 2008-02-09T21:42:10, b: 2048-09-26T06:39:22."},
		{"enum", `a: ${RANDOM_ENUM(values=["available","pending","sold"])}, b: ${RANDOM_ENUM(values=["X","Y","Z"])}.`, "a: available, b: Z."},
		{"faker", `a: ${FAKER(api="name().fullName()")}, b: ${FAKER(api="name().fullName()")}.`, "a: Peter, b: Paul."},
		{"multiple and unknown", "a: ${RANDOM_INTEGER}, b: ${RANDOM_INTEGER()}, c: ${RANDOM_DATE()}, d: ${RANDOM_INTEGER(min=0,max=100)}, e: ${RANDOM_DATE()}, f: ${X}.${}",
			"a: 1, b: 2, c: 2019-01-07, d: 3, e: 2020-10-15, f: ${X}.${}"},
		{"no closing brace", "a ${RANDOM_INTEGER", "a ${RANDOM_INTEGER"},
		{"syntax error kept", "${RANDOM_INTEGER(min=)}", "${RANDOM_INTEGER(min=)}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := render(tt.input, Input{}, mockedRegistry()); got != tt.want {
				t.Errorf("render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func ptr(s string) *string { return &s }

// Cases from TextParserTest that use the real expressions with external input.
func TestRenderExternalInput(t *testing.T) {
	t.Parallel()
	storeBook, err := os.ReadFile("testdata/store-book.json")
	if err != nil {
		t.Fatal(err)
	}
	queries := []domain.HTTPParameter{
		{Name: "queryParam1", Value: "Apple"}, {Name: "queryParam2", Value: "Orange"},
		{Name: "queryParam3", Value: "Banana"}, {Name: "queryParam4", Value: "Papaya"},
	}
	tests := []struct {
		name  string
		input string
		in    Input
		want  string
	}{
		{"path parameter", `a: ${PATH_PARAMETER(parameter="param1")}, b: ${PATH_PARAMETER(parameter="param2")}.`,
			Input{PathParameters: map[string][]string{"param1": {"X"}, "param2": {"Y"}, "param3": {"Z"}}}, "a: X, b: Y."},
		{"path parameter multiple values", `${PATH_PARAMETER(parameter="param1")}`,
			Input{PathParameters: map[string][]string{"param1": {"Y", "X"}}}, "X"},
		{"path parameter missing", `[${PATH_PARAMETER(parameter="nope")}]`,
			Input{PathParameters: map[string][]string{}}, "[]"},
		{"path parameter without input", `[${PATH_PARAMETER(parameter="param1")}]`, Input{}, "[]"},
		{"query string", `a: ${QUERY_STRING(query="queryParam1")}, b: ${QUERY_STRING(query="queryParam2")}.`,
			Input{QueryParameters: queries}, "a: Apple, b: Orange."},
		{"body json path", `a: ${BODY_JSON_PATH(expression="$.store.book[0].title")}, b: ${BODY_JSON_PATH(expression="$.store.book[1].title")}.`,
			Input{Body: ptr(string(storeBook))}, "a: Book title 1, b: Book title 2."},
		{"url host", "a: ${URL_HOST()}, b: ${URL_HOST()}.", Input{URL: ptr("http://localhost:8080/castlemock")}, "a: localhost, b: localhost."},
		{"url host missing", "[${URL_HOST()}]", Input{}, "[]"},
		{"body xpath", `a: ${BODY_XPATH(expression="//user/id/text()")}, b: ${BODY_XPATH(expression="//user/name/text()")}.`,
			Input{Body: ptr("<user>\n   <id>1</id>\n   <name>Peter</name>\n</user>\n")}, "a: 1, b: Peter."},
		{"body xpath with quotes", `${BODY_XPATH(expression="substring-after(//GetWhoIS/HostName/text(), '-')")}`,
			Input{Body: ptr("<GetWhoIS><HostName>Before-After</HostName></GetWhoIS>")}, "After"},
		{"body xpath without body", `[${BODY_XPATH(expression="//a")}]`, Input{}, "[]"},
		{"external argument overrides user argument", `${BODY_XPATH(expression="//a/text()", body="<a>user</a>")}`,
			Input{Body: ptr("<a>request</a>")}, "request"},
		{"faker unsupported", `[${FAKER(api="notExisting().notExisting()")}]`, Input{}, "[]"},
		{"enum without values is kept", "${RANDOM_ENUM}", Input{}, "${RANDOM_ENUM}"},
		{"enum with non array is kept", `${RANDOM_ENUM(values="x")}`, Input{}, `${RANDOM_ENUM(values="x")}`},
		{"integer with zero max is kept", "${RANDOM_INTEGER(max=0)}", Input{}, "${RANDOM_INTEGER(max=0)}"},
		{"enum with numbers", "${RANDOM_ENUM(values=[2.5])}", Input{}, "2.5"},
		{"enum with integer number", "${RANDOM_ENUM(values=[7])}", Input{}, "7.0"},
		{"lexer drops unknown characters", `${RANDOM_ENUM(values=["Bogotá"])}`, Input{}, "Bogot"},
		{"whitespace tolerated", `${RANDOM_ENUM( values = [ "a" ] )}`, Input{}, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Render(tt.input, tt.in); got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderRandomShapes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input   string
		pattern string
	}{
		{"${RANDOM_INTEGER}", `^-?\d+$`},
		{"${RANDOM_INTEGER(min=5,max=1)}", `^5$`},
		{"${RANDOM_DOUBLE(min=1,max=1)}", `^1\.0$`},
		{"${RANDOM_DOUBLE}", `^\d+(\.\d+)?(E\d+)?$`},
		{"${RANDOM_LONG}", `^-?\d+$`},
		{"${RANDOM_FLOAT}", `^\d\.\d+(E-\d+)?$`},
		{"${RANDOM_PASSWORD}", `^\d\.\d+(E-\d+)?$`},
		{"${RANDOM_BOOLEAN}", `^(true|false)$`},
		{"${RANDOM_DATE}", `^\d{4}-\d{2}-\d{2}$`},
		{"${RANDOM_DATE_TIME}", `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$`},
		{"${RANDOM_STRING}", `^[A-Za-z0-9]{5,19}$`},
		{"${RANDOM_UUID}", `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`},
		{`${RANDOM_EMAIL(domain="castlemock",min=5,max=10,topDomain="com")}`, `^[A-Za-z0-9]{5,14}@castlemock\.com$`},
		{"${RANDOM_EMAIL}", `^[A-Za-z0-9]{5,14}@[A-Za-z0-9]{5,14}\.com$`},
		{"${RANDOM_DECIMAL}", `^\$\{RANDOM_DECIMAL\}$`},
		{`${FAKER(api="name().fullName()")}`, `^\S+ \S+$`},
		{`${FAKER(api="name.firstName", locale="pt-BR")}`, `^\S+$`},
		{`${FAKER(api="number().numberBetween(1, 2)")}`, `^1$`},
		{`${FAKER(api="number().numberBetween(2147483647000L, 2147483647001L)")}`, `^2147483647000$`},
		{`${FAKER(api="address().streetAddress(true)")}`, `.+`},
		{`${FAKER(api="address().zipCodeByState('FL')")}`, `.+`},
		{`${FAKER(api="commerce().price(10.5, 1000.29)")}`, `^\d+\.\d{2}$`},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := Render(tt.input, Input{})
			if !regexp.MustCompile(tt.pattern).MatchString(got) {
				t.Errorf("Render(%q) = %q, want match %s", tt.input, got, tt.pattern)
			}
		})
	}
}

func TestRandomDateRange(t *testing.T) {
	t.Parallel()
	for range 50 {
		got := Render("${RANDOM_DATE_TIME}", Input{})
		parsed, err := time.ParseInLocation("2006-01-02T15:04:05", got, time.Local)
		if err != nil {
			t.Fatalf("parse %q: %v", got, err)
		}
		if d := time.Since(parsed); d > 1e12*time.Millisecond+time.Hour || d < -1e12*time.Millisecond-time.Hour {
			t.Fatalf("%q outside now ± 10^12 ms", got)
		}
	}
}

// Cases from ExpressionInputParserTest and ExpressionParserTest.
func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want string
		args map[string]string
	}{
		{"no arguments", "${RANDOM_ENUM}", "RANDOM_ENUM", map[string]string{}},
		{"empty arguments", "${RANDOM_ENUM()}", "RANDOM_ENUM", map[string]string{}},
		{"string", `${RANDOM_ENUM(Key="Value")}`, "RANDOM_ENUM", map[string]string{"key": "Value"}},
		{"string with backslashes", `${RANDOM_BOOLEAN(Key=\"Value\")}`, "RANDOM_BOOLEAN", map[string]string{"key": "Value"}},
		{"string with spaces", `${RANDOM_BOOLEAN(Key="Good day to you.")}`, "RANDOM_BOOLEAN", map[string]string{"key": "Good day to you."}},
		{"string with number", `${RANDOM_BOOLEAN(Key="Value123")}`, "RANDOM_BOOLEAN", map[string]string{"key": "Value123"}},
		{"number", "${RANDOM_INTEGER(min=123)}", "RANDOM_INTEGER", map[string]string{"min": "123.0"}},
		{"array", `${RANDOM_ENUM(values=["X","Y","Z"])}`, "RANDOM_ENUM", map[string]string{"values": "[X, Y, Z]"}},
		{"array with backslashes", `${RANDOM_BOOLEAN(Key=[\"ValueA\",\"ValueB\",\"ValueC\"])}`, "RANDOM_BOOLEAN", map[string]string{"key": "[ValueA, ValueB, ValueC]"}},
		{"array of numbers", "${RANDOM_BOOLEAN(Key=[1,2,3])}", "RANDOM_BOOLEAN", map[string]string{"key": "[1.0, 2.0, 3.0]"}},
		{"mixed", `${RANDOM_ENUM(value1="This is a value", value2=999.9, value3=["X","Y","Z"])}`, "RANDOM_ENUM",
			map[string]string{"value1": "This is a value", "value2": "999.9", "value3": "[X, Y, Z]"}},
		{"multiple with spaces", `${RANDOM_BOOLEAN( Key1="Value1", Key2="Value2")}`, "RANDOM_BOOLEAN", map[string]string{"key1": "Value1", "key2": "Value2"}},
		{"quotation marks", `${BODY_XPATH(expression="substring-after(//GetWhoIS/HostName/text(), 'V'")}`, "BODY_XPATH",
			map[string]string{"expression": "substring-after(//GetWhoIS/HostName/text(), 'V'"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := parse(tt.src)
			if err != nil {
				t.Fatalf("parse(%q): %v", tt.src, err)
			}
			if c.name != tt.want {
				t.Errorf("name = %q, want %q", c.name, tt.want)
			}
			got := map[string]string{}
			for k, v := range c.args {
				got[k] = v.text()
			}
			if len(got) != len(tt.args) {
				t.Fatalf("args = %v, want %v", got, tt.args)
			}
			for k, v := range tt.args {
				if got[k] != v {
					t.Errorf("arg %s = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	for _, src := range []string{"${}", "${A(x)}", "${A(1=2)}", "${A(x=)}", `${A(x="a)}`, "${A(x=[1,)}", "${A B}", "$A}"} {
		if _, err := parse(src); err == nil {
			t.Errorf("parse(%q) succeeded, want error", src)
		}
	}
}

// Cases from ExpressionInputParserTest.testConvert*.
func TestFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		got  string
		want string
	}{
		{Format("RANDOM_INTEGER"), "${RANDOM_INTEGER()}"},
		{Format("RANDOM_INTEGER", FormatArg{"Key", "Value"}), `${RANDOM_INTEGER(Key="Value")}`},
		{Format("RANDOM_INTEGER", FormatArg{"min", 1000.0}), "${RANDOM_INTEGER(min=1000.0)}"},
		{Format("RANDOM_ENUM", FormatArg{"Array", []any{"X", "Y", "Z"}}), `${RANDOM_ENUM(Array=["X","Y","Z"])}`},
		{Format("RANDOM_ENUM", FormatArg{"Key1", "Value"}, FormatArg{"Key2", 1000.0}, FormatArg{"Key3", []any{"X", "Y", "Z"}}),
			`${RANDOM_ENUM(Key1="Value",Key2=1000.0,Key3=["X","Y","Z"])}`},
	}
	for i, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("case %d: got %q, want %q", i, tt.got, tt.want)
		}
	}
}

// ExternalInputBuilderTest.
func TestInputArguments(t *testing.T) {
	t.Parallel()
	in := Input{
		Body:            ptr("Body content"),
		URL:             ptr("http://localhost:8080/castlemock"),
		PathParameters:  map[string][]string{"param1": {"X"}},
		QueryParameters: []domain.HTTPParameter{{Name: "queryParam1", Value: "Apple"}},
	}
	args := in.arguments()
	if got := args[argBody].text(); got != "Body content" {
		t.Errorf("BODY = %q", got)
	}
	if got := args[argURL].text(); got != "http://localhost:8080/castlemock" {
		t.Errorf("URL_ARGUMENT = %q", got)
	}
	if got := args[argPathParameters].(argMap)["param1"].text(); got != "[X]" {
		t.Errorf("PATH_PARAMETERS = %q", got)
	}
	if got := args[argQueryStrings].(argMap)["queryParam1"].text(); got != "Apple" {
		t.Errorf("QUERY_STRINGS = %q", got)
	}
	if len((Input{}).arguments()) != 0 {
		t.Error("empty input must not add arguments")
	}
}

// testParseBigResponse: every expression of a large template is replaced.
func TestRenderBigResponse(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("<users>\n")
	for i := range 200 {
		b.WriteString("   <user>\n      <sequence>" + strconv.Itoa(i+1) + "</sequence>\n")
		b.WriteString("      <id>${RANDOM_LONG()}</id>\n      <name>${RANDOM_STRING()}</name>\n")
		b.WriteString(`      <email>${RANDOM_EMAIL(domain="castlemock",min=5,max=10,topDomain="com")}</email>` + "\n")
		b.WriteString(`      <status>${RANDOM_ENUM(values=["active","inactive","locked"])}</status>` + "\n   </user>\n")
	}
	b.WriteString("</users>\n")
	if got := Render(b.String(), Input{}); strings.Contains(got, "${") {
		t.Errorf("unrendered expression left in output")
	}
}
