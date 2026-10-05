// Package http exposes the finops endpoints.
package http

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

type Handlers struct{ Svc application.Service }

func (h Handlers) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermUsageRead)).Get("/finops/summary", h.summary)
	r.With(auth.Require(auth.PermBillingRead)).Get("/finops/budgets", h.budgets)
	r.With(auth.Require(auth.PermBudgetWrite)).Post("/finops/budgets", h.createBudget)
	r.With(auth.Require(auth.PermBudgetWrite)).Delete("/finops/budgets/{id}", h.deleteBudget)
	r.With(auth.Require(auth.PermBillingRead)).Get("/finops/anomalies", h.anomalies)
}

func internal(w http.ResponseWriter, r *http.Request) {
	httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
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
		internal(w, r)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}

func (h Handlers) budgets(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	items, err := h.Svc.Budgets(r.Context(), c.TenantID)
	if err != nil {
		internal(w, r)
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
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || (in.ProjectID != nil && !httpx.ParseUUID(*in.ProjectID)) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	b, err := h.Svc.CreateBudget(r.Context(), c.TenantID, domain.Budget{ProjectID: in.ProjectID, Amount: in.Amount, Currency: in.Currency, Period: in.Period})
	switch {
	case errors.Is(err, domain.ErrInvalidBudget):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid budget", "amount >= 0, 3-letter currency, period monthly|quarterly|yearly")
	case err != nil:
		internal(w, r)
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
		internal(w, r)
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
		internal(w, r)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
