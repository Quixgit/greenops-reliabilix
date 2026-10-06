// Package http exposes the automation endpoints.
package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/automation/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/automation/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

type Handlers struct{ Svc application.Service }

func (h Handlers) Routes(r chi.Router) {
	read, write, approve := auth.Require(auth.PermAutomationRead), auth.Require(auth.PermAutomationWrite), auth.Require(auth.PermAutomationApprove)
	r.With(read).Get("/automation/jobs", h.list)
	r.With(write).Post("/automation/jobs", h.plan)
	r.With(read).Get("/automation/jobs/{id}", h.get)
	r.With(approve).Post("/automation/jobs/{id}/approve", h.approve)
	r.With(write).Post("/automation/jobs/{id}/result", h.result)
	r.With(write).Post("/automation/jobs/{id}/cancel", h.cancel)
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, domain.ErrInvalidTransition), errors.Is(err, domain.ErrActiveJobExists), errors.Is(err, domain.ErrNotApproved):
		httpx.WriteProblem(w, r, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, domain.ErrRecommendationNot), errors.Is(err, domain.ErrUnsupported), errors.Is(err, domain.ErrInvalidInput), errors.Is(err, domain.ErrNoteRequired):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "cannot plan", err.Error())
	default:
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
	}
}

func jobID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
	}
	return id, ok
}

func (h Handlers) list(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "limit must be 1-100")
			return
		}
		limit = n
	}
	items, err := h.Svc.List(r.Context(), c.TenantID, limit)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) plan(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		RecommendationID string `json:"recommendation_id"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || !httpx.ParseUUID(in.RecommendationID) {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", "recommendation_id must be a UUID")
		return
	}
	j, err := h.Svc.Plan(r.Context(), c.TenantID, c, in.RecommendationID)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, j)
}

func (h Handlers) get(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := jobID(w, r)
	if !ok {
		return
	}
	j, err := h.Svc.Get(r.Context(), c.TenantID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, j)
}

func (h Handlers) approve(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := jobID(w, r)
	if !ok {
		return
	}
	j, err := h.Svc.Approve(r.Context(), c.TenantID, id, c)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, j)
}

func (h Handlers) cancel(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := jobID(w, r)
	if !ok {
		return
	}
	j, err := h.Svc.Cancel(r.Context(), c.TenantID, id, c)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, j)
}

func (h Handlers) result(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := jobID(w, r)
	if !ok {
		return
	}
	var in struct {
		Outcome domain.Outcome `json:"outcome"`
		Note    string         `json:"note"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	j, err := h.Svc.Report(r.Context(), c.TenantID, id, c, in.Outcome, in.Note)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, j)
}
