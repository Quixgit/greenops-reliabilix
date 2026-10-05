// Package http exposes the current-user endpoint.
package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

type Handlers struct{}

func (Handlers) Routes(r chi.Router) {
	r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.FromContext(r.Context())
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"user_id": c.Subject, "tenant_id": c.TenantID, "role": string(c.Role)})
	})
}
