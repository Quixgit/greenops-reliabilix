// Package transport exposes tenant/membership endpoints.
package transport

import (
	"net/http"

	"github.com/quixgit/greenops-reliabilix/pkg/auth"
	"github.com/quixgit/greenops-reliabilix/pkg/httpx"
)

func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/me", func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.FromContext(r.Context())
		httpx.WriteJSON(w, http.StatusOK, map[string]string{
			"subject": c.Subject, "tenant_id": c.TenantID, "role": string(c.Role),
		})
	})
}
