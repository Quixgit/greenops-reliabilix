// Package application holds the carbon use-cases: calculation pipeline, summaries and the CI gate.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/apiusage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/methodology"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

// Store persists calculations and reads aggregates (implemented by repository.Postgres).
type Store interface {
	Summary(ctx context.Context, tenantID string, project *string, from, to time.Time) (domain.Summary, error)
	Trend(ctx context.Context, tenantID string, project *string, from, to time.Time) ([]domain.TrendPoint, error)
	// ReplaceCalculations makes [from, to) of one methodology version equal to calcs; other versions are untouched.
	ReplaceCalculations(ctx context.Context, tenantID, projectID string, from, to time.Time, version string, calcs []domain.Calculation) error
	// VersionTotals aggregates the stored rows of one methodology version.
	VersionTotals(ctx context.Context, tenantID string, project *string, from, to time.Time, version string) (domain.Totals, error)
	SaveGridIntensity(ctx context.Context, provider, region string, at time.Time, gPerKWh float64, forecast bool, measure, resolution string) error
	// Forecast returns one source's stored forecast points of a region in [from, to).
	Forecast(ctx context.Context, region, provider string, from, to time.Time) ([]domain.ForecastPoint, error)
	// RegionCoverage says, per region in use, how many of the last N days have grid data (hourly or a monthly figure).
	RegionCoverage(ctx context.Context, days int) ([]domain.RegionCoverage, error)
	RegionHourlyCoverage(ctx context.Context, days int) ([]domain.RegionCoverage, error)
	// RegionsInUse lists the regions that have calculations, for any organization.
	RegionsInUse(ctx context.Context) ([]string, error)
	// MonthlyHistory returns the published monthly figures of a region that cover [from, to).
	MonthlyHistory(ctx context.Context, region string, from, to time.Time) ([]domain.MonthlyRow, error)
	domain.IntensityReader
	// History returns each source's daily readings of a region in [from, to).
	History(ctx context.Context, region string, from, to time.Time) ([]domain.HistoryRow, error)
	// Regions lists the regions an organization has carbon in; Origins breaks a window's carbon down by intensity source.
	Regions(ctx context.Context, tenantID string, project *string, from, to time.Time, version string) ([]domain.RegionCarbon, error)
	Origins(ctx context.Context, tenantID string, project *string, from, to time.Time, version string) ([]domain.IntensityOrigin, error)
}

// MonthlyProvider returns a source's published monthly national averages for a cloud region.
type MonthlyProvider interface {
	// Monthly returns the figures for [from, to); Provider on each row is the source id.
	Monthly(ctx context.Context, region string, from, to time.Time) ([]domain.MonthlyRow, error)
}

// HourlyExtra is an additional free source of finer-than-monthly readings for some regions (UK Carbon Intensity).
type HourlyExtra interface {
	// ID is the provider id stored with its readings.
	ID() string
	// Covers reports whether the source has data for the cloud region.
	Covers(ctx context.Context, region string) bool
	// Range returns measured readings in [from, to).
	Range(ctx context.Context, region string, from, to time.Time) ([]domain.Intensity, error)
	// Forecast returns forecast points in [from, to), for planning only.
	Forecast(ctx context.Context, region string, from, to time.Time) ([]domain.Intensity, error)
}

// Ports implemented by other modules and wired in the composition root (no cross-domain imports).
type UsageSource interface {
	DailyAggregates(ctx context.Context, tenantID, projectID string, from, to time.Time) ([]domain.UsageDay, error)
}
type UnitsSource interface {
	FunctionalUnitsForDay(ctx context.Context, tenantID, projectID string, day time.Time) (float64, error)
}
type PolicySource interface {
	CIThresholds(ctx context.Context, tenantID, projectID string) (domain.Thresholds, error)
}
type ProjectRef struct{ TenantID, ProjectID string }
type ProjectLister interface {
	ProjectsForJobs(ctx context.Context) ([]ProjectRef, error)
}

