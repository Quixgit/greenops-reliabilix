// Package http exposes the Kubernetes energy endpoints.
package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

type Handlers struct{ Svc application.Service }

func (h Handlers) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermKubernetesIngest)).Post("/kubernetes/energy", h.ingest)
	r.With(auth.Require(auth.PermCarbonRead)).Get("/kubernetes/workloads", h.workloads)
	r.With(auth.Require(auth.PermCarbonRead)).Get("/kubernetes/clusters", h.clusters)
}

func (h Handlers) ingest(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		ProjectID string          `json:"project_id"`
		Samples   []domain.Sample `json:"samples"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || !httpx.ParseUUID(in.ProjectID) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "project_id must be a UUID and samples a list")
		return
	}
	res, err := h.Svc.Ingest(r.Context(), c.TenantID, in.ProjectID, in.Samples)
	switch {
	case errors.Is(err, domain.ErrInvalidSample):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid samples", err.Error())
	case errors.Is(err, application.ErrProjectNotFound):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "unknown project", "")
	case err != nil:
		httpx.Internal(w, r, err)
	default:
		httpx.WriteJSON(w, http.StatusAccepted, map[string]any{"accepted": res.Accepted, "without_grid_data": res.WithoutGrid, "methodology_version": res.MethodologyID})
	}
}

func (h Handlers) workloads(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, err := httpx.Period(r)
	project, okp := httpx.OptionalUUID(r, "project_id")
	if err != nil || !okp {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 500 {
		limit = 100
	}
	items, err := h.Svc.Workloads(r.Context(), c.TenantID, project, r.URL.Query().Get("cluster"), from, to, limit)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) clusters(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, err := httpx.Period(r)
	project, okp := httpx.OptionalUUID(r, "project_id")
	if err != nil || !okp {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "")
		return
	}
	items, err := h.Svc.Clusters(r.Context(), c.TenantID, project, from, to)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
