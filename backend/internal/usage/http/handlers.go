// Package http exposes the usage endpoint.
package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/usage/domain"
)

type Handlers struct{ Repo domain.Repository }

func (h Handlers) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermUsageRead)).Get("/usage", h.list)
}

func (h Handlers) list(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	to := time.Now().UTC()
	from := to.AddDate(0, 0, -30)
	if v, err := time.Parse(time.DateOnly, r.URL.Query().Get("from")); err == nil {
		from = v
	}
	if v, err := time.Parse(time.DateOnly, r.URL.Query().Get("to")); err == nil {
		to = v
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 1000 {
		limit = 100
	}
	items, err := h.Repo.List(r.Context(), c.TenantID, from, to, limit)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