type Service struct {
	Store    Store
	Usage    UsageSource
	Units    UnitsSource
	Policies PolicySource
	Projects ProjectLister
	Provider domain.CarbonDataProvider // nil: live lookups fail closed; stored readings still work
	// Extras are additional sources of measured readings, refreshed with the main provider (optional).
	Extras []HourlyExtra
	// Monthly is the source of monthly national figures (optional).
	Monthly MonthlyProvider
	// Settings supplies the source policy (carbon.* platform settings). nil: catalog defaults.
	Settings settings.Reader
	Queue    queue.Enqueuer
	Log      *slog.Logger
	TTL      time.Duration // live intensity cache (default 1h)

	mu    sync.Mutex
	cache map[string]cachedIntensity
}

type cachedIntensity struct {
	g   float64
	exp time.Time
}

// StoredIntensityAt returns the stored (never provider-fetched) grid reading at or before a time: other modules use it to
// attach the carbon of a past hour without triggering a provider call.
func (s *Service) StoredIntensityAt(ctx context.Context, region string, at time.Time) (float64, bool, error) {
	ref, ok, err := domain.ResolveInstant(ctx, s.Store, s.Policy(ctx), region, at)
	return ref.G, ok, err
}

// Policy is the source policy in force: which source to trust for a region and how much data it needs.
func (s *Service) Policy(ctx context.Context) domain.Policy {
	var r settings.Reader = settings.Defaults{}
	if s.Settings != nil {
		r = s.Settings
	}
	overrides := map[string]string{}
	for _, e := range r.Strings(ctx, "", settings.KeyCarbonOverrides) {
		if region, src, ok := strings.Cut(e, "="); ok {
			overrides[region] = src
		}
	}
	return domain.Policy{
		Priority:       r.Strings(ctx, "", settings.KeyCarbonPriority),
		Overrides:      overrides,
		MinDayReadings: int(r.Int(ctx, "", settings.KeyCarbonMinReadings)),
		MaxAge:         time.Duration(r.Int(ctx, "", settings.KeyCarbonMaxAgeHours)) * time.Hour,
	}
}

// intensityNow returns the current intensity of a region: cached live reading, else the provider (and the
// reading is persisted so history accumulates), else the latest stored reading.
func (s *Service) intensityNow(ctx context.Context, region string) (float64, bool, error) {
	s.mu.Lock()
	if c, ok := s.cache[region]; ok && time.Now().Before(c.exp) {
		s.mu.Unlock()
		return c.g, true, nil
	}
	s.mu.Unlock()
	if s.Provider != nil {
		in, err := s.Provider.GetIntensity(ctx, region)
		if err == nil {
			ttl := s.TTL
			if ttl == 0 {
				ttl = time.Hour
			}
			s.mu.Lock()
			if s.cache == nil {
				s.cache = map[string]cachedIntensity{}
			}
			s.cache[region] = cachedIntensity{g: in.GPerKWh, exp: time.Now().Add(ttl)}
			s.mu.Unlock()
			if err := s.Store.SaveGridIntensity(ctx, domain.ProviderElectricityMaps, region, in.At, in.GPerKWh, false, domain.MeasureAverage, "hourly"); err != nil {
				s.Log.Warn("persist grid intensity", "region", region, "err", err)
			}
			return in.GPerKWh, true, nil
		}
		if !errors.Is(err, domain.ErrNotConfigured) {
			observability.CarbonAPIErrors.Inc()
			s.Log.Warn("grid provider failed; using stored readings", "region", region, "err", err)
		}
	}
	ref, ok, err := domain.ResolveInstant(ctx, s.Store, s.Policy(ctx), region, time.Now())
	return ref.G, ok, err
}

// Calculate converts one measurement in a region to energy and CO2e (live grid intensity).
func (s *Service) Calculate(ctx context.Context, kind domain.UsageKind, amount float64, region string) (domain.Result, error) {
	g, ok, err := s.intensityNow(ctx, region)
	if err != nil {
		return domain.Result{}, err
	}
	if !ok {
		return domain.Result{}, ErrNoGridData
	}
	res, err := domain.Calculate(domain.Input{Kind: kind, Amount: amount, GridIntensityGPerKWh: g})
	if err == nil {
		observability.CarbonCalculations.Inc()
	}
	return res, err
}

// ErrNoGridData: no carbon intensity is known for the region (unsupported region or provider outage with no history).
var ErrNoGridData = errors.New("carbon: no grid intensity data for region")

// WindowResult summarizes one calculation run.
type WindowResult struct {
	Calculated int
	Skipped    map[string]int
}

