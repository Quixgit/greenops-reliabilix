// Package http exposes the finops endpoints.
package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

type Handlers struct{ Repo domain.Repository }

func (h Handlers) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermUsageRead)).Get("/finops/summary", h.summary)
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
