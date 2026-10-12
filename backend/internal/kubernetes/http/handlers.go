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
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/readcache"
)

type Handlers struct{ Svc application.Service }

func (h Handlers) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermKubernetesIngest)).Post("/kubernetes/energy", h.ingest)
	r.With(auth.Require(auth.PermCarbonRead)).Get("/kubernetes/workloads", h.workloads)
	r.With(auth.Require(auth.PermCarbonRead)).Get("/kubernetes/clusters", h.clusters)
	r.With(auth.Require(auth.PermUsageRead), readcache.Middleware).Get("/kubernetes/cost", h.cost)
	r.With(auth.Require(auth.PermUsageRead)).Get("/kubernetes/clusters/pool", h.pool)
	r.With(auth.Require(auth.PermCloudWrite)).Put("/kubernetes/clusters/pool", h.setPool)
}

func (h Handlers) ingest(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		ProjectID string            `json:"project_id"`
		Samples   []domain.Sample   `json:"samples"`
		Capacity  []domain.Capacity `json:"capacity"` // optional: the nodes' allocatable CPU and memory per hour, so that idle can be told
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || !httpx.ParseUUID(in.ProjectID) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "project_id must be a UUID and samples a list")
		return
	}
	res, err := h.Svc.IngestWithCapacity(r.Context(), c.TenantID, in.ProjectID, in.Samples, in.Capacity)
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

// cost allocates the cost of the clusters that reported in the window over their workloads.
func (h Handlers) cost(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, err := httpx.Period(r)
	project, okp := httpx.OptionalUUID(r, "project_id")
	if err != nil || !okp {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "")
		return
	}
	rep, err := h.Svc.Cost(r.Context(), c.TenantID, project, r.URL.Query().Get("cluster"), from, to)
	if errors.Is(err, application.ErrNoPoolSource) {
		httpx.WriteProblem(w, r, http.StatusNotImplemented, "not available", "")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rep)
}

// pool tells which billed services pay for a cluster and which could.
func (h Handlers) pool(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	from, to, err := httpx.Period(r)
	project, okp := httpx.OptionalUUID(r, "project_id")
	cluster := r.URL.Query().Get("cluster")
	if err != nil || !okp || project == nil || cluster == "" {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "project_id and cluster are required")
		return
	}
	pc, err := h.Svc.PoolChoice(r.Context(), c.TenantID, *project, cluster, from, to)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, pc)
}

func (h Handlers) setPool(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		ProjectID string   `json:"project_id"`
		Cluster   string   `json:"cluster"`
		Services  []string `json:"services"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || !httpx.ParseUUID(in.ProjectID) || in.Cluster == "" || len(in.Cluster) > 127 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "project_id, cluster and services are required")
		return
	}
	switch err := h.Svc.SetPool(r.Context(), c.TenantID, in.ProjectID, in.Cluster, in.Services); {
	case errors.Is(err, application.ErrProjectNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, application.ErrInvalidPool):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid services", "at most 50 services")
	case err != nil:
		httpx.Internal(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
