package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

type fakeStore struct {
	saved     []domain.Calculation
	stored    map[string]float64
	savedGrid map[string]float64
	totals    map[string]domain.Totals // by methodology version
	replaced  []string                 // versions passed to ReplaceCalculations
	dayAvg    map[string]float64       // mean reading of the usage day, per region
}

func (f *fakeStore) Summary(context.Context, string, *string, time.Time, time.Time) (domain.Summary, error) {
	return domain.Summary{}, nil
}
func (f *fakeStore) Trend(context.Context, string, *string, time.Time, time.Time) ([]domain.TrendPoint, error) {
	return nil, nil
}
func (f *fakeStore) VersionTotals(_ context.Context, _ string, _ *string, _, _ time.Time, v string) (domain.Totals, error) {
	return f.totals[v], nil
}
func (f *fakeStore) ReplaceCalculations(_ context.Context, _, _ string, _, _ time.Time, v string, c []domain.Calculation) error {
	f.replaced = append(f.replaced, v)
	f.saved = append(f.saved, c...)
	return nil
}
func (f *fakeStore) SaveGridIntensity(_ context.Context, _, region string, _ time.Time, g float64, _ bool, _, _ string) error {
	if f.savedGrid == nil {
		f.savedGrid = map[string]float64{}
	}
	f.savedGrid[region] = g
	return nil
}

// DayStats: the readings of the day (dayAvg), counted as a full day of 24.
func (f *fakeStore) DayStats(_ context.Context, region, provider string, _, _ time.Time) (float64, int, error) {
	if g, ok := f.dayAvg[region]; ok && provider == domain.ProviderElectricityMaps {
		return g, 24, nil
	}
	return 0, 0, nil
}

// Latest: the stored reading (stored), whatever the time.
func (f *fakeStore) Latest(_ context.Context, region, provider string, _ *time.Time) (float64, time.Time, bool, error) {
	g, ok := f.stored[region]
	return g, time.Now(), ok && provider == domain.ProviderElectricityMaps, nil
}
func (f *fakeStore) RegionsInUse(context.Context) ([]string, error) { return nil, nil }
func (f *fakeStore) Monthly(context.Context, string, string, time.Time) (float64, bool, error) {
	return 0, false, nil
}
func (f *fakeStore) MonthlyHistory(context.Context, string, time.Time, time.Time) ([]domain.MonthlyRow, error) {
	return nil, nil
}
func (f *fakeStore) History(context.Context, string, time.Time, time.Time) ([]domain.HistoryRow, error) {
	return nil, nil
}
func (f *fakeStore) Regions(context.Context, string, *string, time.Time, time.Time, string) ([]domain.RegionCarbon, error) {
	return nil, nil
}
func (f *fakeStore) Origins(context.Context, string, *string, time.Time, time.Time, string) ([]domain.IntensityOrigin, error) {
	return nil, nil
}

type usageSrc struct{ rows []domain.UsageDay }

func (u usageSrc) DailyAggregates(context.Context, string, string, time.Time, time.Time) ([]domain.UsageDay, error) {
	return u.rows, nil
}

type unitsSrc map[time.Time]float64

func (u unitsSrc) FunctionalUnitsForDay(_ context.Context, _, _ string, d time.Time) (float64, error) {
	return u[d], nil
}

type policySrc struct{ th domain.Thresholds }

func (p policySrc) CIThresholds(context.Context, string, string) (domain.Thresholds, error) {
	return p.th, nil
}

type grid struct {
	g     float64
	calls int
	err   error
}

func (g *grid) GetIntensity(_ context.Context, region string) (domain.Intensity, error) {
	g.calls++
	return domain.Intensity{Region: region, GPerKWh: g.g, At: time.Now()}, g.err
}
func (g *grid) GetForecast(context.Context, string) (domain.Forecast, error) {
	return domain.Forecast{}, nil
}

type rec struct{ tasks []string }

func (r *rec) Enqueue(_ context.Context, t string, _ any, _ ...asynq.Option) error {
	r.tasks = append(r.tasks, t)
	return nil
}

var day = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

func newSvc(st *fakeStore, rows []domain.UsageDay, units unitsSrc, p domain.CarbonDataProvider, th domain.Thresholds) (*Service, *rec) {
	q := &rec{}
	return &Service{Store: st, Usage: usageSrc{rows}, Units: units, Policies: policySrc{th}, Provider: p, Queue: q,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))}, q
}

func ec2Day() domain.UsageDay {
	return domain.UsageDay{Provider: "aws", ServiceName: "EC2", ServiceCategory: "compute", RegionID: "eu-central-1", Currency: "USD", BilledCost: 100, Day: day}
}

func TestCalculateWindowUsesStoredIntensityAndSCI(t *testing.T) {
	st := &fakeStore{stored: map[string]float64{"eu-central-1": 400}}
	g := &grid{g: 999}
	svc, _ := newSvc(st, []domain.UsageDay{ec2Day()}, unitsSrc{day: 1000}, g, domain.Thresholds{})
	res, err := svc.CalculateWindow(context.Background(), "t", "p", day, day.AddDate(0, 0, 1))
	if err != nil || res.Calculated != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	c := st.saved[0]
	if c.IntensityGPerKWh != 400 || g.calls != 0 {
		t.Errorf("must use the stored reading (provider calls=%d, intensity=%v)", g.calls, c.IntensityGPerKWh)
	}
	if c.SCIScore == nil || math.Abs(*c.SCIScore-c.CO2eKg*1000/1000) > 1e-9 {
		t.Errorf("SCI = %v, carbon %v kg, R=1000", c.SCIScore, c.CO2eKg)
	}
}

