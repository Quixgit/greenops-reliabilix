// Package http exposes tenant-level endpoints.
package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/repository"
)

type Handlers struct{ Repo repository.Postgres }

func (h Handlers) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermAuditRead)).Get("/audit-log", h.auditLog)
}

func (h Handlers) auditLog(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	items, err := h.Repo.AuditLog(r.Context(), c.TenantID, limit)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
