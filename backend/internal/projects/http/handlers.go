// Package http exposes the project endpoints.
package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/domain"
)

type Handlers struct{ Svc application.Service }

func (h Handlers) Routes(r chi.Router) {
	read, write := auth.Require(auth.PermProjectRead), auth.Require(auth.PermProjectWrite)
	r.With(read).Get("/projects", h.list)
	r.With(write).Post("/projects", h.create)
	r.With(read).Get("/projects/{id}", h.get)
	r.With(read).Get("/projects/{id}/functional-units", h.listUnits)
	r.With(write).Post("/projects/{id}/functional-units", h.reportUnits)
	r.With(read).Get("/projects/{id}/policy", h.getPolicy)
	r.With(write).Put("/projects/{id}/policy", h.putPolicy)
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, domain.ErrDuplicate):
		httpx.WriteProblem(w, r, http.StatusConflict, "project already exists", "")
	case errors.Is(err, domain.ErrInvalidName), errors.Is(err, domain.ErrInvalidUnit),
		errors.Is(err, domain.ErrInvalidPeriod), errors.Is(err, domain.ErrInvalidPolicy):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", err.Error())
	default:
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
	}
}

func (h Handlers) list(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context()) // tenant always from the verified identity
	ps, err := h.Svc.List(r.Context(), c.TenantID)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": ps})
}

func (h Handlers) create(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		Name           string `json:"name"`
		FunctionalUnit string `json:"functional_unit"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	p, err := h.Svc.Create(r.Context(), c.TenantID, in.Name, in.FunctionalUnit)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, p)
}

func (h Handlers) get(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	p, err := h.Svc.Get(r.Context(), c.TenantID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

func (h Handlers) listUnits(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	items, err := h.Svc.ListFunctionalUnits(r.Context(), c.TenantID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) reportUnits(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	var in struct {
		PeriodStart string  `json:"period_start"`
		PeriodEnd   string  `json:"period_end"`
		Units       float64 `json:"units"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	start, err1 := time.Parse(time.DateOnly, in.PeriodStart)
	end, err2 := time.Parse(time.DateOnly, in.PeriodEnd)
	if err1 != nil || err2 != nil {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", "dates must be YYYY-MM-DD")
		return
	}
	f := domain.FunctionalUnits{ProjectID: id, PeriodStart: start, PeriodEnd: end, Units: in.Units}
	if err := h.Svc.ReportFunctionalUnits(r.Context(), c.TenantID, f); err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, f)
}

func (h Handlers) getPolicy(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	p, err := h.Svc.GetPolicy(r.Context(), c.TenantID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

func (h Handlers) putPolicy(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	var in domain.Policy
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	in.ProjectID = id
	p, err := h.Svc.SetPolicy(r.Context(), c.TenantID, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}
