// Package http exposes the Overview read models.
package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/dashboard/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

type Handlers struct{ Repo domain.Repository }

func (h Handlers) Routes(r chi.Router) {
	read := r.With(auth.Require(auth.PermUsageRead))
	read.Get("/dashboard/trend", h.trend)
	read.Get("/dashboard/providers", h.providers)
	read.Get("/dashboard/services", h.services)
	read.Get("/dashboard/regions", h.regions)
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
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
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

func (h Handlers) activity(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 50 {
		limit = 10
	}
	v, err := h.Repo.Activity(r.Context(), c.TenantID, limit)
	respond(w, r, v, err)
}
