// Package costcsv reads a cost export of any vendor that is not one of the three clouds (Datadog, Snowflake, MongoDB Atlas, GitHub, a
// hand-made spreadsheet ...) into FOCUS records of the provider "saas" (ADR-0062). Vendors differ in column names but not in what they
// mean, so the columns are found by name from a list of the usual spellings (FOCUS first), and the answer says which column was used for
// what: a file that cannot be understood is refused with the names that would have worked, never guessed at.
//
// What it keeps is cost only: per day, service and currency. There is no region (the cost is "global") and no energy model, so no carbon is
// ever derived from it; its tags (a Tags column, or tag_* / label_* columns) feed showback like those of a cloud.
package costcsv

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

// Limits: a file is customer input.
const (
	MaxBytes    = 8 << 20
	MaxRows     = 200_000
	MaxVendor   = 60
	MaxServices = 2000
)

// Options say what the file does not.
type Options struct {
	Vendor string // shown before the service name: "Datadog: Log Management"
	// Currency is used for rows without one (and when the file has no currency column); empty means USD.
	Currency string
	// TagKeys are the allocation keys to keep (any spelling); empty keeps no tags.
	TagKeys []string
}

// Result is what was read.
type Result struct {
	Records []focus.Record
	Tagged  []focus.TagRow
	// RowsRead counts data rows; RowsSkipped those that could not be used (bad date, bad amount), RowsImported the rest.
	RowsRead, RowsSkipped, RowsImported int
	From, To                            time.Time // first and last day
	Services                            int
	Currency                            string // the currency of most of the cost
	Total                               float64
	Columns                             map[string]string // role -> header used
	Warnings                            []string
}

var (
	dateNames    = []string{"chargeperiodstart", "date", "usagedate", "day", "startdate", "usagestartdate", "periodstart", "billingperiodstart", "usagedatetime", "usagestarttime", "month", "period"}
	costNames    = []string{"billedcost", "cost", "amount", "totalcost", "unblendedcost", "pretaxcost", "charge", "charges", "netcost", "total", "price", "costusd"}
	effNames     = []string{"effectivecost", "amortizedcost", "netamortizedcost"}
	currNames    = []string{"billingcurrency", "currency", "currencycode", "billingcurrencycode"}
	serviceNames = []string{"servicename", "service", "servicetype", "productname", "product", "producttype", "meter", "metername", "sku", "skuname", "feature", "usagetype", "item", "category", "description", "resourcetype"}
	tagsNames    = []string{"tags", "tag", "labels", "resourcetags"}
	tagPrefixes  = []string{"tag_", "tag:", "tags_", "label_", "label:", "labels_"}
)

func norm(h string) string {
	h = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(h), "\ufeff"))
	return regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(h, "")
}

