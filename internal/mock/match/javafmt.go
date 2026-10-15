package match

import (
	"math"
	"strconv"
	"strings"
)

// FormatJavaDouble renders f like Java's Double.toString: plain notation with
// at least one fractional digit for 1e-3 <= |f| < 1e7, otherwise computerized
// scientific notation ("9.07231552E8", "1.0E-5").
func FormatJavaDouble(f float64) string { return formatJava(f, 64) }

// FormatJavaFloat renders f like Java's Float.toString.
func FormatJavaFloat(f float32) string { return formatJava(float64(f), 32) }

func formatJava(f float64, bits int) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		if math.Signbit(f) {
			return "-0.0"
		}
		return "0.0"
	}
	abs := math.Abs(f)
	if abs >= 1e-3 && abs < 1e7 {
		s := strconv.FormatFloat(f, 'f', -1, bits)
		if !strings.Contains(s, ".") {
			s += ".0"
		}
		return s
	}
	s := strconv.FormatFloat(f, 'e', -1, bits)
	mantissa, exp, _ := strings.Cut(s, "e")
	if !strings.Contains(mantissa, ".") {
		mantissa += ".0"
	}
	n, _ := strconv.Atoi(exp)
	return mantissa + "E" + strconv.Itoa(n)
}
