// Package http exposes the carbon endpoints and the CI gate.
package http

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/methodology"
)

type Handlers struct{ Svc *application.Service }

var regionID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,38}$`)

func (h Handlers) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermCarbonRead)).Get("/carbon/summary", h.summary)
	r.With(auth.Require(auth.PermCarbonRead)).Get("/carbon/trend", h.trend)
	r.With(auth.Require(auth.PermCarbonRead)).Get("/carbon/intensity", h.intensity)
	r.With(auth.Require(auth.PermCarbonRead)).Get("/carbon/schedule", h.schedule)
	r.With(auth.Require(auth.PermCarbonRead)).Get("/carbon/methodology", h.methodology)
	r.With(auth.Require(auth.PermCarbonRead)).Get("/carbon/methodology/versions", h.versions)
	r.With(auth.Require(auth.PermCarbonRead)).Get("/carbon/compare", h.compare)
	r.With(auth.Require(auth.PermCarbonCompute)).Post("/carbon/calculate", h.calculate)
	r.With(auth.Require(auth.PermCarbonCompute)).Post("/carbon/recalculate", h.recalculate)
	// CI gate: reachable with an API key (role ci) as well as by people who may compute carbon.
	r.With(auth.Require(auth.PermCIEvaluate)).Post("/ci/evaluate", h.evaluate)
	r.With(auth.Require(auth.PermCIEvaluate)).Get("/ci/schedule", h.schedule)
	r.With(auth.Require(auth.PermCarbonRead)).Get("/ci/evaluations", h.evaluations)
}

type totalsJSON struct {
	EnergyKWh        float64  `json:"energy_kwh"`
	CarbonKgCO2e     float64  `json:"carbon_kg_co2e"`
	SCIScore         *float64 `json:"sci_score"` // null until functional units are reported
	IntensityGPerKWh float64  `json:"carbon_intensity_g_per_kwh"`
}

func toTotals(t domain.Totals) totalsJSON {
	return totalsJSON{EnergyKWh: t.EnergyKWh, CarbonKgCO2e: t.CO2eKg, SCIScore: t.SCI, IntensityGPerKWh: t.IntensityGPerKWh}
}

func query(w http.ResponseWriter, r *http.Request) (from, to time.Time, project *string, ok bool) {
	from, to, err := httpx.Period(r)
	project, okp := httpx.OptionalUUID(r, "project_id")
	if err != nil || !okp {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "")
		return from, to, nil, false
	}
	return from, to, project, true
}

func (h Handlers) summary(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, project, ok := query(w, r)
	if !ok {
		return
	}
	s, err := h.Svc.Summary(r.Context(), c.TenantID, project, from, to)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	rawOrigins, err := h.Svc.Origins(r.Context(), c.TenantID, project, from, to)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	origins, measured := toOrigins(rawOrigins)
	httpx.WriteJSON(w, http.StatusOK, struct {
		totalsJSON
		HasData            bool       `json:"has_data"`
		Previous           totalsJSON `json:"previous"`
		Comparable         bool       `json:"comparable"` // false: windows not both >= 80% covered, so no delta may be shown
		MethodologyVersion string     `json:"methodology_version"`
		// Methodology travels with the figures: a number must never look more complete than it is.
		Methodology methodologyJSON `json:"methodology"`
		// IntensityOrigins says where the grid intensities behind the carbon came from (source and basis).
		IntensityOrigins []originJSON `json:"intensity_origins"`
		// MeasuredSharePct is the share of the carbon whose intensity is the mean of readings of the day itself.
		MeasuredSharePct *float64 `json:"intensity_measured_share_pct"`
	}{toTotals(s.Current), s.HasData(), toTotals(s.Previous),
		domain.Comparable(s.Current.Days, s.Previous.Days, int(to.Sub(from).Hours()/24+0.5)), domain.MethodologyVersion, describeMethodology(),
		origins, measured})
}

type originJSON struct {
	Source   string  `json:"source"` // "" = calculated before the origin was recorded
	Basis    string  `json:"basis"`
	CarbonKg float64 `json:"carbon_kg_co2e"`
	SharePct float64 `json:"share_pct"`
}

// toOrigins turns the breakdown into shares of the window's total and the share that is a same-day mean.
func toOrigins(in []domain.IntensityOrigin) (out []originJSON, measured *float64) {
	var total, day float64
	for _, o := range in {
		total += o.CarbonKg
		if o.Basis == domain.BasisDayMean {
			day += o.CarbonKg
		}
	}
	out = []originJSON{}
	for _, o := range in {
		share := 0.0
		if total > 0 {
			share = o.CarbonKg / total * 100
		}
		out = append(out, originJSON{Source: o.Source, Basis: o.Basis, CarbonKg: o.CarbonKg, SharePct: share})
	}
	if total > 0 {
		m := day / total * 100
		measured = &m
	}
	return out, measured
}

type intensityJSON struct {
	Region  string                `json:"region"`
	Regions []regionJSON          `json:"regions"`
	Points  []domain.HistoryPoint `json:"points"`
	// Policy is the source policy the points were chosen under.
	Policy struct {
		Priority       []string `json:"priority"`
		MinDayReadings int      `json:"min_day_readings"`
		MaxAgeHours    int      `json:"max_age_hours"`
	} `json:"policy"`
	WindowDays   int `json:"window_days"`
	DaysWithData int `json:"days_with_data"`
}

type regionJSON struct {
	Region   string  `json:"region"`
	CarbonKg float64 `json:"carbon_kg_co2e"`
}

// intensity answers the history of the grid carbon intensity of one region (default: the organization's largest
// emitter), from stored readings, under the source policy. Gaps stay gaps.
func (h Handlers) intensity(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, project, ok := query(w, r)
	if !ok {
		return
	}
	regions, err := h.Svc.Regions(r.Context(), c.TenantID, project, from, to)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	out := intensityJSON{Regions: []regionJSON{}, Points: []domain.HistoryPoint{}}
	for _, x := range regions {
		out.Regions = append(out.Regions, regionJSON{Region: x.Region, CarbonKg: x.CarbonKg})
	}
	region := r.URL.Query().Get("region")
	if region == "" && len(regions) > 0 {
		region = regions[0].Region
	}
	if !regionID.MatchString(region) && region != "" {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "region must be a cloud region id")
		return
	}
	out.Region = region
	pol := h.Svc.Policy(r.Context())
	out.Policy.Priority, out.Policy.MinDayReadings, out.Policy.MaxAgeHours = pol.Priority, pol.MinDayReadings, int(pol.MaxAge.Hours())
	out.WindowDays = int(to.Sub(from).Hours()/24 + 0.5)
	if region != "" {
		if out.Points, err = h.Svc.IntensityHistory(r.Context(), region, from, to); err != nil {
			httpx.Internal(w, r, err)
			return
		}
		out.DaysWithData = len(out.Points)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// methodologyJSON is what a client needs to qualify the figures it shows.
type methodologyJSON struct {
	Version                string   `json:"version"`
	Status                 string   `json:"status"`
	Provenance             string   `json:"provenance"`
	EmbodiedCarbonIncluded bool     `json:"embodied_carbon_included"`
	Caveats                []string `json:"caveats"`
}

func describeMethodology() methodologyJSON {
	c := domain.Current
	return methodologyJSON{Version: c.Version, Status: c.Status, Provenance: c.Provenance, EmbodiedCarbonIncluded: c.EmbodiedIncluded(), Caveats: c.Caveats}
}

func (h Handlers) trend(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, project, ok := query(w, r)
	if !ok {
		return
	}
	t, err := h.Svc.Trend(r.Context(), c.TenantID, project, from, to)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": t})
}

// methodology publishes the active coefficient set: users can see exactly how numbers are produced.
func (h Handlers) methodology(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, domain.Current)
}

// versions lists every coefficient set the platform knows, so a customer can see which one produced which result.
func (h Handlers) versions(w http.ResponseWriter, r *http.Request) {
	type item struct {
		Version    string `json:"version"`
		Status     string `json:"status"`
		Provenance string `json:"provenance"`
		Active     bool   `json:"active"`
	}
	out := []item{}
	for _, f := range methodology.Versions() {
		out = append(out, item{Version: f.Version, Status: f.Status, Provenance: f.Provenance, Active: f.Version == methodology.Version})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

// compare puts two versions' stored results side by side; it never calculates anything.
func (h Handlers) compare(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, project, ok := query(w, r)
	if !ok {
		return
	}
	a, b := r.URL.Query().Get("a"), r.URL.Query().Get("b")
	if a == "" || b == "" {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "a and b (methodology versions) are required")
		return
	}
	cmp, err := h.Svc.Compare(r.Context(), c.TenantID, project, from, to, a, b)
	if errors.Is(err, application.ErrUnknownVersion) {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "unknown methodology version", "")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	side := func(s application.VersionSide) map[string]any {
		return map[string]any{"version": s.Version, "status": s.Status, "provenance": s.Provenance, "has_data": s.HasData,
			"days": s.Totals.Days, "totals": toTotals(s.Totals)}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"a": side(cmp.A), "b": side(cmp.B),
		"delta_carbon_kg_co2e": cmp.DeltaCO2eKg, "delta_pct": cmp.DeltaPct, "note": cmp.Note})
}

func (h Handlers) calculate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind   domain.UsageKind `json:"kind"`
		Amount float64          `json:"amount"`
		Region string           `json:"region"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil || req.Region == "" {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	res, err := h.Svc.Calculate(r.Context(), req.Kind, req.Amount, req.Region)
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid input", err.Error())
	case errors.Is(err, application.ErrNoGridData):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "no grid data for region", "")
	case err != nil:
		httpx.WriteProblem(w, r, http.StatusBadGateway, "carbon data unavailable", "")
	default:
		httpx.WriteJSON(w, http.StatusOK, res)
	}
}

