package aws

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

// SourceFocusExport is the focus.RawBatch.Source of the aggregate built from a FOCUS data export.
const SourceFocusExport = "focus_export"

// Hard limits: an export is customer-controlled input, so every dimension is bounded.
const (
	maxManifestBytes  = 1 << 20
	maxDataFiles      = 500
	maxFileBytes      = 8 << 30 // decompressed, per file
	maxAggregateKeys  = 500_000
	ctxCheckEveryRows = 10_000
)

// S3API is the slice of the S3 client needed to read an export (faked in tests).
type S3API interface {
	GetObject(ctx context.Context, in *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// S3Assumer returns an S3 client acting as roleARN with the given ExternalId, for a bucket in region.
type S3Assumer func(ctx context.Context, roleARN, externalID, region string) (S3API, error)

// exportBatch reads the months overlapping [from, to) of the connection's export and returns the daily
// aggregate (see ingestion/normalizers/awsexport for the contract).
func (p *Provider) exportBatch(ctx context.Context, c domain.Connection, from, to time.Time) (focus.RawBatch, error) {
	cfg := c.Export
	if p.AssumeS3 == nil {
		return focus.RawBatch{}, errors.New("aws: S3 access is not configured")
	}
	s3c, err := p.AssumeS3(ctx, c.CredentialRef, c.ExternalID, cfg.Region)
	if err != nil {
		return focus.RawBatch{}, classify(err)
	}
	agg := newAggregate(from, to)
	found := 0
	for _, month := range monthsIn(from, to) {
		keys, ok, err := manifestFiles(ctx, s3c, *cfg, month)
		if err != nil {
			return focus.RawBatch{}, err
		}
		if !ok {
			continue // not delivered yet (typical for the newest month)
		}
		found++
		for _, key := range keys {
			if err := readDataFile(ctx, s3c, cfg.Bucket, key, agg); err != nil {
				return focus.RawBatch{}, err
			}
		}
	}
	if found == 0 {
		return focus.RawBatch{}, domain.ErrExportNotFound
	}
	payload, err := json.Marshal(agg.payload())
	if err != nil {
		return focus.RawBatch{}, err
	}
	return focus.RawBatch{Provider: "aws", Source: SourceFocusExport, Payload: payload}, nil
}

// monthsIn lists the first day of every month that overlaps [from, to).
func monthsIn(from, to time.Time) []time.Time {
	var out []time.Time
	for m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC); m.Before(to); m = m.AddDate(0, 1, 0) {
		out = append(out, m)
	}
	return out
}

// manifestFiles returns the data-file keys listed by the month's manifest. Only keys inside the export's own
// folder are accepted: a manifest must not be able to point the platform at other objects.
func manifestFiles(ctx context.Context, s3c S3API, cfg domain.ExportConfig, month time.Time) ([]string, bool, error) {
	key := fmt.Sprintf("%smetadata/BILLING_PERIOD=%s/%s-Manifest.json", cfg.Root(), month.Format("2006-01"), cfg.Name)
	out, err := s3c.GetObject(ctx, &s3.GetObjectInput{Bucket: &cfg.Bucket, Key: &key})
	if err != nil {
		var nf *s3types.NoSuchKey
		if errors.As(err, &nf) {
			return nil, false, nil
		}
		return nil, false, classify(err)
	}
	defer func() { _ = out.Body.Close() }()
	var m struct {
		DataFiles []string `json:"dataFiles"`
	}
	if err := json.NewDecoder(io.LimitReader(out.Body, maxManifestBytes)).Decode(&m); err != nil {
		return nil, false, fmt.Errorf("aws: unreadable export manifest: %w", err)
	}
	if len(m.DataFiles) > maxDataFiles {
		return nil, false, fmt.Errorf("aws: export has %d data files (limit %d)", len(m.DataFiles), maxDataFiles)
	}
	for _, k := range m.DataFiles {
		if !strings.HasPrefix(k, cfg.Root()+"data/") || strings.Contains(k, "..") {
			return nil, false, fmt.Errorf("aws: manifest lists a file outside the export folder")
		}
	}
	return m.DataFiles, true, nil
}

// limitedReader fails (instead of silently truncating) when a stream exceeds its limit.
type limitedReader struct {
	r io.Reader
	n int64
}

var errTooLarge = errors.New("aws: export file exceeds the size limit")

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.n <= 0 {
		return 0, errTooLarge
	}
	if int64(len(p)) > l.n {
		p = p[:l.n]
	}
	n, err := l.r.Read(p)
	l.n -= int64(n)
	return n, err
}

