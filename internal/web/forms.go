package web

import (
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var rowField = regexp.MustCompile(`^([a-z]+)\[(\d+)\]\.(\w+)$`)

// indexedRows groups fields named prefix[i].field (row editors, DESIGN.md
// §7.7) into rows ordered by index; removed rows simply leave gaps.
func indexedRows(f url.Values, prefix string) []map[string]string {
	rows := map[int]map[string]string{}
	for key, vals := range f {
		m := rowField.FindStringSubmatch(key)
		if m == nil || m[1] != prefix || len(vals) == 0 {
			continue
		}
		i, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		if rows[i] == nil {
			rows[i] = map[string]string{}
		}
		rows[i][m[3]] = vals[0]
	}
	idx := make([]int, 0, len(rows))
	for i := range rows {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	out := make([]map[string]string, 0, len(idx))
	for _, i := range idx {
		out = append(out, rows[i])
	}
	return out
}

func rowBool(r map[string]string, key string) bool {
	v := r[key]
	return v == "true" || v == "on" || v == "1"
}

func atoiField(s string) (int, error) { return strconv.Atoi(strings.TrimSpace(s)) }
