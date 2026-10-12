// Package http exposes the finops endpoints.
package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/readcache"
)

type Handlers struct{ Svc application.Service }

func (h Handlers) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermUsageRead), readcache.Middleware).Get("/finops/summary", h.summary)
	r.With(auth.Require(auth.PermBillingRead)).Get("/finops/budgets", h.budgets)
	r.With(auth.Require(auth.PermBudgetWrite)).Post("/finops/budgets", h.createBudget)
	r.With(auth.Require(auth.PermBudgetWrite)).Delete("/finops/budgets/{id}", h.deleteBudget)
	r.With(auth.Require(auth.PermBillingRead), readcache.Middleware).Get("/finops/anomalies", h.anomalies)
	r.With(auth.Require(auth.PermBillingRead)).Get("/finops/commitments", h.commitments)
	r.With(auth.Require(auth.PermUsageRead), readcache.Middleware).Get("/finops/forecast", h.forecast)
	r.With(auth.Require(auth.PermUsageRead)).Get("/finops/tag-keys", h.tagKeys)
	r.With(auth.Require(auth.PermUsageRead), readcache.Middleware).Get("/finops/showback", h.showback)
}

func internal(w http.ResponseWriter, r *http.Request, err error) {
	httpx.Internal(w, r, err)
}

func (h Handlers) summary(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, err := httpx.Period(r)
	project, ok := httpx.OptionalUUID(r, "project_id")
	if err != nil || !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "")
		return
	}
	s, err := h.Svc.Summary(r.Context(), c.TenantID, project, from, to)
	if err != nil {
		internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}

func (h Handlers) budgets(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	items, err := h.Svc.Budgets(r.Context(), c.TenantID)
	if err != nil {
		internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) createBudget(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		ProjectID *string `json:"project_id"`
		Amount    float64 `json:"amount"`
		Currency  string  `json:"currency"`
		Period    string  `json:"period"`
		Cloud     *string `json:"cloud"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || (in.ProjectID != nil && !httpx.ParseUUID(*in.ProjectID)) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	b, err := h.Svc.CreateBudget(r.Context(), c.TenantID, domain.Budget{ProjectID: in.ProjectID, Amount: in.Amount, Currency: in.Currency, Period: in.Period, Cloud: in.Cloud})
	switch {
	case errors.Is(err, domain.ErrInvalidBudget):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid budget", "amount >= 0, 3-letter currency, period monthly|quarterly|yearly")
	case err != nil:
		internal(w, r, err)
	default:
		httpx.WriteJSON(w, http.StatusCreated, b)
	}
}

func (h Handlers) deleteBudget(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	switch err := h.Svc.DeleteBudget(r.Context(), c.TenantID, id); {
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r)
	case err != nil:
		internal(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h Handlers) anomalies(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, err := httpx.Period(r)
	project, ok := httpx.OptionalUUID(r, "project_id")
	if err != nil || !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "")
		return
	}
	items, err := h.Svc.Anomalies(r.Context(), c.TenantID, project, from, to)
	if err != nil {
		internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

// commitments reports what a commitment would save. Every choice has a default and a range; anything outside it is refused, not clamped.
func (h Handlers) commitments(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	project, ok := httpx.OptionalUUID(r, "project_id")
	q := r.URL.Query()
	num := func(key string, def, lo, hi float64) (float64, bool) {
		v := q.Get(key)
		if v == "" {
			return def, true
		}
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil && f >= lo && f <= hi
	}
	look, ok1 := num("lookback_days", 60, 14, 180)
	term, ok2 := num("term_years", 1, 1, 3)
	cover, ok3 := num("coverage_pct", domain.DefaultCoverage, 10, 100)
	p := domain.CommitmentParams{LookbackDays: int(look), TermYears: int(term), CoveragePct: cover, DiscountPct: map[string]float64{}}
	valid := ok && ok1 && ok2 && ok3 && (p.TermYears == 1 || p.TermYears == 3) && look == float64(p.LookbackDays)
	p.Manual = map[string]domain.ManualExisting{}
	for prov := range domain.DefaultDiscountPct {
		d, okd := num("discount_"+prov, -1, -1, 90)
		valid = valid && okd
		if d >= 0 {
			p.DiscountPct[prov] = d
		}
		cv, okc := num("existing_coverage_"+prov, -1, -1, 100)
		ut, oku := num("existing_utilization_"+prov, -1, -1, 100)
		valid = valid && okc && oku
		if cv >= 0 || ut >= 0 {
			m := domain.ManualExisting{}
			if cv >= 0 {
				m.CoveragePct = &cv
			}
			if ut >= 0 {
				m.UtilizationPct = &ut
			}
			p.Manual[prov] = m
		}
	}
	if !valid {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "lookback_days 14-180, term_years 1 or 3, coverage_pct 10-100, discount_aws|azure|gcp 0-90, existing_coverage_<cloud> and existing_utilization_<cloud> 0-100")
		return
	}
	out, err := h.Svc.Commitments(r.Context(), c.TenantID, project, p)
	if err != nil {
		internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// forecast is the expected cost at the end of the current month (ADR-0053).
func (h Handlers) forecast(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	project, ok := httpx.OptionalUUID(r, "project_id")
	if !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "")
		return
	}
	f, cur, err := h.Svc.Forecast(r.Context(), c.TenantID, project)
	if err != nil {
		internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"currency": cur, "forecast": f})
}

// tagKeys lists what the bill can be split by.
func (h Handlers) tagKeys(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, err := httpx.Period(r)
	project, ok := httpx.OptionalUUID(r, "project_id")
	if err != nil || !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "")
		return
	}
	keys, cur, err := h.Svc.TagKeys(r.Context(), c.TenantID, project, from, to)
	if err != nil {
		internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": keys, "currency": cur})
}

// showback splits the bill by the values of one tag key (the key is validated: the same shape the setting allows).
func (h Handlers) showback(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, err := httpx.Period(r)
	project, ok := httpx.OptionalUUID(r, "project_id")
	tag := focus.NormalizeTagKey(r.URL.Query().Get("tag"))
	if err != nil || !ok || tag == "" {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "tag must be a tag key")
		return
	}
	sb, err := h.Svc.Showback(r.Context(), c.TenantID, project, tag, from, to)
	if err != nil {
		internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sb)
}
