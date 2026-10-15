package match

import (
	"slices"
	"testing"
)

const storeBody = `{
  "store": {
    "book": [
      {"category": "reference", "author": "Author 1", "title": "Book title 1", "price": 8.95},
      {"category": "fiction", "author": "Author 2", "title": "Book title 2", "isbn": "1-553-21311-3", "price": 8.99},
      {"category": "fiction", "author": "Author 3", "title": "Book title 3", "isbn": "1-395-19395-8", "price": 22.99}
    ],
    "bicycle": {"color": "red", "price": 19.95}
  },
  "expensive": 10
}`

// Cases from JsonPathUtilityTest.
func TestJSONPathMatches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		expr string
		want bool
	}{
		{"filter lower than", storeBody, "$.store.book[?(@.price < 10)]", true},
		{"filter equals", storeBody, "$.store.book[?(@.author == 'Author 1')]", true},
		{"filter no match", storeBody, "$.store.book[?(@.author == 'Karl Dahlgren')]", false},
		{"filter on root object", storeBody, "$.[?(@.expensive == 10)]", true},
		{"filter on root object no match", storeBody, "$.[?(@.expensive == 11)]", false},
		{"definite scalar (D-8)", storeBody, "$.expensive", true},
		{"definite object (D-8)", storeBody, "$.store.bicycle", true},
		{"definite missing", storeBody, "$.store.car", false},
		{"definite null", `{"a": null}`, "$.a", false},
		{"definite empty array", `{"a": []}`, "$.a", false},
		{"indefinite empty", storeBody, "$..car", false},
		{"deep scan", storeBody, "$..isbn", true},
		{"invalid json", "<xml/>", "$.a", false},
		{"invalid expression", storeBody, "$.store[?(", false},
		{"function", storeBody, "$.store.book.length()", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := JSONPathMatches(tt.body, tt.expr); got != tt.want {
				t.Errorf("JSONPathMatches(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestJSONPathValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		body   string
		expr   string
		want   string
		wantOK bool
	}{
		{"single filtered value", storeBody, "$.store.book[?(@.price > 20)].author", "Author 3", true},
		{"list of objects keeps key order", storeBody, "$.store.book[*]",
			`[{"category":"reference","author":"Author 1","title":"Book title 1","price":8.95},` +
				`{"category":"fiction","author":"Author 2","title":"Book title 2","isbn":"1-553-21311-3","price":8.99},` +
				`{"category":"fiction","author":"Author 3","title":"Book title 3","isbn":"1-395-19395-8","price":22.99}]`, true},
		{"list of strings", storeBody, "$.store.book[?(@.price > 1)].author", `["Author 1","Author 2","Author 3"]`, true},
		{"definite property", `{"definite": "value"}`, "$.definite", "value", true},
		{"length function", storeBody, "$.store.book.length()", "3", true},
		{"index", storeBody, "$.store.book[0].title", "Book title 1", true},
		{"negative index", storeBody, "$.store.book[-1].title", "Book title 3", true},
		{"slice", storeBody, "$.store.book[0:2].author", `["Author 1","Author 2"]`, true},
		{"union", storeBody, "$.store.bicycle['color','price']", `["red",19.95]`, true},
		{"integer", storeBody, "$.expensive", "10", true},
		{"double", storeBody, "$.store.bicycle.price", "19.95", true},
		{"double without fraction", `{"a": 10.0}`, "$.a", "10.0", true},
		{"object", storeBody, "$.store.bicycle", `{"color":"red","price":19.95}`, true},
		{"slash escaped like json-smart", `{"a": ["x/y", "z"]}`, "$.a", `["x\/y","z"]`, true},
		{"path without root", storeBody, "store.bicycle.color", "red", true},
		{"missing", storeBody, "$.nothing", "", false},
		{"empty selection", storeBody, "$..nothing", "", false},
		{"invalid json", "nope", "$.a", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := JSONPathValue(tt.body, tt.expr)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("JSONPathValue(%q) = %q, %v; want %q, %v", tt.expr, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

const whoIsResponse = `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema">
   <soap:Body>
      <GetWhoISResponse xmlns="http://www.webservicex.net">
         <GetWhoISResult>google.com is a good domain</GetWhoISResult>
      </GetWhoISResponse>
   </soap:Body>
</soap:Envelope>`

const getUser = `<soapenv:Envelope xmlns:cm="http://castlemock.com" xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:web="http://schemas.xmlsoap.org/wsdl/">
   <soapenv:Body>
      <cm:getUser>
         <username>admin</username>
         <email>admin@castlemock.com</email>
         <permissions>
            <permission name="READ">The user can read</permission>
            <permission name="WRITE">The user can write</permission>
            <permission name="DELETE">The user can delete</permission>
            <permission name="MODIFY">The user can modify</permission>
         </permissions>
      </cm:getUser>
   </soapenv:Body>
</soapenv:Envelope>`

const entries = `<entries>
     <entry>
        <value>10.50</value>
     </entry>
     <entry>
        <value>1.50</value>
     </entry>
      <entry>
        <value>10</value>
     </entry>
</entries>`

const attrEntries = `<entries>
    <entry key="mykey1" attr="attr1"/>
    <entry key="mykey2" attr="attr2"/>
    <otherentry key="mykey1" attr="attr3"/>
    <entry key="mykey4"/>
    <otherentry key="mykey4"/>
</entries>`

func whoIs(host string) string {
	return `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:web="http://www.webservicex.net">
   <soapenv:Header/>
   <soapenv:Body>
      <web:GetWhoIS>
         <!--Optional:-->
         <web:HostName>` + host + `</web:HostName>
      </web:GetWhoIS>
   </soapenv:Body>
</soapenv:Envelope>`
}

// Cases from XPathUtilityTest, plus the non namespace aware quirks.
func TestXPathMatches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		expr string
		want bool
	}{
		{"default namespace", whoIsResponse, "//GetWhoISResponse/GetWhoISResult[text()='google.com is a good domain']", true},
		{"number result", entries, "sum(//value)", true},
		{"attribute", getUser, "//getUser/permissions/permission[@name='READ'][text()='The user can read']", true},
		{"attribute multiple expression", getUser, "//getUser/permissions/permission[@name='READ'][text()='The user can read' and //getUser/permissions/permission[@name='WRITE'][text()='The user can write']]", true},
		{"attribute exists", attrEntries, "//entry[@key]", true},
		{"number compare", "<entries>\n\t<entry>\n\t\t1234\n\t</entry>\n</entries>", "//entry[text() = 1234]", true},
		{"prefixed element by local name", whoIs("castlemock.com"), "//GetWhoIS/HostName[text() = 'castlemock.com']", true},
		{"prefixed name test never resolves", whoIs("castlemock.com"), "//web:GetWhoIS", false},
		{"no match", whoIs("castlemock.com"), "//GetWhoIS/HostName[text() = 'other']", false},
		{"boolean result", whoIs("castlemock.com"), "count(//HostName) = 5", true},
		{"invalid xml", `{"a": 1}`, "//a", false},
		{"empty body", "", "//a", false},
		{"invalid expression", whoIs("x"), "//[", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := XPathMatches(tt.body, tt.expr); got != tt.want {
				t.Errorf("XPathMatches(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestXPathValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		body   string
		expr   string
		want   string
		wantOK bool
	}{
		{"text", whoIs("castlemock.com"), "//GetWhoIS/HostName/text()", "castlemock.com", true},
		{"substring", whoIs("castlemock.com"), "substring(//GetWhoIS/HostName/text(), 12, 3)", "com", true},
		{"attribute", attrEntries, "//entry/@key", "mykey1", true},
		{"quotation", whoIs("Before-After"), "substring-after(//GetWhoIS/HostName/text(), '-')", "After", true},
		{"sum", entries, "sum(//value)", "22", true},
		{"fraction", entries, "sum(//value) div 4", "5.5", true},
		{"boolean", entries, "count(//value) = 3", "true", true},
		{"no node", entries, "//missing", "", true},
		{"invalid xml", "nope", "//a", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := XPathValue(tt.body, tt.expr)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("XPathValue(%q) = %q, %v; want %q, %v", tt.expr, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestAlphanumCompare(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"AlphanumComparatorTest", []string{"a100", "a1", "a10", "a11", "a101"}, []string{"a1", "a10", "a11", "a100", "a101"}},
		{"mixed", []string{"Response 10", "Response 2", "Recorded", "Response 1"}, []string{"Recorded", "Response 1", "Response 2", "Response 10"}},
		{"leading zeros compare by length", []string{"a01", "a1", "a002"}, []string{"a1", "a01", "a002"}},
		{"prefix", []string{"ab", "a", "abc"}, []string{"a", "ab", "abc"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := slices.Clone(tt.input)
			slices.SortStableFunc(got, AlphanumCompare)
			if !slices.Equal(got, tt.want) {
				t.Errorf("sorted = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatJavaDouble(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   float64
		want string
	}{
		{42, "42.0"}, {999.9, "999.9"}, {9.07231552e8, "9.07231552E8"}, {1.971414777e9, "1.971414777E9"},
		{1e7, "1.0E7"}, {9999999, "9999999.0"}, {0.001, "0.001"}, {0.0001, "1.0E-4"}, {-2.5, "-2.5"}, {0, "0.0"},
	}
	for _, tt := range tests {
		if got := FormatJavaDouble(tt.in); got != tt.want {
			t.Errorf("FormatJavaDouble(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if got := FormatJavaFloat(0.77921385); got != "0.77921385" {
		t.Errorf("FormatJavaFloat = %q", got)
	}
}
