package azure

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

// Hard limits: an export is customer-controlled input, so every dimension is bounded.
const (
	maxDataFiles      = 500
	maxFileBytes      = 8 << 30  // decompressed, per file
	maxCompressedFile = 2 << 30  // as stored, per file
	maxTotalBytes     = 24 << 30 // decompressed, over all files of one sync: bounds time and bandwidth
	maxLineBytes      = 1 << 20  // one CSV line: a file without line breaks must not be buffered whole
	maxAggregateKeys  = 500_000
	ctxCheckEveryRows = 10_000
)

var (
	errTooLarge = errors.New("azure: export exceeds the size limit")
	errLongLine = errors.New("azure: export line exceeds the length limit")

	// Cost Management writes <export directory>/<yyyymmdd>-<yyyymmdd>/<run>/part_*.csv(.gz).
	dateRangeRe = regexp.MustCompile(`^(\d{8})-(\d{8})$`)
	hoursRe     = regexp.MustCompile(`(?i)^\s*(\d+(?:\.\d+)?)?\s*(?:hours?|hrs?)\s*$`)
	gbMonthRe   = regexp.MustCompile(`(?i)^\s*(\d+(?:\.\d+)?)?\s*GB\s*/\s*Month\s*$`)
)

const (
	vmService      = "Virtual Machines"
	vmCompute      = "Virtual Machines - Compute" // the lines whose running hours are measured
	vmOther        = "Virtual Machines - Other"
	storageService = "Storage"
	storageCap     = "Storage - Capacity"
)

// dataFile is one CSV of the newest run of a date range.
type dataFile struct {
	name string
	size int64
}

// selectFiles picks, for every date range overlapping [from, to), the files of its newest run (a re-delivery
// writes a new run folder, and the newest one is complete). The listing is customer-controlled, so only names
// under the export directory with the expected shape are accepted.
func selectFiles(dir string, blobs []BlobInfo, from, to time.Time) ([]dataFile, error) {
	type run struct {
		key   string
		files []dataFile
	}
	best := map[string]*run{} // date range -> newest run
	prefix := dir + "/"
	for _, b := range blobs {
		rel, ok := strings.CutPrefix(b.Name, prefix)
		if !ok || strings.Contains(rel, "..") {
			continue
		}
		lower := strings.ToLower(rel)
		if !strings.HasSuffix(lower, ".csv") && !strings.HasSuffix(lower, ".csv.gz") {
			continue
		}
		segs := strings.Split(rel, "/")
		m := dateRangeRe.FindStringSubmatch(segs[0])
		if m == nil || len(segs) < 2 {
			continue
		}
		start, err1 := time.Parse("20060102", m[1])
		end, err2 := time.Parse("20060102", m[2])
		if err1 != nil || err2 != nil || !start.Before(to) || end.Before(from) {
			continue
		}
		key := ""
		if len(segs) > 2 {
			key = segs[1]
		}
		r := best[segs[0]]
		switch {
		case r == nil || key > r.key:
			best[segs[0]] = &run{key: key, files: []dataFile{{b.Name, b.Size}}}
		case key == r.key:
			r.files = append(r.files, dataFile{b.Name, b.Size})
		}
	}
	var out []dataFile
	for _, r := range best {
		out = append(out, r.files...)
	}
	if len(out) > maxDataFiles {
		return nil, fmt.Errorf("azure: export has %d data files in the window (limit %d)", len(out), maxDataFiles)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out, nil
}

// limitedReader fails (instead of silently truncating) when a stream exceeds its limit.
type limitedReader struct {
	r     io.Reader
	n     int64
	total *int64 // the sync-wide byte budget this stream draws from
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.n <= 0 {
		return 0, errTooLarge
	}
	if int64(len(p)) > l.n {
		p = p[:l.n]
	}
	n, err := l.r.Read(p)
	l.n -= int64(n)
	if l.total != nil {
		*l.total -= int64(n)
		if *l.total < 0 {
			return n, errTooLarge
		}
	}
	return n, err
}

// lineLimited fails when more than maxLineBytes pass without a line break: encoding/csv would otherwise buffer an
// arbitrarily long "line" in memory.
type lineLimited struct {
	r   io.Reader
	run int
}

func (l *lineLimited) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	for _, b := range p[:n] {
		if b == '\n' {
			l.run = 0
		} else if l.run++; l.run > maxLineBytes {
			return n, errLongLine
		}
	}
	return n, err
}