func (h Handlers) recalculate(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		ProjectID string `json:"project_id"`
		From      string `json:"from"`
		To        string `json:"to"`
		// MethodologyVersion calculates under another coefficient set (a shadow run for comparison); empty = active.
		MethodologyVersion string `json:"methodology_version"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || !httpx.ParseUUID(in.ProjectID) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	from, err1 := time.Parse(time.DateOnly, in.From)
	to, err2 := time.Parse(time.DateOnly, in.To)
	if err1 != nil || err2 != nil || !to.After(from) || to.Sub(from) > 400*24*time.Hour {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid period", "from/to must be YYYY-MM-DD, to after from, at most 400 days")
		return
	}
	if err := h.Svc.RecalculateVersion(r.Context(), c.TenantID, in.ProjectID, from, to, in.MethodologyVersion); err != nil {
		if errors.Is(err, application.ErrUnknownVersion) {
			httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "unknown methodology version", "")
			return
		}
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (h Handlers) evaluate(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var req application.GateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || !httpx.ParseUUID(req.ProjectID) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "project_id must be a UUID")
		return
	}
	res, err := h.Svc.Evaluate(r.Context(), c.TenantID, req)
	switch {
	case errors.Is(err, application.ErrInvalidGateRequest):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", err.Error())
	case errors.Is(err, application.ErrNoGridData):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "no grid data for region", "")
	case err != nil:
		httpx.WriteProblem(w, r, http.StatusBadGateway, "carbon data unavailable", "")
	default:
		source := "console"
		if c.KeyID != "" {
			source = "api_key"
		}
		h.Svc.RecordGate(r.Context(), c.TenantID, source, req, res) // best effort: never changes the answer
		httpx.WriteJSON(w, http.StatusOK, res)
	}
}

// evaluations lists the recorded CI gate decisions of a window with the counts by verdict.
func (h Handlers) evaluations(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, project, ok := query(w, r)
	if !ok {
		return
	}
	verdict := r.URL.Query().Get("verdict")
	if verdict != "" && verdict != "PASS" && verdict != "WARN" && verdict != "BLOCK" {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "verdict must be PASS, WARN or BLOCK")
		return
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 500 {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "limit must be 1-500")
			return
		}
		limit = n
	}
	items, sum, err := h.Svc.GateHistory(r.Context(), c.TenantID, project, verdict, from, to, limit)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "summary": sum})
}

// schedule answers when to start a flexible job (CI pipeline, backup, batch) to use the cleanest grid hours, from stored
// forecasts. Parameters: region, duration_hours (1-24, default 1), within_hours (duration-48, default 24).
func (h Handlers) schedule(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	region := q.Get("region")
	atoi := func(key string, def int) (int, bool) {
		v := q.Get(key)
		if v == "" {
			return def, true
		}
		n, err := strconv.Atoi(v)
		return n, err == nil
	}
	dur, ok1 := atoi("duration_hours", 1)
	within, ok2 := atoi("within_hours", 24)
	if !regionID.MatchString(region) || !ok1 || !ok2 || dur < 1 || dur > 24 || within < dur || within > 48 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "region (a cloud region id), duration_hours 1-24 and within_hours from duration up to 48 are required")
		return
	}
	win, src, err := h.Svc.ScheduleAdvice(r.Context(), region, time.Duration(dur)*time.Hour, time.Duration(within)*time.Hour)
	if errors.Is(err, domain.ErrNoForecast) {
		httpx.WriteProblem(w, r, http.StatusNotFound, "no forecast", "There is no forecast stored for this region and window yet.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"region": region, "source": src, "window": win,
		"basis": "forecast of average grid intensity; advice, not a guarantee",
	})
}