// CalculateWindow recomputes carbon for [from, to) of a project from stored FOCUS usage. It is idempotent:
// rerunning replaces the window's rows of the same methodology version (new versions add rows, history is kept).
func (s *Service) CalculateWindow(ctx context.Context, tenantID, projectID string, from, to time.Time) (WindowResult, error) {
	return s.CalculateWindowVersion(ctx, tenantID, projectID, from, to, methodology.Version)
}

// ErrUnknownVersion means a methodology version that is not in the registry.
var ErrUnknownVersion = errors.New("unknown methodology version")

// CalculateWindowVersion is CalculateWindow under a chosen coefficient set. Rows of that version only are replaced:
// results of other versions stay as they were, so a new version can be calculated next to the active one and compared.
func (s *Service) CalculateWindowVersion(ctx context.Context, tenantID, projectID string, from, to time.Time, version string) (WindowResult, error) {
	factors, ok := methodology.Get(version)
	if !ok {
		return WindowResult{}, ErrUnknownVersion
	}
	rows, err := s.Usage.DailyAggregates(ctx, tenantID, projectID, from, to)
	if err != nil {
		return WindowResult{}, fmt.Errorf("carbon: read usage: %w", err)
	}
	units := map[time.Time]float64{}
	for _, r := range rows {
		if _, seen := units[r.Day]; seen {
			continue
		}
		u, err := s.Units.FunctionalUnitsForDay(ctx, tenantID, projectID, r.Day)
		if err != nil {
			return WindowResult{}, fmt.Errorf("carbon: functional units: %w", err)
		}
		units[r.Day] = u
	}
	policy := s.Policy(ctx)
	memo := map[string]struct {
		ref domain.IntensityRef
		ok  bool
	}{}
	lookup := func(ctx context.Context, region string, at time.Time) (domain.IntensityRef, bool, error) {
		k := region + "@" + at.Format(time.DateOnly)
		if m, ok := memo[k]; ok {
			return m.ref, m.ok, nil
		}
		// at is the end of the usage day: it ran under the mean of its own readings of the first trusted source that
		// has enough of them (domain.ResolveDay), never under a blend of sources.
		ref, ok, err := domain.ResolveDay(ctx, s.Store, policy, region, at.Add(-24*time.Hour))
		if err == nil && !ok {
			var g float64
			if g, ok, err = s.intensityNow(ctx, region); ok { // nothing stored yet: ask the provider once
				ref = domain.IntensityRef{G: g, Source: domain.ProviderElectricityMaps, Basis: domain.BasisLive}
			}
		}
		if err != nil {
			return domain.IntensityRef{}, false, err
		}
		memo[k] = struct {
			ref domain.IntensityRef
			ok  bool
		}{ref, ok}
		return ref, ok, nil
	}
	calcs, skipped, err := domain.ComputeWith(factors, ctx, rows, lookup, units)
	if err != nil {
		return WindowResult{}, err
	}
	if err := s.Store.ReplaceCalculations(ctx, tenantID, projectID, from, to, factors.Version, calcs); err != nil {
		return WindowResult{}, fmt.Errorf("carbon: persist: %w", err)
	}
	observability.CarbonCalculations.Add(float64(len(calcs)))
	return WindowResult{Calculated: len(calcs), Skipped: skipped}, nil
}

// RunWindowJob is the carbon:calculate job: calculate, then chain the recommendation job.
func (s *Service) RunWindowJob(ctx context.Context, p queue.WindowPayload) error {
	from, err1 := time.Parse(time.DateOnly, p.From)
	to, err2 := time.Parse(time.DateOnly, p.To)
	if err1 != nil || err2 != nil || !to.After(from) || to.Sub(from) > 400*24*time.Hour {
		return fmt.Errorf("carbon: bad window %q..%q", p.From, p.To)
	}
	version := p.MethodologyVersion
	if version == "" {
		version = methodology.Version
	}
	res, err := s.CalculateWindowVersion(ctx, p.TenantID, p.ProjectID, from, to, version)
	if err != nil {
		return err
	}
	if version != methodology.Version {
		return nil // a shadow calculation for comparison: recommendations and dashboards read the active version only
	}
	if len(res.Skipped) > 0 {
		s.Log.Info("carbon rows skipped", "project", p.ProjectID, "skipped", res.Skipped)
	}
	if res.Calculated > 0 {
		return s.Queue.Enqueue(ctx, queue.TaskCalcRecommendation, queue.TenantPayload{TenantID: p.TenantID, ProjectID: p.ProjectID})
	}
	return nil
}