// readDataFile streams one CSV (plain or gzip) and feeds its rows to the aggregate.
func readDataFile(ctx context.Context, b Blobs, e domain.AzureExport, f dataFile, agg *aggregate) error {
	body, size, err := b.Open(ctx, e, f.name)
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()
	if size > maxCompressedFile {
		return fmt.Errorf("azure: export file is %d bytes (limit %d)", size, int64(maxCompressedFile))
	}
	var src io.Reader = io.LimitReader(body, maxCompressedFile+1)
	if strings.HasSuffix(strings.ToLower(f.name), ".gz") {
		gz, err := gzip.NewReader(src)
		if err != nil {
			return fmt.Errorf("azure: export file is not gzip: %w", err)
		}
		defer func() { _ = gz.Close() }()
		src = gz
	}
	r := csv.NewReader(&lineLimited{r: &limitedReader{r: src, n: min(maxFileBytes, agg.budget), total: &agg.budget}})
	r.ReuseRecord = true
	header, err := r.Read()
	if err != nil {
		return fmt.Errorf("azure: export file has no header: %w", err)
	}
	cols, err := newColumns(header)
	if err != nil {
		return err
	}
	for rows := 0; ; rows++ {
		if rows%ctxCheckEveryRows == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("azure: export row %d: %w", rows+2, err)
		}
		if err := agg.add(cols, rec); err != nil {
			return err
		}
	}
}

// columns maps FOCUS column names to positions. The required ones must exist; x_ columns are Azure extensions
// and optional (without them a line is simply not measured and keeps its cost-based estimate).
type columns struct {
	billed, effective, currency, start, region, service, category, qty, unit, subAccount int
	charge, meterCategory, meterSub, meterName, cores                                    int // -1 when absent
	tags                                                                                 int // the FOCUS Tags column, -1 when absent
}

func newColumns(header []string) (columns, error) {
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.TrimPrefix(h, "\ufeff")] = i
	}
	var c columns
	for _, f := range []struct {
		dst  *int
		name string
	}{
		{&c.billed, "BilledCost"}, {&c.effective, "EffectiveCost"}, {&c.currency, "BillingCurrency"}, {&c.start, "ChargePeriodStart"},
		{&c.region, "RegionId"}, {&c.service, "ServiceName"}, {&c.category, "ServiceCategory"},
		{&c.qty, "ConsumedQuantity"}, {&c.unit, "ConsumedUnit"}, {&c.subAccount, "SubAccountId"},
	} {
		i, ok := idx[f.name]
		if !ok {
			return c, fmt.Errorf("azure: export lacks the FOCUS column %q (is it a FOCUS 1.0 cost and usage export?)", f.name)
		}
		*f.dst = i
	}
	opt := func(name string) int {
		if i, ok := idx[name]; ok {
			return i
		}
		return -1
	}
	c.charge, c.meterCategory, c.meterSub = opt("ChargeCategory"), opt("x_SkuMeterCategory"), opt("x_SkuMeterSubcategory")
	c.meterName, c.cores, c.tags = opt("x_SkuMeterName"), opt("x_SkuCoreCount"), opt("Tags")
	return c, nil
}

func (c columns) width() int {
	w := 0
	for _, i := range []int{c.billed, c.effective, c.currency, c.start, c.region, c.service, c.category, c.qty, c.unit,
		c.subAccount, c.charge, c.meterCategory, c.meterSub, c.meterName, c.cores, c.tags} {
		w = max(w, i+1)
	}
	return w
}

type costKey struct{ day, service, category, region, currency string }
type regionDay struct{ day, region string }

// aggregate folds the rows of an export into daily sums within [from, to) for one subscription.
type aggregate struct {
	budget       int64 // decompressed bytes this sync may still read, across all files
	from, to     time.Time
	subscription string
	cost         map[costKey]*[2]float64 // billed, effective
	vcpuHours    map[regionDay]float64
	storageGBMo  map[regionDay]float64
	matched      bool // a row of the subscription was seen in any period
	tags         *focus.TagSums
}

func newAggregate(subscription string, from, to time.Time) *aggregate {
	return &aggregate{budget: maxTotalBytes, from: from, to: to, subscription: strings.ToLower(subscription),
		cost: map[costKey]*[2]float64{}, vcpuHours: map[regionDay]float64{}, storageGBMo: map[regionDay]float64{}}
}

func (a *aggregate) size() int {
	return len(a.cost) + len(a.vcpuHours) + len(a.storageGBMo) + a.tags.Len()
}

