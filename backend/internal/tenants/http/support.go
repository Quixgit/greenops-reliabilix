package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/domain"
)

// supportRoutes: the organization's own control over platform support access. Owner and admin only; the operators' side is in
// the operator console and refuses without a grant.
func (h Handlers) supportRoutes(r chi.Router) {
	if h.Support == nil {
		return
	}
	admin := auth.Require(auth.PermTenantAdmin)
	r.With(admin).Get("/support-access", h.supportStatus)
	r.With(admin).Post("/support-access", h.supportGrant)
	r.With(admin).Delete("/support-access", h.supportRevoke)
}

func (h Handlers) supportStatus(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	gs, vs, err := h.Support.History(r.Context(), c.TenantID, 10, 50)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"grants": gs, "views": vs, "min_hours": domain.MinSupportHours, "max_hours": domain.MaxSupportHours})
}

func (h Handlers) supportGrant(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		Hours int    `json:"hours"`
		Note  string `json:"note"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid body", "")
		return
	}
	g, err := h.Support.Grant(r.Context(), c.TenantID, c.Subject, in.Hours, in.Note)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, g)
}

func (h Handlers) supportRevoke(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	if _, err := h.Support.Revoke(r.Context(), c.TenantID); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
