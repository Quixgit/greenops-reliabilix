// Package http exposes the carbon endpoints.
package http

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

type Handlers struct {
	Svc  *application.Service
	Repo repository.Postgres
}

func (h Handlers) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermCarbonRead)).Get("/carbon/summary", h.summary)
	r.With(auth.Require(auth.PermCarbonRead)).Get("/carbon/trend", h.trend)
	r.With(auth.Require(auth.PermCarbonCompute)).Post("/carbon/calculate", h.calculate)
}

func (h Handlers) summary(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, err := httpx.Period(r)
	project, ok := httpx.OptionalUUID(r, "project_id")
	if err != nil || !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "")
		return
	}
	s, err := h.Repo.Summary(r.Context(), c.TenantID, project, from, to)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}

func (h Handlers) trend(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, err := httpx.Period(r)
	project, ok := httpx.OptionalUUID(r, "project_id")
	if err != nil || !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "")
		return
	}
	t, err := h.Repo.Trend(r.Context(), c.TenantID, project, from, to)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": t})
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
	case err != nil:
		httpx.WriteProblem(w, r, http.StatusBadGateway, "carbon data unavailable", "")
	default:
		httpx.WriteJSON(w, http.StatusOK, res)
	}
}
