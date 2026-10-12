package focus

import (
	"encoding/json"
	"sort"
	"strings"
)

// MaxTagsCell bounds the Tags cell of one export row that is parsed.
const MaxTagsCell = 16 << 10

// ParseTagsCell reads the FOCUS Tags column: a JSON object, or (older Azure exports) the members of one without the braces. Anything else
// gives nil: a row whose tags cannot be read is untagged, it never fails an export.
func ParseTagsCell(s string) map[string]string {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > MaxTagsCell {
		return nil
	}
	if !strings.HasPrefix(s, "{") {
		s = "{" + s + "}"
	}
	var raw map[string]any
	if json.Unmarshal([]byte(s), &raw) != nil {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		switch x := v.(type) {
		case string:
			out[k] = x
		case float64, bool:
			b, _ := json.Marshal(x)
			out[k] = string(b)
		}
	}
	return out
}

// TagRow is one line of the "tagged" part of a provider's aggregate (the contract the normalizers read).
type TagRow struct {
	Day       string  `json:"day"`
	Service   string  `json:"service"`
	Currency  string  `json:"currency"`
	Key       string  `json:"key"`
	Value     string  `json:"value"`
	Billed    float64 `json:"billed"`
	Effective float64 `json:"effective"`
}

// TagSums folds the cost of charges by (day, service, currency, tag key, tag value) for the allocation keys only. The zero value reads nothing.
type TagSums struct {
	keys map[string]bool
	sums map[TagRow]*[2]float64 // keyed by the row with its amounts zeroed
}

// NewTagSums reads the given keys (any spelling; see NormalizeTagKey).
func NewTagSums(keys []string) *TagSums {
	return &TagSums{keys: KeySet(keys), sums: map[TagRow]*[2]float64{}}
}

// Len is the number of distinct lines held (providers count it against their size limit).
func (t *TagSums) Len() int {
	if t == nil {
		return 0
	}
	return len(t.sums)
}

// Add folds one charge in. lowerValues makes values case-insensitive (Azure and GCP treat them so; AWS values are case-sensitive).
func (t *TagSums) Add(day, service, currency string, tags map[string]string, lowerValues bool, billed, effective float64) {
	if t == nil || len(t.keys) == 0 || len(tags) == 0 || (billed == 0 && effective == 0) {
		return // a charge of nothing says nothing about who spends (the cost lines skip it too)
	}
	for k, v := range SelectTags(tags, t.keys) {
		if lowerValues {
			v = strings.ToLower(v)
		}
		id := TagRow{Day: day, Service: service, Currency: currency, Key: k, Value: v}
		s := t.sums[id]
		if s == nil {
			s = &[2]float64{}
			t.sums[id] = s
		}
		s[0] += billed
		s[1] += effective
	}
}

// Rows lists the lines in a stable order (the aggregate is archived and compared in tests).
func (t *TagSums) Rows() []TagRow {
	if t == nil {
		return []TagRow{}
	}
	out := make([]TagRow, 0, len(t.sums))
	for id, v := range t.sums {
		id.Billed, id.Effective = v[0], v[1]
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool {
		x, y := out[i], out[j]
		return x.Day+"|"+x.Service+"|"+x.Key+"|"+x.Value+"|"+x.Currency < y.Day+"|"+y.Service+"|"+y.Key+"|"+y.Value+"|"+y.Currency
	})
	return out
}

// Active reports whether any key is read (so a provider can skip parsing the Tags cell of every row when none is).
func (t *TagSums) Active() bool { return t != nil && len(t.keys) > 0 }