func TestCalculateWindowFallsBackToProviderAndPersists(t *testing.T) {
	st := &fakeStore{stored: map[string]float64{}}
	g := &grid{g: 250}
	svc, _ := newSvc(st, []domain.UsageDay{ec2Day()}, unitsSrc{}, g, domain.Thresholds{})
	res, err := svc.CalculateWindow(context.Background(), "t", "p", day, day.AddDate(0, 0, 1))
	if err != nil || res.Calculated != 1 || st.saved[0].IntensityGPerKWh != 250 || st.savedGrid["eu-central-1"] != 250 {
		t.Fatalf("provider fallback failed: %+v %v saved=%v", res, err, st.savedGrid)
	}
	if st.saved[0].SCIScore != nil {
		t.Error("SCI must stay NULL without reported functional units")
	}
}

func TestCalculateWindowProviderOutageSkipsInsteadOfGuessing(t *testing.T) {
	st := &fakeStore{stored: map[string]float64{}}
	svc, _ := newSvc(st, []domain.UsageDay{ec2Day()}, unitsSrc{}, &grid{err: errors.New("503")}, domain.Thresholds{})
	res, err := svc.CalculateWindow(context.Background(), "t", "p", day, day.AddDate(0, 0, 1))
	if err != nil || res.Calculated != 0 || res.Skipped[domain.SkipNoGridData] != 1 {
		t.Errorf("outage must skip rows, not fail or invent data: %+v %v", res, err)
	}
}

func TestRunWindowJobChainsRecommendations(t *testing.T) {
	st := &fakeStore{stored: map[string]float64{"eu-central-1": 400}}
	svc, q := newSvc(st, []domain.UsageDay{ec2Day()}, unitsSrc{}, nil, domain.Thresholds{})
	if err := svc.RunWindowJob(context.Background(), queue.WindowPayload{TenantID: "t", ProjectID: "p", From: "2026-09-01", To: "2026-09-02"}); err != nil {
		t.Fatal(err)
	}
	if len(q.tasks) != 1 || q.tasks[0] != queue.TaskCalcRecommendation {
		t.Errorf("chain: %v", q.tasks)
	}
	svc2, q2 := newSvc(&fakeStore{}, nil, unitsSrc{}, nil, domain.Thresholds{})
	_ = svc2.RunWindowJob(context.Background(), queue.WindowPayload{TenantID: "t", ProjectID: "p", From: "2026-09-01", To: "2026-09-02"})
	if len(q2.tasks) != 0 {
		t.Error("nothing calculated must not queue recommendations")
	}
	if err := svc.RunWindowJob(context.Background(), queue.WindowPayload{From: "bad", To: "worse"}); err == nil {
		t.Error("bad window accepted")
	}
}

func TestEvaluateGate(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	st := &fakeStore{stored: map[string]float64{"eu-central-1": 400}}
	svc, _ := newSvc(st, nil, unitsSrc{}, nil, domain.Thresholds{CarbonWarnKg: f(1), CarbonBlockKg: f(5), CostBlock: f(100)})
	change := Change{Kind: domain.KindVCPUHours, Amount: 100_000, Region: "eu-central-1", MonthlyCostDelta: 40}
	r, err := svc.Evaluate(context.Background(), "t", GateRequest{ProjectID: "p", Changes: []Change{change}})
	if err != nil {
		t.Fatal(err)
	}
	// 100000 h * 0.0021 kWh * 1.135 * 400 g / 1000 = ~95 kg
	if r.Verdict != domain.Block || r.CarbonKgMonth < 90 || r.CarbonKgMonth > 100 || !r.ThresholdsApplied || r.MethodologyVersion == "" {
		t.Errorf("verdict: %+v", r)
	}
	// request thresholds override policy
	r2, _ := svc.Evaluate(context.Background(), "t", GateRequest{ProjectID: "p", Changes: []Change{change},
		Thresholds: &ThresholdsInput{CarbonWarnKg: f(1000), CarbonBlockKg: f(2000), CostBlock: f(1000)}})
	if r2.Verdict != domain.Pass {
		t.Errorf("override ignored: %+v", r2)
	}
	for name, req := range map[string]GateRequest{
		"empty":     {ProjectID: "p"},
		"no region": {ProjectID: "p", Changes: []Change{{Kind: domain.KindVCPUHours, Amount: 1}}},
		"bad kind":  {ProjectID: "p", Changes: []Change{{Kind: "x", Amount: 1, Region: "eu-central-1"}}},
	} {
		if _, err := svc.Evaluate(context.Background(), "t", req); !errors.Is(err, ErrInvalidGateRequest) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := svc.Evaluate(context.Background(), "t", GateRequest{ProjectID: "p", Changes: []Change{{Kind: domain.KindVCPUHours, Amount: 1, Region: "mars"}}}); !errors.Is(err, ErrNoGridData) {
		t.Errorf("unknown region must fail closed, got %v", err)
	}
}

// A day of usage ran under the mean of that day's readings, not under the last reading stored before midnight.
func TestCalculateWindowUsesTheMeanIntensityOfTheDay(t *testing.T) {
	st := &fakeStore{stored: map[string]float64{"eu-central-1": 900}, dayAvg: map[string]float64{"eu-central-1": 300}}
	svc, _ := newSvc(st, []domain.UsageDay{ec2Day()}, unitsSrc{}, &grid{g: 999}, domain.Thresholds{})
	res, err := svc.CalculateWindow(context.Background(), "t", "p", day, day.AddDate(0, 0, 1))
	if err != nil || res.Calculated != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if got := st.saved[0].IntensityGPerKWh; got != 300 {
		t.Errorf("intensity = %v, want the day mean 300 (not the last reading 900)", got)
	}
}