// Parse reads the file. It returns an error only for a file it cannot use at all.
func Parse(data []byte, opt Options) (Result, error) {
	var res Result
	if len(data) == 0 {
		return res, errors.New("the file is empty")
	}
	if len(data) > MaxBytes {
		return res, fmt.Errorf("the file is larger than %d MB", MaxBytes>>20)
	}
	vendor := strings.TrimSpace(opt.Vendor)
	if vendor == "" || len(vendor) > MaxVendor || strings.ContainsAny(vendor, "\r\n\t") {
		return res, errors.New("the vendor name is required (1 to 60 characters)")
	}
	cur0 := strings.ToUpper(strings.TrimSpace(opt.Currency))
	if cur0 == "" {
		cur0 = "USD"
	}
	if len(cur0) != 3 {
		return res, errors.New("the currency must be a three-letter code like USD")
	}

	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	r := csv.NewReader(bufio.NewReader(bytes.NewReader(data)))
	r.Comma = delimiter(data)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.ReuseRecord = false
	header, err := r.Read()
	if err != nil {
		return res, errors.New("the file has no header row")
	}
	idx := map[string]int{}
	for i, h := range header {
		if n := norm(h); n != "" {
			if _, dup := idx[n]; !dup {
				idx[n] = i
			}
		}
	}
	find := func(names []string) (int, string) {
		for _, n := range names {
			if i, ok := idx[n]; ok {
				return i, header[i]
			}
		}
		return -1, ""
	}
	// A vendor's own name for a column ("CREDITS_USED_COST", "usage_date") is found when exactly one header contains the word: two candidates
	// would be a guess, and then the file is refused with what was found.
	loose := func(word string, not ...string) (int, string) {
		hit, name := -1, ""
		for n, i := range idx {
			if !strings.Contains(n, word) {
				continue
			}
			skip := false
			for _, x := range not {
				if strings.Contains(n, x) {
					skip = true
				}
			}
			if skip {
				continue
			}
			if hit >= 0 {
				return -1, ""
			}
			hit, name = i, header[i]
		}
		return hit, name
	}
	dateI, dateH := find(dateNames)
	if dateI < 0 {
		dateI, dateH = loose("date")
	}
	costI, costH := find(costNames)
	if costI < 0 {
		costI, costH = loose("cost", "effective", "amortized", "currency", "unit", "usd")
	}
	if dateI < 0 || costI < 0 {
		return res, fmt.Errorf("the file needs a date column (for example ChargePeriodStart, Date, UsageDate) and a cost column (BilledCost, Cost, Amount); found: %s",
			strings.Join(trimmed(header), ", "))
	}
	effI, effH := find(effNames)
	curI, curH := find(currNames)
	svcI, svcH := find(serviceNames)
	if svcI < 0 {
		svcI, svcH = loose("service")
	}
	tagsI, tagsH := find(tagsNames)
	type tagCol struct {
		i   int
		key string
	}
	var tagCols []tagCol
	keys := focus.KeySet(opt.TagKeys)
	for i, h := range header {
		n := strings.ToLower(strings.TrimSpace(h))
		for _, p := range tagPrefixes {
			if strings.HasPrefix(n, p) {
				if k := focus.NormalizeTagKey(strings.TrimPrefix(n, p)); k != "" {
					tagCols = append(tagCols, tagCol{i, k})
				}
				break
			}
		}
	}
	res.Columns = map[string]string{"date": dateH, "cost": costH}
	if effI >= 0 {
		res.Columns["effective_cost"] = effH
	}
	if curI >= 0 {
		res.Columns["currency"] = curH
	}
	if svcI >= 0 {
		res.Columns["service"] = svcH
	}
	if tagsI >= 0 {
		res.Columns["tags"] = tagsH
	}
	for _, c := range tagCols {
		res.Columns["tag:"+c.key] = header[c.i]
	}

	type key struct {
		day, service, currency string
	}
	sums := map[key]*[2]float64{}
	tagSums := focus.NewTagSums(opt.TagKeys)
	monthly := false
	curTotals := map[string]float64{}
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			res.RowsRead++
			res.RowsSkipped++
			continue
		}
		if blank(rec) {
			continue
		}
		res.RowsRead++
		if res.RowsRead > MaxRows {
			return res, fmt.Errorf("the file has more than %d rows: split it, or sum it by day first", MaxRows)
		}
		get := func(i int) string {
			if i < 0 || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		day, isMonth, ok := parseDay(get(dateI))
		billed, ok2 := parseAmount(get(costI), r.Comma == ';')
		if !ok || !ok2 {
			res.RowsSkipped++
			continue
		}
		if isMonth {
			monthly = true
		}
		eff := billed
		if effI >= 0 {
			if v, ok := parseAmount(get(effI), r.Comma == ';'); ok {
				eff = v
			}
		}
		cur := cur0
		if c := strings.ToUpper(get(curI)); len(c) == 3 {
			cur = c
		}
		service := vendor
		if s := get(svcI); s != "" && !strings.EqualFold(s, vendor) {
			service = vendor + ": " + s
		}
		if len(service) > 120 {
			service = service[:120]
		}
		if billed == 0 && eff == 0 {
			res.RowsImported++
			continue
		}
		k := key{day.Format(time.DateOnly), service, cur}
		s := sums[k]
		if s == nil {
			if len(sums) >= MaxServices*400 {
				return res, errors.New("the file has too many distinct services and days")
			}
			s = &[2]float64{}
			sums[k] = s
		}
		s[0] += billed
		s[1] += eff
		curTotals[cur] += billed
		res.RowsImported++
		if len(keys) > 0 {
			tags := map[string]string{}
			if tagsI >= 0 {
				for k, v := range focus.ParseTagsCell(get(tagsI)) {
					tags[k] = v
				}
			}
			for _, c := range tagCols {
				if v := get(c.i); v != "" {
					tags[c.key] = v
				}
			}
			tagSums.Add(k.day, service, cur, tags, true, billed, eff)
		}
	}
	if res.RowsRead == 0 {
		return res, errors.New("the file has a header but no rows")
	}
	if len(sums) == 0 {
		return res, fmt.Errorf("no row could be used: %d of %d rows had no readable date or amount", res.RowsSkipped, res.RowsRead)
	}
	services := map[string]bool{}
	ks := make([]key, 0, len(sums))
	for k := range sums {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool {
		a, b := ks[i], ks[j]
		return a.day+"|"+a.service+"|"+a.currency < b.day+"|"+b.service+"|"+b.currency
	})
	for _, k := range ks {
		d, _ := time.Parse(time.DateOnly, k.day)
		v := sums[k]
		res.Records = append(res.Records, focus.Record{Provider: "saas", Source: "cost_csv", ServiceName: k.service, ServiceCategory: "other", RegionID: "global",
			BilledCost: v[0], EffectiveCost: v[1], Currency: k.currency, ChargePeriodStart: d, ChargePeriodEnd: d.AddDate(0, 0, 1)})
		services[k.service] = true
		if res.From.IsZero() || d.Before(res.From) {
			res.From = d
		}
		if d.After(res.To) {
			res.To = d
		}
	}
	res.Services = len(services)
	res.Tagged = tagSums.Rows()
	best := 0.0
	for c, v := range curTotals {
		if math.Abs(v) >= best {
			best, res.Currency = math.Abs(v), c
		}
		res.Total += 0 // the total is only meaningful in one currency: below
	}
	for _, rec := range res.Records {
		if rec.Currency == res.Currency {
			res.Total += rec.BilledCost
		}
	}
	if len(curTotals) > 1 {
		res.Warnings = append(res.Warnings, "the file has more than one currency; each is kept as it is and converted for reporting")
	}
	if monthly {
		res.Warnings = append(res.Warnings, "some dates are months: that cost is placed on the first day of the month, not spread over its days")
	}
	if res.RowsSkipped > 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("%d of %d rows were skipped: no readable date or amount", res.RowsSkipped, res.RowsRead))
	}
	if svcI < 0 {
		res.Warnings = append(res.Warnings, "the file has no service column: all of it is filed under the vendor name")
	}
	return res, nil
}

