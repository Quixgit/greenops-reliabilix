// Package http exposes the recommendation endpoints.
package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/domain"
)

type Handlers struct{ Svc application.Service }

func (h Handlers) Routes(r chi.Router) {
	read, decide := auth.Require(auth.PermRecommendRead), auth.Require(auth.PermRecommendApply)
	r.With(read).Get("/recommendations", h.list)
	r.With(decide).Post("/recommendations/refresh", h.refresh)
	r.With(read).Get("/recommendations/{id}", h.get)
	r.With(decide).Post("/recommendations/{id}/approve", h.decide("approve"))
	r.With(decide).Post("/recommendations/{id}/apply", h.decide("apply"))
	r.With(decide).Post("/recommendations/{id}/dismiss", h.decide("dismiss"))
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, domain.ErrInvalidTransition):
		httpx.WriteProblem(w, r, http.StatusConflict, "invalid status transition", err.Error())
	case errors.Is(err, domain.ErrComplianceChanged):
		httpx.WriteProblem(w, r, http.StatusConflict, "compliance changed", "The project's data-residency policy no longer allows the recommended region.")
	default:
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
	}
}

func (h Handlers) list(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	status := r.URL.Query().Get("status")
	switch status {
	case "", "open", "approved", "applied", "dismissed":
	default:
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid status", "")
		return
	}
	project, ok := httpx.OptionalUUID(r, "project_id")
	if !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	items, err := h.Svc.List(r.Context(), c.TenantID, status, project, limit)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) get(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	rec, err := h.Svc.Get(r.Context(), c.TenantID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rec)
}

func (h Handlers) decide(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.FromContext(r.Context())
		if c.KeyID != "" { // decisions are made by people, never by machine credentials
			httpx.WriteProblem(w, r, http.StatusForbidden, "forbidden", "")
			return
		}
		id, ok := httpx.PathUUID(r, "id")
		if !ok {
			httpx.NotFound(w, r)
			return
		}
		var (
			rec domain.Recommendation
			err error
		)
		switch action {
		case "approve":
			rec, err = h.Svc.Approve(r.Context(), c.TenantID, id, c)
		case "apply":
			rec, err = h.Svc.Apply(r.Context(), c.TenantID, id, c)
		default:
			rec, err = h.Svc.Dismiss(r.Context(), c.TenantID, id, c)
		}
		if err != nil {
			fail(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, rec)
	}
}

func (h Handlers) refresh(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		ProjectID string `json:"project_id"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || !httpx.ParseUUID(in.ProjectID) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	if err := h.Svc.Refresh(r.Context(), c.TenantID, in.ProjectID); err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}
