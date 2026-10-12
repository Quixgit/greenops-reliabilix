package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/domain"
)

// PublicRoutes need no sign-in: the sign-in page asks where an address signs in. The answer is the name of a connection and nothing else.
func (h Handlers) PublicRoutes(r chi.Router) {
	r.Get("/sso/discover", h.ssoDiscover)
}

func (h Handlers) ssoRoutes(r chi.Router) {
	admin := auth.Require(auth.PermTenantAdmin)
	r.With(admin).Get("/sso/domains", h.ssoList)
	r.With(admin).Post("/sso/domains", h.ssoAdd)
	r.With(admin).Post("/sso/domains/{id}/verify", h.ssoVerify)
	r.With(admin).Delete("/sso/domains/{id}", h.ssoDelete)
}

func (h Handlers) ssoDiscover(w http.ResponseWriter, r *http.Request) {
	conn, err := h.Svc.SSOConnection(r.Context(), r.URL.Query().Get("email"))
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"connection": conn})
}

func (h Handlers) ssoList(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	items, err := h.Svc.SSODomains(r.Context(), c.TenantID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func ssoFail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrDomainInvalid):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid domain", "Use your company's domain, not a public mail provider.")
	case errors.Is(err, domain.ErrConnectionName), errors.Is(err, domain.ErrInvalidJITRole):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", err.Error())
	case errors.Is(err, domain.ErrDomainTaken):
		httpx.WriteProblem(w, r, http.StatusConflict, "domain taken", err.Error())
	case errors.Is(err, domain.ErrNotVerified):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "not verified", "The TXT record was not found yet. DNS changes can take a few minutes to appear.")
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r)
	default:
		httpx.Internal(w, r, err)
	}
}

func (h Handlers) ssoAdd(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		Domain     string `json:"domain"`
		Connection string `json:"connection"`
		JITRole    string `json:"jit_role"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	d, err := h.Svc.AddSSODomain(r.Context(), c.TenantID, c.Subject, in.Domain, in.Connection, strings.TrimSpace(in.JITRole))
	if err != nil {
		ssoFail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, d)
}

func (h Handlers) ssoVerify(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	if err := h.Svc.VerifySSODomain(r.Context(), c.TenantID, id); err != nil {
		ssoFail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) ssoDelete(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	if err := h.Svc.DeleteSSODomain(r.Context(), c.TenantID, id); err != nil {
		ssoFail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
