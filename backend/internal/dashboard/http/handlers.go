// Package http exposes the Overview read models.
package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/dashboard/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/readcache"
)

type Handlers struct{ Repo domain.Repository }

func (h Handlers) Routes(r chi.Router) {
	read := r.With(auth.Require(auth.PermUsageRead))
	// The heavy questions that depend only on ingested data are answered from memory for a few seconds and shared between concurrent askers (ADR-0060).
	// What a person changes and then looks at (connections, recommendations, functional units) is not kept.
	heavy := r.With(auth.Require(auth.PermUsageRead), readcache.Middleware)
	heavy.Get("/dashboard/trend", h.trend)
	heavy.Get("/dashboard/providers", h.providers)
	heavy.Get("/dashboard/services", h.services)
	read.Get("/dashboard/connections", h.connections)
	read.Get("/dashboard/grid-sources", h.gridSources)
	heavy.Get("/dashboard/regions", h.regions)
	heavy.Get("/dashboard/countries", h.countries)
	heavy.Get("/dashboard/regions/footprint", h.footprint)
	read.Get("/dashboard/optimization", h.optimization)
	read.Get("/dashboard/efficiency", h.efficiency)
	read.Get("/dashboard/unit-economics", h.unitEconomics)
	read.Get("/dashboard/activity", h.activity)
}

func filter(w http.ResponseWriter, r *http.Request) (domain.Filter, bool) {
	c, _ := auth.FromContext(r.Context())
	from, to, err := httpx.Period(r)
	project, ok := httpx.OptionalUUID(r, "project_id")
	if err != nil || !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "")
		return domain.Filter{}, false
	}
	return domain.Filter{TenantID: c.TenantID, Project: project, From: from, To: to}, true
}

func respond[T any](w http.ResponseWriter, r *http.Request, items T, err error) {
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) connections(w http.ResponseWriter, r *http.Request) {
	if f, ok := filter(w, r); ok {
		v, err := h.Repo.ConnectionCosts(r.Context(), f)
		respond(w, r, v, err)
	}
}

func (h Handlers) gridSources(w http.ResponseWriter, r *http.Request) {
	v, err := h.Repo.GridSources(r.Context())
	respond(w, r, v, err)
}

func (h Handlers) trend(w http.ResponseWriter, r *http.Request) {
	if f, ok := filter(w, r); ok {
		v, err := h.Repo.Trend(r.Context(), f)
		respond(w, r, v, err)
	}
}

func (h Handlers) providers(w http.ResponseWriter, r *http.Request) {
	if f, ok := filter(w, r); ok {
		v, err := h.Repo.Providers(r.Context(), f)
		respond(w, r, v, err)
	}
}

func (h Handlers) services(w http.ResponseWriter, r *http.Request) {
	f, ok := filter(w, r)
	if !ok {
		return
	}
	cat := r.URL.Query().Get("category")
	if !domain.Categories[cat] {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid category", "")
		return
	}
	v, err := h.Repo.Services(r.Context(), f, cat)
	respond(w, r, v, err)
}

func (h Handlers) regions(w http.ResponseWriter, r *http.Request) {
	v, err := h.Repo.Regions(r.Context())
	respond(w, r, v, err)
}

func (h Handlers) countries(w http.ResponseWriter, r *http.Request) {
	v, err := h.Repo.Countries(r.Context())
	respond(w, r, v, err)
}

func (h Handlers) activity(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 50 {
		limit = 10
	}
	v, err := h.Repo.Activity(r.Context(), c.TenantID, limit)
	respond(w, r, v, err)
}

func (h Handlers) footprint(w http.ResponseWriter, r *http.Request) {
	if f, ok := filter(w, r); ok {
		v, err := h.Repo.RegionFootprint(r.Context(), f)
		respond(w, r, v, err)
	}
}

func (h Handlers) optimization(w http.ResponseWriter, r *http.Request) {
	f, ok := filter(w, r)
	if !ok {
		return
	}
	v, err := h.Repo.Optimization(r.Context(), f)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (h Handlers) efficiency(w http.ResponseWriter, r *http.Request) {
	f, ok := filter(w, r)
	if !ok {
		return
	}
	v, err := h.Repo.Efficiency(r.Context(), f)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

// unitEconomics is the cost and carbon of one unit of work, per project and month (ADR-0053).
func (h Handlers) unitEconomics(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	project, ok := httpx.OptionalUUID(r, "project_id")
	months := 6
	if v := r.URL.Query().Get("months"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 2 || n > 12 {
			ok = false
		} else {
			months = n
		}
	}
	if !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "months 2-12, project_id a UUID")
		return
	}
	v, err := h.Repo.UnitEconomics(r.Context(), c.TenantID, project, months)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}