// readDataFile streams one gzip-compressed CSV and feeds its rows to the aggregate.
func readDataFile(ctx context.Context, s3c S3API, bucket, key string, agg *aggregate) error {
	out, err := s3c.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		return classify(err)
	}
	defer func() { _ = out.Body.Close() }()
	gz, err := gzip.NewReader(out.Body)
	if err != nil {
		return fmt.Errorf("aws: export file is not gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

	r := csv.NewReader(&limitedReader{r: gz, n: maxFileBytes})
	r.ReuseRecord = true
	header, err := r.Read()
	if err != nil {
		return fmt.Errorf("aws: export file has no header: %w", err)
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
			return fmt.Errorf("aws: export row %d: %w", rows+2, err)
		}
		if err := agg.add(cols, rec); err != nil {
			return err
		}
	}
}

// columns maps FOCUS column names to positions. The required ones must exist; x_ columns are AWS extensions.
type columns struct {
	billed, effective, currency, start, region, service, category, qty, unit int
	usageType, serviceCode                                                   int // -1 when absent
}

func newColumns(header []string) (columns, error) {
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.TrimPrefix(h, "\ufeff")] = i
	}
	need := func(name string) (int, error) {
		i, ok := idx[name]
		if !ok {
			return 0, fmt.Errorf("aws: export lacks the FOCUS column %q (is it a FOCUS 1.0 export?)", name)
		}
		return i, nil
	}
	opt := func(name string) int {
		if i, ok := idx[name]; ok {
			return i
		}
		return -1
	}
	var c columns
	var err error
	for _, f := range []struct {
		dst  *int
		name string
	}{
		{&c.billed, "BilledCost"}, {&c.effective, "EffectiveCost"}, {&c.currency, "BillingCurrency"}, {&c.start, "ChargePeriodStart"},
		{&c.region, "RegionId"}, {&c.service, "ServiceName"}, {&c.category, "ServiceCategory"},
		{&c.qty, "ConsumedQuantity"}, {&c.unit, "ConsumedUnit"},
	} {
		if *f.dst, err = need(f.name); err != nil {
			return c, err
		}
	}
	c.usageType, c.serviceCode = opt("x_UsageType"), opt("x_ServiceCode")
	return c, nil
}

func (c columns) width() int {
	w := 0
	for _, i := range []int{c.billed, c.effective, c.currency, c.start, c.region, c.service, c.category, c.qty, c.unit, c.usageType, c.serviceCode} {
		w = max(w, i+1)
	}
	return w
}

const (
	ec2ComputeService = "Amazon Elastic Compute Cloud - Compute" // named like Cost Explorer so measured hours match their cost line
	ec2OtherService   = "EC2 - Other"
)

var (
	// "BoxUsage:m5.large", "EUC1-SpotUsage:c7g.xlarge", "HeavyUsage:m5.large" ... = instance running hours.
	ec2HoursUsage = regexp.MustCompile(`^(?:[A-Z0-9]+-)?(?:BoxUsage|SpotUsage|HeavyUsage|DedicatedUsage|HostBoxUsage):([a-z0-9-]+\.[a-z0-9]+)$`)
	// "TimedStorage-ByteHrs", "EUC1-TimedStorage-SIA-ByteHrs" ... = S3 storage.
	s3StorageUsage = regexp.MustCompile(`^(?:[A-Z0-9]+-)?TimedStorage-(?:[A-Z0-9-]+-)?ByteHrs$`)
)

type costKey struct{ day, service, category, region, currency string }
type regionDay struct{ day, region string }
type hoursKey struct {
	day, region, instanceType string
}

// aggregate folds the rows of an export into daily sums within [from, to).
type aggregate struct {
	from, to time.Time
	cost     map[costKey]*[2]float64 // billed, effective
	hours    map[hoursKey]float64
	s3       map[regionDay]float64
}

