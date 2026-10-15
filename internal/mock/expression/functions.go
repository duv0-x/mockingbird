package expression

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	mrand "math/rand/v2"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/duv0-x/mockingbird/internal/mock/match"
)

type function func(c call) (string, error)

// functions is TextParser's registry. RANDOM_PASSWORD is bound to
// RandomDecimalExpression (a float), a CastleMock bug kept on purpose, and
// RANDOM_DECIMAL itself is not registered.
var functions = map[string]function{
	"RANDOM_INTEGER":   randomInteger,
	"RANDOM_DOUBLE":    randomDouble,
	"RANDOM_LONG":      randomLong,
	"RANDOM_FLOAT":     randomFloat,
	"RANDOM_BOOLEAN":   randomBoolean,
	"RANDOM_DATE":      randomDate("2006-01-02"),
	"RANDOM_STRING":    randomString,
	"RANDOM_UUID":      randomUUID,
	"RANDOM_EMAIL":     randomEmail,
	"RANDOM_PASSWORD":  randomFloat,
	"RANDOM_DATE_TIME": randomDate("2006-01-02T15:04:05"),
	"RANDOM_ENUM":      randomEnum,
	"PATH_PARAMETER":   pathParameter,
	"QUERY_STRING":     queryString,
	"BODY_JSON_PATH":   bodyJSONPath,
	"URL_HOST":         urlHost,
	"BODY_XPATH":       bodyXPath,
	"FAKER":            faker,
}

var errBound = errors.New("bound must be positive")

// javaInt mirrors Double.intValue: truncation, saturating at the int range.
func javaInt(f float64) int32 {
	switch {
	case math.IsNaN(f):
		return 0
	case f >= math.MaxInt32:
		return math.MaxInt32
	case f <= math.MinInt32:
		return math.MinInt32
	}
	return int32(f)
}

func intArg(c call, name string, def int32) int32 {
	if n, ok := c.arg(name).(argNumber); ok {
		return javaInt(float64(n))
	}
	return def
}

// nextInt mirrors Random.nextInt(bound), which rejects bound <= 0.
func nextInt(bound int32) (int32, error) {
	if bound <= 0 {
		return 0, errBound
	}
	return mrand.Int32N(bound), nil
}

// RandomIntegerExpression: nextInt(max) + min with Java int overflow.
func randomInteger(c call) (string, error) {
	n, err := nextInt(intArg(c, "max", 2000))
	if err != nil {
		return "", err
	}
	return strconv.Itoa(int(n + intArg(c, "min", -1000))), nil
}

// RandomDoubleExpression: an int (nextInt(max) + min) printed as a double.
func randomDouble(c call) (string, error) {
	n, err := nextInt(intArg(c, "max", math.MaxInt32))
	if err != nil {
		return "", err
	}
	return match.FormatJavaDouble(float64(n + intArg(c, "min", 0))), nil
}

func randomLong(call) (string, error) {
	return strconv.FormatInt(int64(mrand.Uint64()), 10), nil
}

// randomFloat mirrors Float.toString(Random.nextFloat()).
func randomFloat(call) (string, error) {
	return match.FormatJavaFloat(float32(mrand.Uint32()>>8) / (1 << 24)), nil
}

func randomBoolean(call) (string, error) {
	return strconv.FormatBool(mrand.IntN(2) == 1), nil
}

// randomDate mirrors RandomDate(Time)Expression: |now ± 10^12 ms| formatted in
// the server's time zone.
func randomDate(layout string) function {
	const dateRange = int64(1_000_000_000_000)
	return func(call) (string, error) {
		now := time.Now().UnixMilli()
		v := now - dateRange + int64(mrand.Float64()*float64(2*dateRange))
		if v < 0 {
			v = -v
		}
		return time.UnixMilli(v).Format(layout), nil
	}
}

const alphanumeric = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// alphanumericString mirrors RandomStringUtils.randomAlphanumeric.
func alphanumericString(length int32) (string, error) {
	if length < 0 {
		return "", fmt.Errorf("requested random string length %d is less than 0", length)
	}
	b := make([]byte, length)
	for i := range b {
		b[i] = alphanumeric[mrand.IntN(len(alphanumeric))]
	}
	return string(b), nil
}

func randomString(call) (string, error) {
	return alphanumericString(mrand.Int32N(15) + 5)
}

func randomUUID(call) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("random uuid: %w", err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func randomEmail(c call) (string, error) {
	domain, ok := c.arg("domain").(argString)
	if !ok {
		d, err := alphanumericString(mrand.Int32N(10) + 5)
		if err != nil {
			return "", err
		}
		domain = argString(d)
	}
	topDomain, ok := c.arg("topDomain").(argString)
	if !ok {
		topDomain = "com"
	}
	n, err := nextInt(intArg(c, "max", 10))
	if err != nil {
		return "", err
	}
	user, err := alphanumericString(n + intArg(c, "min", 5))
	if err != nil {
		return "", err
	}
	return user + "@" + string(domain) + "." + string(topDomain), nil
}

func randomEnum(c call) (string, error) {
	values, ok := c.arg("values").(argArray)
	if !ok {
		return "", errors.New("unable to extract the enum values")
	}
	i, err := nextInt(int32(len(values)))
	if err != nil {
		return "", err
	}
	if _, nested := values[i].(argArray); nested {
		return "", errors.New("the enum argument can't be an array")
	}
	return values[i].text(), nil
}

// pathParameter returns the smallest value of the parameter (the Java code
// sorts the value set and takes the first).
func pathParameter(c call) (string, error) {
	params, ok := c.arg(argPathParameters).(argMap)
	key, isString := c.arg("parameter").(argString)
	if !ok || !isString {
		return "", nil
	}
	values, _ := params[string(key)].(argArray)
	texts := make([]string, 0, len(values))
	for _, v := range values {
		texts = append(texts, v.text())
	}
	if len(texts) == 0 {
		return "", nil
	}
	return slices.Min(texts), nil
}

func queryString(c call) (string, error) {
	params, ok := c.arg(argQueryStrings).(argMap)
	key, isString := c.arg("query").(argString)
	if !ok || !isString {
		return "", nil
	}
	if v, found := params[string(key)]; found {
		return v.text(), nil
	}
	return "", nil
}

// urlHost mirrors new URI(url).toURL().getHost().
func urlHost(c call) (string, error) {
	raw, ok := c.arg(argURL).(argString)
	if !ok {
		return "", nil
	}
	u, err := url.Parse(string(raw))
	if err != nil {
		return "", nil
	}
	if !u.IsAbs() {
		return "", errors.New("URI is not absolute")
	}
	host := u.Host
	if strings.HasPrefix(host, "[") {
		if i := strings.LastIndex(host, "]"); i >= 0 {
			return host[:i+1], nil
		}
	}
	return u.Hostname(), nil
}

func bodyXPath(c call) (string, error) {
	return bodyQuery(c, match.XPathValue)
}

func bodyJSONPath(c call) (string, error) {
	return bodyQuery(c, match.JSONPathValue)
}

func bodyQuery(c call, eval func(body, expr string) (string, bool)) (string, error) {
	body, expr := c.arg(argBody), c.arg("expression")
	if body == nil || expr == nil {
		return "", nil
	}
	v, _ := eval(body.text(), expr.text())
	return v, nil
}
