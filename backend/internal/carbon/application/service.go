// Package application holds the carbon use-cases: calculation pipeline, summaries and the CI gate.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

// Store persists calculations and reads aggregates (implemented by repository.Postgres).
type Store interface {
	Summary(ctx context.Context, tenantID string, project *string, from, to time.Time) (domain.Summary, error)
	Trend(ctx context.Context, tenantID string, project *string, from, to time.Time) ([]domain.TrendPoint, error)
	ReplaceCalculations(ctx context.Context, tenantID, projectID string, from, to time.Time, calcs []domain.Calculation) error
	SaveGridIntensity(ctx context.Context, provider, region string, at time.Time, gPerKWh float64, forecast bool) error
	IntensityAt(ctx context.Context, region string, ts time.Time) (float64, bool, error)
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
			if err := s.Store.SaveGridIntensity(ctx, domain.ProviderElectricityMaps, region, in.At, in.GPerKWh, false); err != nil {
				s.Log.Warn("persist grid intensity", "region", region, "err", err)
			}
			return in.GPerKWh, true, nil
		}
		observability.CarbonAPIErrors.Inc()
		s.Log.Warn("grid provider failed; using stored readings", "region", region, "err", err)
	}
	return s.Store.IntensityAt(ctx, region, time.Now())
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
	memo := map[string]struct {
		g  float64
		ok bool
	}{}
	lookup := func(ctx context.Context, region string, at time.Time) (float64, bool, error) {
		k := region + "@" + at.Format(time.DateOnly)
		if m, ok := memo[k]; ok {
			return m.g, m.ok, nil
		}
		g, ok, err := s.Store.IntensityAt(ctx, region, at)
		if err == nil && !ok {
			g, ok, err = s.intensityNow(ctx, region) // nothing stored yet: ask the provider once
		}
		if err != nil {
			return 0, false, err
		}
		memo[k] = struct {
			g  float64
			ok bool
		}{g, ok}
		return g, ok, nil
	}
	calcs, skipped, err := domain.Compute(ctx, rows, lookup, units)
	if err != nil {
		return WindowResult{}, err
	}
	if err := s.Store.ReplaceCalculations(ctx, tenantID, projectID, from, to, calcs); err != nil {
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
	res, err := s.CalculateWindow(ctx, p.TenantID, p.ProjectID, from, to)
	if err != nil {
		return err
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
	return s.Queue.Enqueue(ctx, queue.TaskCalculateCarbon, queue.WindowPayload{
		TenantID: tenantID, ProjectID: projectID, From: from.Format(time.DateOnly), To: to.Format(time.DateOnly)})
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

// RefreshGrid persists the latest intensity of every region the provider covers, so dashboards read local data.
func (s *Service) RefreshGrid(ctx context.Context) error {
	lister, ok := s.Provider.(domain.RegionLister)
	if !ok {
		s.Log.Warn("grid refresh skipped: no provider configured")
		return nil
	}
	var firstErr error
	for _, region := range lister.Regions() {
		in, err := s.Provider.GetIntensity(ctx, region)
		if err == nil {
			err = s.Store.SaveGridIntensity(ctx, domain.ProviderElectricityMaps, region, in.At, in.GPerKWh, false)
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

// saveForecast persists the provider forecast for a region. A forecast is optional (not every plan includes
// it), so a failure is logged and never fails the refresh of the current readings.
func (s *Service) saveForecast(ctx context.Context, region string) {
	f, err := s.Provider.GetForecast(ctx, region)
	if err != nil {
		s.Log.Warn("grid forecast unavailable", "region", region, "err", err)
		return
	}
	for _, p := range f.Points {
		if err := s.Store.SaveGridIntensity(ctx, domain.ProviderElectricityMaps, region, p.At, p.GPerKWh, true); err != nil {
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