func newAggregate(from, to time.Time) *aggregate {
	return &aggregate{from: from, to: to, cost: map[costKey]*[2]float64{}, hours: map[hoursKey]float64{}, s3: map[regionDay]float64{}}
}

func (a *aggregate) size() int { return len(a.cost) + len(a.hours) + len(a.s3) }

func num(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

func (a *aggregate) add(c columns, rec []string) error {
	if len(rec) < c.width() {
		return errors.New("aws: export row is shorter than its header")
	}
	if len(rec[c.start]) < 10 {
		return fmt.Errorf("aws: bad ChargePeriodStart %q", rec[c.start])
	}
	day := rec[c.start][:10] // charge periods are UTC ISO-8601
	d, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return fmt.Errorf("aws: bad ChargePeriodStart %q", rec[c.start])
	}
	if d.Before(a.from) || !d.Before(a.to) {
		return nil
	}
	if a.size() > maxAggregateKeys {
		return errors.New("aws: export has too many distinct lines (limit exceeded)")
	}

	region := strings.ToLower(strings.TrimSpace(rec[c.region]))
	service := rec[c.service]
	usageType := ""
	if c.usageType >= 0 {
		usageType = rec[c.usageType]
	}
	isEC2 := (c.serviceCode >= 0 && rec[c.serviceCode] == "AmazonEC2") || service == "Amazon Elastic Compute Cloud"
	if isEC2 {
		service = ec2OtherService
		if ec2HoursUsage.MatchString(usageType) {
			service = ec2ComputeService
		}
	}

	k := costKey{day, service, rec[c.category], region, strings.ToUpper(strings.TrimSpace(rec[c.currency]))}
	sums := a.cost[k]
	if sums == nil {
		sums = &[2]float64{}
		a.cost[k] = sums
	}
	sums[0] += num(rec[c.billed])
	sums[1] += num(rec[c.effective])

	qty := num(rec[c.qty])
	unit := strings.TrimSpace(rec[c.unit])
	switch {
	case isEC2 && qty > 0 && strings.EqualFold(unit, "Hrs"):
		if m := ec2HoursUsage.FindStringSubmatch(usageType); m != nil {
			a.hours[hoursKey{day, region, strings.ToLower(m[1])}] += qty
		}
	case qty > 0 && strings.EqualFold(unit, "GB-Mo") && s3StorageUsage.MatchString(usageType) && service == "Amazon Simple Storage Service":
		a.s3[regionDay{day, region}] += qty
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
	type hoursRow struct {
		Day          string  `json:"day"`
		Region       string  `json:"region"`
		InstanceType string  `json:"instance_type"`
		Hours        float64 `json:"hours"`
	}
	hours := make([]hoursRow, 0, len(a.hours))
	for k, v := range a.hours {
		hours = append(hours, hoursRow{k.day, k.region, k.instanceType, v})
	}
	sort.Slice(hours, func(i, j int) bool {
		x, y := hours[i], hours[j]
		return x.Day+x.Region+x.InstanceType < y.Day+y.Region+y.InstanceType
	})
	type s3Row struct {
		Day      string  `json:"day"`
		Region   string  `json:"region"`
		GBMonths float64 `json:"gb_months"`
	}
	s3rows := make([]s3Row, 0, len(a.s3))
	for k, v := range a.s3 {
		s3rows = append(s3rows, s3Row{k.day, k.region, v})
	}
	sort.Slice(s3rows, func(i, j int) bool { return s3rows[i].Day+s3rows[i].Region < s3rows[j].Day+s3rows[j].Region })
	return map[string]any{"cost": cost, "ec2_hours": hours, "s3_gb_months": s3rows}
}

// validateExport proves the role can read the export: a manifest must exist for the current or previous month.
func (p *Provider) validateExport(ctx context.Context, c domain.Connection) error {
	if p.AssumeS3 == nil {
		return errors.New("aws: S3 access is not configured")
	}
	s3c, err := p.AssumeS3(ctx, c.CredentialRef, c.ExternalID, c.Export.Region)
	if err != nil {
		return classify(err)
	}
	now := time.Now().UTC()
	for _, month := range []time.Time{time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC), time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)} {
		if _, ok, err := manifestFiles(ctx, s3c, *c.Export, month); err != nil {
			return err
		} else if ok {
			return nil
		}
	}
	return domain.ErrExportNotFound
}
