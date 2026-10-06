// Package http exposes the carbon endpoints and the CI gate.
package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

type Handlers struct{ Svc *application.Service }

func (h Handlers) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermCarbonRead)).Get("/carbon/summary", h.summary)
	r.With(auth.Require(auth.PermCarbonRead)).Get("/carbon/trend", h.trend)
	r.With(auth.Require(auth.PermCarbonRead)).Get("/carbon/methodology", h.methodology)
	r.With(auth.Require(auth.PermCarbonCompute)).Post("/carbon/calculate", h.calculate)
	r.With(auth.Require(auth.PermCarbonCompute)).Post("/carbon/recalculate", h.recalculate)
	// CI gate: reachable with an API key (role ci) as well as by people who may compute carbon.
	r.With(auth.Require(auth.PermCIEvaluate)).Post("/ci/evaluate", h.evaluate)
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
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, struct {
		totalsJSON
		HasData            bool       `json:"has_data"`
		Previous           totalsJSON `json:"previous"`
		Comparable         bool       `json:"comparable"` // false: windows not both >= 80% covered, so no delta may be shown
		MethodologyVersion string     `json:"methodology_version"`
		// Methodology travels with the figures: a number must never look more complete than it is.
		Methodology methodologyJSON `json:"methodology"`
	}{toTotals(s.Current), s.HasData(), toTotals(s.Previous),
		domain.Comparable(s.Current.Days, s.Previous.Days, int(to.Sub(from).Hours()/24+0.5)), domain.MethodologyVersion, describeMethodology()})
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
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": t})
}

// methodology publishes the active coefficient set: users can see exactly how numbers are produced.
func (h Handlers) methodology(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, domain.Current)
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
	if err := h.Svc.Recalculate(r.Context(), c.TenantID, in.ProjectID, from, to); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
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
		httpx.WriteJSON(w, http.StatusOK, res)
	}
}