// Recalculate queues a calculation (e.g. after functional units were reported).
func (s *Service) Recalculate(ctx context.Context, tenantID, projectID string, from, to time.Time) error {
	return s.RecalculateVersion(ctx, tenantID, projectID, from, to, "")
}

// RecalculateVersion queues a calculation under a methodology version ("" = the active one).
func (s *Service) RecalculateVersion(ctx context.Context, tenantID, projectID string, from, to time.Time, version string) error {
	if version != "" {
		if _, ok := methodology.Get(version); !ok {
			return ErrUnknownVersion
		}
	}
	return s.Queue.Enqueue(ctx, queue.TaskCalculateCarbon, queue.WindowPayload{
		TenantID: tenantID, ProjectID: projectID, From: from.Format(time.DateOnly), To: to.Format(time.DateOnly), MethodologyVersion: version})
}

// VersionSide is one methodology version's result over the compared window.
type VersionSide struct {
	Version    string        `json:"version"`
	Status     string        `json:"status"`
	Provenance string        `json:"provenance"`
	HasData    bool          `json:"has_data"` // false: this version has not been calculated for the window
	Totals     domain.Totals `json:"-"`
}

// Comparison puts two methodology versions side by side over the same window and scope.
type Comparison struct {
	A, B VersionSide
	// DeltaCO2eKg and DeltaPct are B relative to A; nil unless both versions have the same days of data.
	DeltaCO2eKg *float64
	DeltaPct    *float64
	Note        string
}

// Compare reads the stored results of two versions. It never calculates: a version that has not been run for the
// window says so (run POST /carbon/recalculate with that version first), it is not estimated here.
func (s *Service) Compare(ctx context.Context, tenantID string, project *string, from, to time.Time, a, b string) (Comparison, error) {
	side := func(v string) (VersionSide, error) {
		f, ok := methodology.Get(v)
		if !ok {
			return VersionSide{}, ErrUnknownVersion
		}
		t, err := s.Store.VersionTotals(ctx, tenantID, project, from, to, v)
		if err != nil {
			return VersionSide{}, err
		}
		return VersionSide{Version: v, Status: f.Status, Provenance: f.Provenance, HasData: t.Rows > 0, Totals: t}, nil
	}
	var c Comparison
	var err error
	if c.A, err = side(a); err != nil {
		return c, err
	}
	if c.B, err = side(b); err != nil {
		return c, err
	}
	switch {
	case !c.A.HasData || !c.B.HasData:
		c.Note = "At least one version has no results for this window; recalculate it under that version first."
	case c.A.Totals.Days != c.B.Totals.Days:
		c.Note = "The versions cover a different number of days, so they are not compared."
	default:
		d := c.B.Totals.CO2eKg - c.A.Totals.CO2eKg
		c.DeltaCO2eKg = &d
		if c.A.Totals.CO2eKg > 0 {
			p := d / c.A.Totals.CO2eKg * 100
			c.DeltaPct = &p
		}
	}
	return c, nil
}

// RecalculateAll queues the last 35 days of every project (scheduler): picks up newly reported functional units.
func (s *Service) RecalculateAll(ctx context.Context) (int, error) {
	refs, err := s.Projects.ProjectsForJobs(ctx)
	if err != nil {
		return 0, err
	}
	to := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	for _, r := range refs {
		if err := s.Recalculate(ctx, r.TenantID, r.ProjectID, to.AddDate(0, 0, -35), to); err != nil {
			s.Log.Error("enqueue recalculation", "project", r.ProjectID, "err", err)
		}
	}
	return len(refs), nil
}

func (s *Service) Summary(ctx context.Context, tenantID string, project *string, from, to time.Time) (domain.Summary, error) {
	return s.Store.Summary(ctx, tenantID, project, from, to)
}

func (s *Service) Trend(ctx context.Context, tenantID string, project *string, from, to time.Time) ([]domain.TrendPoint, error) {
	return s.Store.Trend(ctx, tenantID, project, from, to)
}