func num(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

// multiplier reads the leading count of a unit like "100 GB/Month" or "1 Hour" (absent = 1).
func multiplier(m []string) float64 {
	if len(m) > 1 && m[1] != "" {
		if v := num(m[1]); v > 0 {
			return v
		}
	}
	return 1
}

func (a *aggregate) add(c columns, rec []string) error {
	if len(rec) < c.width() {
		return errors.New("azure: export row is shorter than its header")
	}
	// A billing-scope export covers many subscriptions; only this connection's rows are read.
	sub := strings.ToLower(strings.TrimSpace(rec[c.subAccount]))
	if strings.TrimPrefix(sub, "/subscriptions/") != a.subscription {
		return nil
	}
	a.matched = true
	if len(rec[c.start]) < 10 {
		return fmt.Errorf("azure: bad ChargePeriodStart %q", rec[c.start])
	}
	day := rec[c.start][:10] // charge periods are UTC ISO-8601
	d, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return fmt.Errorf("azure: bad ChargePeriodStart %q", rec[c.start])
	}
	if d.Before(a.from) || !d.Before(a.to) {
		return nil
	}
	if a.size() > maxAggregateKeys {
		return errors.New("azure: export has too many distinct lines (limit exceeded)")
	}
	field := func(i int) string {
		if i < 0 {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}
	region := strings.ToLower(strings.TrimSpace(rec[c.region]))
	service := strings.TrimSpace(rec[c.service])
	meterCat := field(c.meterCategory)
	usage := field(c.charge) == "" || strings.EqualFold(field(c.charge), "Usage")
	qty := num(rec[c.qty])
	unit := rec[c.unit]

	var vcpu, gbMo float64
	switch {
	case service == vmService && usage && strings.EqualFold(meterCat, vmService) && qty > 0 &&
		!strings.Contains(strings.ToLower(field(c.meterSub)), "windows"): // the license meter repeats the same hours
		if m := hoursRe.FindStringSubmatch(unit); m != nil {
			if cores := num(field(c.cores)); cores > 0 {
				vcpu = qty * multiplier(m) * cores
			}
		}
	case service == storageService && usage && strings.EqualFold(meterCat, "Storage") && qty > 0 &&
		strings.Contains(strings.ToLower(field(c.meterName)), "data stored"):
		if m := gbMonthRe.FindStringSubmatch(unit); m != nil {
			gbMo = qty * multiplier(m)
		}
	}
	// Measured lines are filed under their own service name, so the carbon engine replaces exactly the cost lines
	// they correspond to (licences, purchases, operations and the like keep a cost-based estimate).
	switch {
	case service == vmService && vcpu > 0:
		service = vmCompute
	case service == vmService:
		service = vmOther
	case service == storageService && gbMo > 0:
		service = storageCap
	}

	k := costKey{day, service, strings.TrimSpace(rec[c.category]), region, strings.ToUpper(strings.TrimSpace(rec[c.currency]))}
	sums := a.cost[k]
	if sums == nil {
		sums = &[2]float64{}
		a.cost[k] = sums
	}
	sums[0] += num(rec[c.billed])
	sums[1] += num(rec[c.effective])
	if a.tags.Active() && c.tags >= 0 {
		a.tags.Add(day, service, k.currency, focus.ParseTagsCell(rec[c.tags]), true, num(rec[c.billed]), num(rec[c.effective]))
	}
	if vcpu > 0 {
		a.vcpuHours[regionDay{day, region}] += vcpu
	}
	if gbMo > 0 {
		a.storageGBMo[regionDay{day, region}] += gbMo
	}
	return nil
}

// payload renders the aggregate in a stable order (the JSON is archived and compared in tests).
func (a *aggregate) payload() map[string]any {
	type costRow struct {
		Day       string  `json:"day"`
		Service   string  `json:"service"`
		Category  string  `json:"category"`
		Region    string  `json:"region"`
		Currency  string  `json:"currency"`
		Billed    float64 `json:"billed"`
		Effective float64 `json:"effective"`
	}
	cost := make([]costRow, 0, len(a.cost))
	for k, v := range a.cost {
		cost = append(cost, costRow{k.day, k.service, k.category, k.region, k.currency, v[0], v[1]})
	}
	sort.Slice(cost, func(i, j int) bool {
		x, y := cost[i], cost[j]
		return x.Day+x.Service+x.Region+x.Currency < y.Day+y.Service+y.Region+y.Currency
	})
	type measureRow struct {
		Day    string  `json:"day"`
		Region string  `json:"region"`
		Value  float64 `json:"value"`
	}
	rows := func(m map[regionDay]float64) []measureRow {
		out := make([]measureRow, 0, len(m))
		for k, v := range m {
			out = append(out, measureRow{k.day, k.region, v})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Day+out[i].Region < out[j].Day+out[j].Region })
		return out
	}
	return map[string]any{"cost": cost, "vm_vcpu_hours": rows(a.vcpuHours), "storage_gb_months": rows(a.storageGBMo), "tagged": a.tags.Rows()}
}
