package expression

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/brianvoe/gofakeit/v7"

	"github.com/duv0-x/mockingbird/internal/mock/match"
)

// fakerAPI maps JavaFaker "category.method" paths (lower-cased) to gofakeit.
// CastleMock evaluates the api argument as a SpEL expression on a JavaFaker
// instance (FakerExpression), so "name().fullName()" and "name.fullName" both
// work there; both spellings are accepted here. Methods taking integer
// arguments receive them in args. The locale argument is ignored: gofakeit
// only generates English data.
var fakerAPI = map[string]func(f *gofakeit.Faker, args fakerArgs) string{
	"name.fullname": func(f *gofakeit.Faker, _ fakerArgs) string { return f.FirstName() + " " + f.LastName() },
	"name.name":     func(f *gofakeit.Faker, _ fakerArgs) string { return f.FirstName() + " " + f.LastName() },
	"name.namewithmiddle": func(f *gofakeit.Faker, _ fakerArgs) string {
		return f.FirstName() + " " + f.FirstName() + " " + f.LastName()
	},
	"name.firstname": func(f *gofakeit.Faker, _ fakerArgs) string { return f.FirstName() },
	"name.lastname":  func(f *gofakeit.Faker, _ fakerArgs) string { return f.LastName() },
	"name.prefix":    func(f *gofakeit.Faker, _ fakerArgs) string { return f.NamePrefix() },
	"name.suffix":    func(f *gofakeit.Faker, _ fakerArgs) string { return f.NameSuffix() },
	"name.title":     func(f *gofakeit.Faker, _ fakerArgs) string { return f.JobTitle() },
	"name.username":  func(f *gofakeit.Faker, _ fakerArgs) string { return f.Username() },

	"address.fulladdress": func(f *gofakeit.Faker, _ fakerArgs) string { return f.Address().Address },
	"address.streetaddress": func(f *gofakeit.Faker, args fakerArgs) string {
		if args.bool(0) {
			return f.Street() + " Apt. " + strconv.Itoa(f.Number(100, 999))
		}
		return f.Street()
	},
	"address.streetname":       func(f *gofakeit.Faker, _ fakerArgs) string { return f.StreetName() },
	"address.streetsuffix":     func(f *gofakeit.Faker, _ fakerArgs) string { return f.StreetSuffix() },
	"address.buildingnumber":   func(f *gofakeit.Faker, _ fakerArgs) string { return f.StreetNumber() },
	"address.secondaryaddress": func(f *gofakeit.Faker, _ fakerArgs) string { return "Apt. " + strconv.Itoa(f.Number(100, 999)) },
	"address.city":             func(f *gofakeit.Faker, _ fakerArgs) string { return f.City() },
	"address.cityname":         func(f *gofakeit.Faker, _ fakerArgs) string { return f.City() },
	"address.state":            func(f *gofakeit.Faker, _ fakerArgs) string { return f.State() },
	"address.stateabbr":        func(f *gofakeit.Faker, _ fakerArgs) string { return f.StateAbr() },
	"address.zipcode":          func(f *gofakeit.Faker, _ fakerArgs) string { return f.Zip() },
	"address.zipcodebystate":   func(f *gofakeit.Faker, _ fakerArgs) string { return f.Zip() },
	"address.country":          func(f *gofakeit.Faker, _ fakerArgs) string { return f.Country() },
	"address.countrycode":      func(f *gofakeit.Faker, _ fakerArgs) string { return f.CountryAbr() },
	"address.latitude":         func(f *gofakeit.Faker, _ fakerArgs) string { return strconv.FormatFloat(f.Latitude(), 'f', 8, 64) },
	"address.longitude":        func(f *gofakeit.Faker, _ fakerArgs) string { return strconv.FormatFloat(f.Longitude(), 'f', 8, 64) },
	"address.timezone":         func(f *gofakeit.Faker, _ fakerArgs) string { return f.TimeZone() },

	"internet.emailaddress":     func(f *gofakeit.Faker, _ fakerArgs) string { return f.Email() },
	"internet.safeemailaddress": func(f *gofakeit.Faker, _ fakerArgs) string { return f.Username() + "@example.com" },
	"internet.url":              func(f *gofakeit.Faker, _ fakerArgs) string { return f.URL() },
	"internet.domainname":       func(f *gofakeit.Faker, _ fakerArgs) string { return f.DomainName() },
	"internet.domainsuffix":     func(f *gofakeit.Faker, _ fakerArgs) string { return f.DomainSuffix() },
	"internet.ipv4address":      func(f *gofakeit.Faker, _ fakerArgs) string { return f.IPv4Address() },
	"internet.ipv6address":      func(f *gofakeit.Faker, _ fakerArgs) string { return f.IPv6Address() },
	"internet.macaddress":       func(f *gofakeit.Faker, _ fakerArgs) string { return f.MacAddress() },
	"internet.password":         func(f *gofakeit.Faker, _ fakerArgs) string { return f.Password(true, true, true, false, false, 12) },
	"internet.useragentany":     func(f *gofakeit.Faker, _ fakerArgs) string { return f.UserAgent() },
	"internet.uuid":             func(f *gofakeit.Faker, _ fakerArgs) string { return f.UUID() },

	"phonenumber.phonenumber": func(f *gofakeit.Faker, _ fakerArgs) string { return f.PhoneFormatted() },
	"phonenumber.cellphone":   func(f *gofakeit.Faker, _ fakerArgs) string { return f.PhoneFormatted() },

	"company.name":        func(f *gofakeit.Faker, _ fakerArgs) string { return f.Company() },
	"company.suffix":      func(f *gofakeit.Faker, _ fakerArgs) string { return f.CompanySuffix() },
	"company.buzzword":    func(f *gofakeit.Faker, _ fakerArgs) string { return f.BS() },
	"company.bs":          func(f *gofakeit.Faker, _ fakerArgs) string { return f.BS() },
	"company.catchphrase": func(f *gofakeit.Faker, _ fakerArgs) string { return f.Slogan() },
	"company.profession":  func(f *gofakeit.Faker, _ fakerArgs) string { return f.JobTitle() },
	"company.industry":    func(f *gofakeit.Faker, _ fakerArgs) string { return f.JobDescriptor() },
	"company.url":         func(f *gofakeit.Faker, _ fakerArgs) string { return f.URL() },

	"lorem.word":      func(f *gofakeit.Faker, _ fakerArgs) string { return f.LoremIpsumWord() },
	"lorem.character": func(f *gofakeit.Faker, _ fakerArgs) string { return strings.ToLower(f.Letter()) },
	"lorem.sentence": func(f *gofakeit.Faker, args fakerArgs) string {
		return f.LoremIpsumSentence(args.int(0, 6))
	},
	"lorem.paragraph": func(f *gofakeit.Faker, args fakerArgs) string {
		return f.LoremIpsumParagraph(1, args.int(0, 3), 8, " ")
	},
	"lorem.characters": func(f *gofakeit.Faker, args fakerArgs) string {
		var b strings.Builder
		for range args.int(0, 255) {
			b.WriteString(strings.ToLower(f.Letter()))
		}
		return b.String()
	},

	"number.randomdigit": func(f *gofakeit.Faker, _ fakerArgs) string { return f.Digit() },
	"number.randomdigitnotzero": func(f *gofakeit.Faker, _ fakerArgs) string {
		return strconv.Itoa(f.Number(1, 9))
	},
	"number.randomnumber": func(f *gofakeit.Faker, _ fakerArgs) string { return strconv.Itoa(f.Number(0, math.MaxInt32)) },
	"number.numberbetween": func(f *gofakeit.Faker, args fakerArgs) string {
		// JavaFaker: min inclusive, max exclusive.
		lo, hi := args.int64(0), args.int64(1)
		if hi <= lo {
			return strconv.FormatInt(lo, 10)
		}
		return strconv.FormatInt(lo+int64(f.Uint64()%uint64(hi-lo)), 10)
	},
	"number.digits": func(f *gofakeit.Faker, args fakerArgs) string {
		var b strings.Builder
		for range args.int(0, 1) {
			b.WriteString(f.Digit())
		}
		return b.String()
	},
	"number.randomdouble": func(f *gofakeit.Faker, args fakerArgs) string {
		decimals := args.int(0, 2)
		v := f.Float64Range(float64(args.int(1, 0)), float64(args.int(2, 100)))
		p := math.Pow(10, float64(decimals))
		return match.FormatJavaDouble(math.Round(v*p) / p)
	},

	"commerce.productname": func(f *gofakeit.Faker, _ fakerArgs) string { return f.ProductName() },
	"commerce.department":  func(f *gofakeit.Faker, _ fakerArgs) string { return f.ProductCategory() },
	"commerce.color":       func(f *gofakeit.Faker, _ fakerArgs) string { return f.Color() },
	"commerce.material":    func(f *gofakeit.Faker, _ fakerArgs) string { return f.Noun() },
	"commerce.price": func(f *gofakeit.Faker, args fakerArgs) string {
		return strconv.FormatFloat(f.Price(args.float(0, 1), args.float(1, 100)), 'f', 2, 64)
	},
	"commerce.promotioncode": func(f *gofakeit.Faker, _ fakerArgs) string { return strings.ToUpper(f.Word()) + f.Digit() + f.Digit() },

	"color.name": func(f *gofakeit.Faker, _ fakerArgs) string { return f.Color() },
	"color.hex":  func(f *gofakeit.Faker, _ fakerArgs) string { return f.HexColor() },

	"book.title":     func(f *gofakeit.Faker, _ fakerArgs) string { return f.BookTitle() },
	"book.author":    func(f *gofakeit.Faker, _ fakerArgs) string { return f.BookAuthor() },
	"book.genre":     func(f *gofakeit.Faker, _ fakerArgs) string { return f.BookGenre() },
	"book.publisher": func(f *gofakeit.Faker, _ fakerArgs) string { return f.Company() },

	"job.title":     func(f *gofakeit.Faker, _ fakerArgs) string { return f.JobTitle() },
	"job.field":     func(f *gofakeit.Faker, _ fakerArgs) string { return f.JobDescriptor() },
	"job.position":  func(f *gofakeit.Faker, _ fakerArgs) string { return f.JobTitle() },
	"job.seniority": func(f *gofakeit.Faker, _ fakerArgs) string { return f.JobLevel() },

	"animal.name":              func(f *gofakeit.Faker, _ fakerArgs) string { return f.Animal() },
	"cat.name":                 func(f *gofakeit.Faker, _ fakerArgs) string { return f.PetName() },
	"cat.breed":                func(f *gofakeit.Faker, _ fakerArgs) string { return f.Cat() },
	"dog.name":                 func(f *gofakeit.Faker, _ fakerArgs) string { return f.PetName() },
	"dog.breed":                func(f *gofakeit.Faker, _ fakerArgs) string { return f.Dog() },
	"beer.name":                func(f *gofakeit.Faker, _ fakerArgs) string { return f.BeerName() },
	"beer.style":               func(f *gofakeit.Faker, _ fakerArgs) string { return f.BeerStyle() },
	"app.name":                 func(f *gofakeit.Faker, _ fakerArgs) string { return f.AppName() },
	"programminglanguage.name": func(f *gofakeit.Faker, _ fakerArgs) string { return f.ProgrammingLanguage() },
	"currency.name":            func(f *gofakeit.Faker, _ fakerArgs) string { return f.CurrencyLong() },
	"currency.code":            func(f *gofakeit.Faker, _ fakerArgs) string { return f.CurrencyShort() },
	"demographic.sex":          func(f *gofakeit.Faker, _ fakerArgs) string { return f.Gender() },
	"educator.university":      func(f *gofakeit.Faker, _ fakerArgs) string { return f.School() },
	"food.fruit":               func(f *gofakeit.Faker, _ fakerArgs) string { return f.Fruit() },
	"food.vegetable":           func(f *gofakeit.Faker, _ fakerArgs) string { return f.Vegetable() },
	"food.dish":                func(f *gofakeit.Faker, _ fakerArgs) string { return f.Dinner() },
	"hipster.word":             func(f *gofakeit.Faker, _ fakerArgs) string { return f.HipsterWord() },
	"idnumber.ssnvalid":        func(f *gofakeit.Faker, _ fakerArgs) string { return f.SSN() },
	"idnumber.valid":           func(f *gofakeit.Faker, _ fakerArgs) string { return f.SSN() },
	"finance.creditcard":       func(f *gofakeit.Faker, _ fakerArgs) string { return f.CreditCardNumber(nil) },
	"bool.bool":                func(f *gofakeit.Faker, _ fakerArgs) string { return strconv.FormatBool(f.Bool()) },
	"crypto.md5":               func(f *gofakeit.Faker, _ fakerArgs) string { return strings.ReplaceAll(f.UUID(), "-", "") },
}

