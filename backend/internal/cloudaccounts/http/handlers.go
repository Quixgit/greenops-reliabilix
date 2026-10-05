// Package http exposes the cloud-accounts endpoints.
package http

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

type Handlers struct{ Svc application.Service }

func (h Handlers) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermCloudRead)).Get("/cloud-accounts", h.list)
	r.With(auth.Require(auth.PermCloudWrite)).Post("/cloud-accounts", h.create)
}

func (h Handlers) list(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	items, err := h.Svc.List(r.Context(), c.TenantID)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) create(w http.ResponseWriter, r *http.Request) {
	cl, _ := auth.FromContext(r.Context())
	var in struct {
		ProjectID     string              `json:"project_id"`
		Provider      domain.ProviderType `json:"provider"`
		AccountRef    string              `json:"account_ref"`
		CredentialRef string              `json:"credential_ref"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	conn, err := h.Svc.Connect(r.Context(), domain.Connection{TenantID: cl.TenantID, ProjectID: in.ProjectID,
		Provider: in.Provider, AccountRef: in.AccountRef, CredentialRef: in.CredentialRef})
	switch {
	case errors.Is(err, domain.ErrInvalidConnection):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid connection", err.Error())
	case err != nil:
		httpx.WriteProblem(w, r, http.StatusBadGateway, "could not verify cloud access", "")
	default:
		httpx.WriteJSON(w, http.StatusCreated, conn)
	}
}
