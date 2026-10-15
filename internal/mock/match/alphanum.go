package match

import "unicode/utf16"

// alphanumCompare is a literal port of AlphanumComparator. It works on UTF-16
// code units like Java's String.charAt, so non-ASCII names sort exactly as in
// CastleMock.
func alphanumCompare(a, b string) int {
	s1, s2 := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	i, j := 0, 0
	for i < len(s1) && j < len(s2) {
		c1 := chunk(s1, i)
		i += len(c1)
		c2 := chunk(s2, j)
		j += len(c2)
		var result int
		if isDigit(c1[0]) && isDigit(c2[0]) {
			result = len(c1) - len(c2)
			if result == 0 {
				for k := range c1 {
					if d := int(c1[k]) - int(c2[k]); d != 0 {
						return d
					}
				}
			}
		} else {
			result = compareUnits(c1, c2)
		}
		if result != 0 {
			return result
		}
	}
	return len(s1) - len(s2)
}

func isDigit(c uint16) bool { return c >= '0' && c <= '9' }

func chunk(s []uint16, marker int) []uint16 {
	digit := isDigit(s[marker])
	end := marker + 1
	for end < len(s) && isDigit(s[end]) == digit {
		end++
	}
	return s[marker:end]
}

// compareUnits mirrors String.compareTo.
func compareUnits(a, b []uint16) int {
	for k := 0; k < len(a) && k < len(b); k++ {
		if a[k] != b[k] {
			return int(a[k]) - int(b[k])
		}
	}
	return len(a) - len(b)
}