// fakerArgs are the raw arguments of the faker method ("1", "2147483647000L",
// "10.5", "true", "'FL'").
type fakerArgs []string

func (a fakerArgs) int(i, def int) int {
	if i >= len(a) {
		return def
	}
	n, err := strconv.ParseInt(strings.TrimRight(a[i], "lL"), 10, 64)
	if err != nil {
		return def
	}
	return int(n)
}

func (a fakerArgs) float(i int, def float64) float64 {
	if i >= len(a) {
		return def
	}
	f, err := strconv.ParseFloat(strings.TrimRight(a[i], "dDfFlL"), 64)
	if err != nil {
		return def
	}
	return f
}

func (a fakerArgs) bool(i int) bool { return i < len(a) && a[i] == "true" }

var fakerStep = regexp.MustCompile(`^\s*([A-Za-z0-9_]+)\s*(?:\(([^()]*)\))?\s*$`)

// fakerCall splits a SpEL-like "category().method(1, 2)" into its lookup key
// and raw arguments. Anything richer (constructors, T(...) types) is not
// supported.
func fakerCall(api string) (string, fakerArgs, error) {
	steps := splitTopLevel(api)
	if len(steps) != 2 {
		return "", nil, fmt.Errorf("unsupported faker api %q", api)
	}
	var names []string
	var args fakerArgs
	for i, step := range steps {
		m := fakerStep.FindStringSubmatch(step)
		if m == nil {
			return "", nil, fmt.Errorf("unsupported faker api %q", api)
		}
		names = append(names, strings.ToLower(m[1]))
		if i == 1 && strings.TrimSpace(m[2]) != "" {
			for _, a := range strings.Split(m[2], ",") {
				args = append(args, strings.Trim(strings.TrimSpace(a), `'"`))
			}
		}
	}
	return strings.Join(names, "."), args, nil
}

// faker mirrors FakerExpression: every failure yields "".
func faker(c call) (string, error) {
	api, ok := c.arg("api").(argString)
	if !ok {
		return "", nil
	}
	key, args, err := fakerCall(string(api))
	if err != nil {
		return "", nil
	}
	fn, ok := fakerAPI[key]
	if !ok {
		return "", nil
	}
	return fn(gofakeit.New(0), args), nil
}

func (a fakerArgs) int64(i int) int64 {
	if i >= len(a) {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimRight(a[i], "lL"), 10, 64)
	return n
}

// splitTopLevel splits on the dots that are not inside parentheses.
func splitTopLevel(api string) []string {
	var steps []string
	depth, last := 0, 0
	for i, r := range api {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case '.':
			if depth == 0 {
				steps = append(steps, api[last:i])
				last = i + 1
			}
		}
	}
	return append(steps, api[last:])
}