func trimmed(h []string) []string {
	out := make([]string, 0, len(h))
	for _, x := range h {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
		if len(out) == 12 {
			break
		}
	}
	return out
}

func blank(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// delimiter picks the separator by the header line: the most frequent of comma, semicolon and tab.
func delimiter(data []byte) rune {
	line := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		line = data[:i]
	}
	best, n := ',', 0
	for _, d := range []rune{',', ';', '\t'} {
		if c := bytes.Count(line, []byte(string(d))); c > n {
			best, n = d, c
		}
	}
	return best
}

var dateLayouts = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02", "2006/01/02", "02.01.2006", "01/02/2006", "20060102"}

// parseDay reads a date or timestamp as a UTC day; a bare year-month is accepted and flagged.
func parseDay(s string) (day time.Time, month bool, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false, false
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			t = t.UTC()
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), false, plausible(t)
		}
	}
	for _, l := range []string{"2006-01", "2006/01", "January 2006", "Jan 2006"} {
		if t, err := time.Parse(l, s); err == nil {
			return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), true, plausible(t)
		}
	}
	return time.Time{}, false, false
}

// plausible refuses dates that are surely a mistake (a unit price column read as a date, year 0001): 2000 to 2100.
func plausible(t time.Time) bool { return t.Year() >= 2000 && t.Year() <= 2100 }

var decimalComma = regexp.MustCompile(`^-?\d+,\d{1,2}$`)

// parseAmount reads a money amount: an optional currency sign, thousands separators, a decimal point or (in semicolon files) a decimal comma,
// parentheses or a leading minus for a credit.
func parseAmount(s string, semicolon bool) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	neg := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg, s = true, strings.Trim(s, "()")
	}
	s = strings.NewReplacer("$", "", "€", "", "£", "", "¥", "", " ", "", " ", "").Replace(s)
	switch {
	case decimalComma.MatchString(s) && (semicolon || !strings.Contains(s, ".")):
		s = strings.Replace(s, ",", ".", 1)
	default:
		s = strings.ReplaceAll(s, ",", "")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e12 {
		return 0, false
	}
	if neg {
		v = -v
	}
	return v, true
}

// Normalizer adapts Parse to the ingestion pipeline. What the file cannot say comes in the batch's Meta: vendor, currency, tag_keys (comma
// separated allocation keys) and import_id, which becomes the source of every line so that the file can be taken back.
type Normalizer struct{}

func optionsOf(raw focus.RawBatch) Options {
	var keys []string
	if k := raw.Meta["tag_keys"]; k != "" {
		keys = strings.Split(k, ",")
	}
	return Options{Vendor: raw.Meta["vendor"], Currency: raw.Meta["currency"], TagKeys: keys}
}

// Normalize implements the ingestion Normalizer port.
func (Normalizer) Normalize(raw focus.RawBatch) ([]focus.Record, error) {
	res, err := Parse(raw.Payload, optionsOf(raw))
	if err != nil {
		return nil, err
	}
	if id := raw.Meta["import_id"]; id != "" {
		for i := range res.Records {
			res.Records[i].Source = "import:" + id
		}
	}
	return res.Records, nil
}

// NormalizeTags implements focus.TagNormalizer.
func (Normalizer) NormalizeTags(raw focus.RawBatch) ([]focus.TaggedCost, error) {
	res, err := Parse(raw.Payload, optionsOf(raw))
	if err != nil {
		return nil, err
	}
	out := make([]focus.TaggedCost, 0, len(res.Tagged))
	for _, t := range res.Tagged {
		d, err := time.Parse(time.DateOnly, t.Day)
		if err != nil {
			continue
		}
		out = append(out, focus.TaggedCost{Provider: "saas", ServiceName: t.Service, Currency: t.Currency, Day: d, TagKey: t.Key, TagValue: t.Value,
			BilledCost: t.Billed, EffectiveCost: t.Effective})
	}
	return out, nil
}