// RefreshGrid persists the latest intensity of every region the main provider covers, so dashboards read local data,
// and the last hours of every additional source for the regions in use. One failing source does not stop the others.
func (s *Service) RefreshGrid(ctx context.Context) error {
	firstErr := s.refreshMain(ctx)
	if err := s.refreshExtras(ctx, 6*time.Hour); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (s *Service) refreshMain(ctx context.Context) error {
	lister, ok := s.Provider.(domain.RegionLister)
	if !ok || len(lister.Regions()) == 0 {
		s.Log.Warn("grid refresh skipped: no provider configured")
		return nil
	}
	var firstErr error
	for _, region := range lister.Regions() {
		in, err := s.Provider.GetIntensity(ctx, region)
		if err == nil {
			err = s.Store.SaveGridIntensity(ctx, domain.ProviderElectricityMaps, region, in.At, in.GPerKWh, false, domain.MeasureAverage, "hourly")
		}
		if err == nil {
			s.saveForecast(ctx, region)
		}
		if err != nil {
			observability.CarbonAPIErrors.Inc()
			s.Log.Error("grid refresh", "region", region, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// refreshExtras stores the last `window` of every additional source for the regions in use that it covers.
func (s *Service) refreshExtras(ctx context.Context, window time.Duration) error {
	if len(s.Extras) == 0 {
		return nil
	}
	regions, err := s.Store.RegionsInUse(ctx)
	if err != nil {
		return err
	}
	to := time.Now().UTC()
	first := s.loadExtras(ctx, regions, to.Add(-window), to)
	// Forecasts of the next two days for planning (a failing forecast never fails the refresh of readings).
	for _, x := range s.Extras {
		for _, region := range regions {
			if !x.Covers(ctx, region) {
				continue
			}
			pts, err := x.Forecast(ctx, region, to, to.Add(48*time.Hour))
			if err != nil {
				s.Log.Warn("grid forecast", "source", x.ID(), "region", region, "err", err)
				continue
			}
			for _, p := range pts {
				if e := s.Store.SaveGridIntensity(ctx, x.ID(), region, p.At, p.GPerKWh, true, domain.MeasureAverage, "hourly"); e != nil {
					return e
				}
			}
		}
	}
	return first
}

// loadExtras fetches [from, to) from every additional source for the regions it covers and stores the readings.
func (s *Service) loadExtras(ctx context.Context, regions []string, from, to time.Time) error {
	var first error
	for _, x := range s.Extras {
		for _, region := range regions {
			if !x.Covers(ctx, region) {
				continue
			}
			pts, err := x.Range(ctx, region, from, to)
			if err != nil {
				observability.CarbonAPIErrors.Inc()
				s.Log.Warn("grid source", "source", x.ID(), "region", region, "err", err)
				if first == nil {
					first = err
				}
				if errors.Is(err, apiusage.ErrBudget) {
					break
				}
				continue
			}
			for _, p := range pts {
				if e := s.Store.SaveGridIntensity(ctx, x.ID(), region, p.At, p.GPerKWh, false, domain.MeasureAverage, "hourly"); e != nil {
					return e
				}
			}
		}
	}
	return first
}

// saveForecast persists the provider forecast for a region. A forecast is optional (not every plan includes
// it), so a failure is logged and never fails the refresh of the current readings.
func (s *Service) saveForecast(ctx context.Context, region string) {
	f, err := s.Provider.GetForecast(ctx, region)
	if err != nil {
		s.Log.Warn("grid forecast unavailable", "region", region, "err", err)
		return
	}
	for _, p := range f.Points {
		if err := s.Store.SaveGridIntensity(ctx, domain.ProviderElectricityMaps, region, p.At, p.GPerKWh, true, domain.MeasureAverage, "hourly"); err != nil {
			s.Log.Error("save grid forecast", "region", region, "err", err)
			return
		}
	}
}

// ---- CI gate ----

// Change is one planned infrastructure change, expressed per month.
type Change struct {
	Kind             domain.UsageKind `json:"kind"`
	Amount           float64          `json:"amount"`
	Region           string           `json:"region"`
	MonthlyCostDelta float64          `json:"monthly_cost_delta"`
}

type GateRequest struct {
	ProjectID  string           `json:"project_id"`
	Changes    []Change         `json:"changes"`
	Thresholds *ThresholdsInput `json:"thresholds"`
}

// ThresholdsInput overrides project policy values field by field.
type ThresholdsInput struct {
	CarbonWarnKg  *float64 `json:"carbon_warn_kg"`
	CarbonBlockKg *float64 `json:"carbon_block_kg"`
	CostWarn      *float64 `json:"cost_warn"`
	CostBlock     *float64 `json:"cost_block"`
}

type GateResponse struct {
	Verdict            domain.Verdict `json:"verdict"`
	CarbonKgMonth      float64        `json:"carbon_kg_month"`
	CostDeltaMonth     float64        `json:"cost_delta_month"`
	Reasons            []string       `json:"reasons"`
	ThresholdsApplied  bool           `json:"thresholds_applied"`
	MethodologyVersion string         `json:"methodology_version"`
	// EmbodiedCarbonIncluded is false while the methodology leaves out embodied emissions: the verdict is based
	// on operational emissions only.
	EmbodiedCarbonIncluded bool     `json:"embodied_carbon_included"`
	Caveats                []string `json:"caveats"`
}

var ErrInvalidGateRequest = errors.New("invalid gate request")

// Evaluate estimates the carbon and cost impact of a planned change and applies PASS/WARN/BLOCK thresholds
// (request values override the project's policy).
func (s *Service) Evaluate(ctx context.Context, tenantID string, req GateRequest) (GateResponse, error) {
	if len(req.Changes) == 0 || len(req.Changes) > 200 {
		return GateResponse{}, fmt.Errorf("%w: 1-200 changes required", ErrInvalidGateRequest)
	}
	var carbonKg, cost float64
	for i, c := range req.Changes {
		if c.Region == "" {
			return GateResponse{}, fmt.Errorf("%w: change %d has no region", ErrInvalidGateRequest, i)
		}
		res, err := s.Calculate(ctx, c.Kind, c.Amount, c.Region)
		if errors.Is(err, domain.ErrInvalidInput) {
			return GateResponse{}, fmt.Errorf("%w: change %d: %v", ErrInvalidGateRequest, i, err)
		}
		if err != nil {
			return GateResponse{}, err
		}
		carbonKg += res.CO2eKg
		cost += c.MonthlyCostDelta
	}
	th, err := s.Policies.CIThresholds(ctx, tenantID, req.ProjectID)
	if err != nil {
		return GateResponse{}, err
	}
	if o := req.Thresholds; o != nil {
		if o.CarbonWarnKg != nil {
			th.CarbonWarnKg = o.CarbonWarnKg
		}
		if o.CarbonBlockKg != nil {
			th.CarbonBlockKg = o.CarbonBlockKg
		}
		if o.CostWarn != nil {
			th.CostWarn = o.CostWarn
		}
		if o.CostBlock != nil {
			th.CostBlock = o.CostBlock
		}
	}
	v, reasons := domain.Gate(carbonKg, cost, th)
	return GateResponse{Verdict: v, CarbonKgMonth: carbonKg, CostDeltaMonth: cost, Reasons: reasons, ThresholdsApplied: th.Any(),
		MethodologyVersion: domain.MethodologyVersion, EmbodiedCarbonIncluded: domain.Current.EmbodiedIncluded(), Caveats: domain.Current.Caveats}, nil
}

// IntensityHistory returns the day-by-day grid intensity of a region from stored readings. A day with no reading is
// absent, never filled in.
func (s *Service) IntensityHistory(ctx context.Context, region string, from, to time.Time) ([]domain.HistoryPoint, error) {
	rows, err := s.Store.History(ctx, region, from, to)
	if err != nil {
		return nil, err
	}
	monthly, err := s.Store.MonthlyHistory(ctx, region, from, to)
	if err != nil {
		return nil, err
	}
	return domain.ResolveHistory(rows, monthly, s.Policy(ctx), region, from, to), nil
}

// Regions lists the regions an organization has carbon in over a window.
func (s *Service) Regions(ctx context.Context, tenantID string, project *string, from, to time.Time) ([]domain.RegionCarbon, error) {
	return s.Store.Regions(ctx, tenantID, project, from, to, methodology.Version)
}

// Origins says where the intensities behind a window's carbon came from.
func (s *Service) Origins(ctx context.Context, tenantID string, project *string, from, to time.Time) ([]domain.IntensityOrigin, error) {
	return s.Store.Origins(ctx, tenantID, project, from, to, methodology.Version)
}

// Backfill limits: the provider bills and throttles by request, so a run is bounded before it starts.
const (
	BackfillMaxDays     = 400
	BackfillMaxRequests = 2000
	backfillChunkDays   = 9
)

// BackfillEstimate is how many provider requests a backfill of regions over days needs.
func BackfillEstimate(regions, days int) int {
	chunks := (days + backfillChunkDays - 1) / backfillChunkDays
	return regions * chunks
}

// BackfillIntensity fetches the past hourly readings of the regions from the provider and stores them (idempotent: a
// stored reading is never overwritten). It does nothing without a provider that offers history. One region failing
// does not stop the others; the first error is returned after all were tried.
func (s *Service) BackfillIntensity(ctx context.Context, regions []string, from, to time.Time) (stored int, err error) {
	if days := int(to.Sub(from).Hours() / 24); days < 1 || days > BackfillMaxDays || BackfillEstimate(len(regions), days) > BackfillMaxRequests {
		return 0, fmt.Errorf("carbon: backfill of %d regions over %d days is outside the limits", len(regions), days)
	}
	var first error
	// The additional free sources (UK Carbon Intensity) load their past without a key or a plan.
	if err := s.loadExtras(ctx, regions, from, to); err != nil {
		first = err
	}
	hp, ok := s.Provider.(domain.HistoryProvider)
	if !ok {
		if first == nil && len(s.Extras) == 0 {
			first = errors.New("carbon: the grid provider cannot return past data")
		}
		return 0, first
	}
	for _, region := range regions {
		pts, e := hp.GetHistory(ctx, region, from, to)
		if e != nil {
			observability.CarbonAPIErrors.Inc()
			s.Log.Error("grid backfill", "region", region, "err", e)
			if first == nil {
				first = e
			}
			if errors.Is(e, domain.ErrNotConfigured) {
				break // no key: every region would fail the same way
			}
			continue
		}
		for _, p := range pts {
			if e := s.Store.SaveGridIntensity(ctx, domain.ProviderElectricityMaps, region, p.At, p.GPerKWh, false, domain.MeasureAverage, "hourly"); e != nil {
				return stored, e
			}
			stored++
		}
	}
	return stored, first
}

// RefreshMonthly stores the last 36 months of monthly national figures for every region in use. A figure that was
// published again is replaced (monthly values are revised); hourly readings are never touched. Regions of one country
// share one provider request (the provider adapter caches per run).
func (s *Service) RefreshMonthly(ctx context.Context) (stored int, err error) {
	if s.Monthly == nil {
		return 0, nil
	}
	regions, err := s.Store.RegionsInUse(ctx)
	if err != nil {
		return 0, err
	}
	to := time.Now().UTC()
	from := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -36, 0)
	var first error
	for _, region := range regions {
		rows, e := s.Monthly.Monthly(ctx, region, from, to)
		if e != nil {
			if first == nil {
				first = e
			}
			s.Log.Warn("monthly grid data", "region", region, "err", e)
			if errors.Is(e, domain.ErrNotConfigured) || errors.Is(e, apiusage.ErrBudget) {
				break // every region would fail the same way
			}
			continue
		}
		for _, r := range rows {
			if e := s.Store.SaveGridIntensity(ctx, r.Provider, region, r.Month, r.G, false, domain.MeasureAverage, "monthly"); e != nil {
				return stored, e
			}
			stored++
		}
	}
	return stored, first
}

// minCoverage is the share of days that must have grid data before a region is left alone.
const minCoverage = 0.9

// AutoLoadResult says what the daily automatic loading did.
type AutoLoadResult struct {
	Regions    []string // regions whose past was loaded
	Recalc     int      // projects queued for recalculation
	WindowFrom time.Time
	WindowTo   time.Time
}

// AutoLoadHistory is the daily automation that makes a new or gappy region right without anyone asking: for regions in use
// whose last N days are not (90%) covered by grid data it loads the past from every source that can supply it (the
// additional free sources, the main provider's past data when its plan has it, the monthly national figures) and then
// queues a recalculation of every project over the window, so the stored carbon uses the data that now exists.
// Requests stay under the daily cap and the backfill limits; a failing source is logged and the others still run.
func (s *Service) AutoLoadHistory(ctx context.Context) (AutoLoadResult, error) {
	var res AutoLoadResult
	var r settings.Reader = settings.Defaults{}
	if s.Settings != nil {
		r = s.Settings
	}
	if !r.Bool(ctx, "", settings.KeyCarbonAutoLoad) {
		return res, nil
	}
	days := int(r.Int(ctx, "", settings.KeyCarbonAutoDays))
	if days < 7 {
		days = 90
	}
	cov, err := s.Store.RegionCoverage(ctx, days)
	if err != nil {
		return res, err
	}
	hourly, err := s.Store.RegionHourlyCoverage(ctx, days)
	if err != nil {
		return res, err
	}
	hourlyOf := map[string]domain.RegionCoverage{}
	for _, h := range hourly {
		hourlyOf[h.Region] = h
	}
	for _, c := range cov {
		if c.Total <= 0 {
			continue
		}
		need := float64(c.Covered)/float64(c.Total) < minCoverage
		// A monthly figure alone does not settle a region that a free hourly source covers: that source has the past.
		if h, ok := hourlyOf[c.Region]; !need && ok && h.Total > 0 && float64(h.Covered)/float64(h.Total) < minCoverage && s.extraCovers(ctx, c.Region) {
			need = true
		}
		if need {
			res.Regions = append(res.Regions, c.Region)
		}
	}
	if len(res.Regions) == 0 {
		return res, nil
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	res.WindowFrom, res.WindowTo = today.AddDate(0, 0, -days), today
	_, first := s.BackfillIntensity(ctx, res.Regions, res.WindowFrom, res.WindowTo)
	if errors.Is(first, domain.ErrPastUnavailable) {
		s.Log.Info("main grid provider has no past data on this plan; filling the past from the other sources")
		first = nil
	}
	if _, err := s.RefreshMonthly(ctx); err != nil && first == nil {
		first = err
	}
	n, err := s.recalculateAll(ctx, res.WindowFrom, res.WindowTo.AddDate(0, 0, 1))
	res.Recalc = n
	if err != nil && first == nil {
		first = err
	}
	return res, first
}

// extraCovers reports whether an additional free hourly source can supply the region.
func (s *Service) extraCovers(ctx context.Context, region string) bool {
	for _, x := range s.Extras {
		if x.Covers(ctx, region) {
			return true
		}
	}
	return false
}

// RecalculateEverything queues a recalculation of [from, to) for every project (operator action).
func (s *Service) RecalculateEverything(ctx context.Context, from, to time.Time) (int, error) {
	return s.recalculateAll(ctx, from, to)
}

// recalculateAll queues a recalculation of [from, to) for every project that has jobs.
func (s *Service) recalculateAll(ctx context.Context, from, to time.Time) (int, error) {
	refs, err := s.Projects.ProjectsForJobs(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range refs {
		if err := s.Recalculate(ctx, r.TenantID, r.ProjectID, from, to); err != nil {
			s.Log.Error("enqueue recalculation", "project", r.ProjectID, "err", err)
			continue
		}
		n++
	}
	return n, nil
}

// ScheduleAdvice says when to start a flexible job of `duration` within `within` from now so that the grid's average
// intensity is lowest, from stored forecasts. The first source in the policy order that has a forecast for the region is
// used (sources are never blended). It is advice from a forecast of average intensity, not a guarantee.
func (s *Service) ScheduleAdvice(ctx context.Context, region string, duration, within time.Duration) (domain.BestWindow, string, error) {
	now := time.Now().UTC()
	to := now.Add(within)
	for _, src := range s.Policy(ctx).Order(region) {
		pts, err := s.Store.Forecast(ctx, region, src, now.Add(-time.Hour), to.Add(time.Hour))
		if err != nil {
			return domain.BestWindow{}, "", err
		}
		if len(pts) == 0 {
			continue
		}
		w, err := domain.ChooseWindow(pts, now, to, duration)
		if err == nil {
			return w, src, nil
		}
		if !errors.Is(err, domain.ErrNoForecast) {
			return domain.BestWindow{}, "", err
		}
	}
	return domain.BestWindow{}, "", domain.ErrNoForecast
}
