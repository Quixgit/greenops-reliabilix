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
	read, write := auth.Require(auth.PermCloudRead), auth.Require(auth.PermCloudWrite)
	r.With(read).Get("/cloud-accounts", h.list)
	r.With(write).Post("/cloud-accounts", h.create)
	r.With(read).Get("/cloud-accounts/{id}", h.get)
	r.With(write).Delete("/cloud-accounts/{id}", h.delete)
	r.With(write).Post("/cloud-accounts/{id}/verify", h.verify)
	r.With(write).Post("/cloud-accounts/{id}/sync", h.sync)
	r.With(read).Get("/cloud-accounts/{id}/sync-runs", h.runs)
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, domain.ErrDuplicate):
		httpx.WriteProblem(w, r, http.StatusConflict, "cloud account already connected", "")
	case errors.Is(err, domain.ErrInvalidConnection):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid connection", err.Error())
	case errors.Is(err, domain.ErrUnsupportedProvider):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "provider not supported yet", "")
	case errors.Is(err, domain.ErrAccessDenied):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "access verification failed",
			"The role could not be assumed or lacks ce:GetCostAndUsage. Check the trust policy ExternalId.")
	default:
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
	}
}

func idParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
	}
	return id, ok
}

func (h Handlers) list(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	items, err := h.Svc.List(r.Context(), c.TenantID)
	if err != nil {
		fail(w, r, err)
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
	if !httpx.ParseUUID(in.ProjectID) {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid connection", "project_id must be a UUID")
		return
	}
	conn, setup, err := h.Svc.Connect(r.Context(), domain.Connection{TenantID: cl.TenantID, ProjectID: in.ProjectID,
		Provider: in.Provider, AccountRef: in.AccountRef, CredentialRef: in.CredentialRef})
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"connection": conn, "setup": setup})
}

func (h Handlers) get(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	conn, err := h.Svc.Get(r.Context(), c.TenantID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	setup, err := h.Svc.SetupFor(r.Context(), c.TenantID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"connection": conn, "setup": setup})
}

func (h Handlers) delete(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	if err := h.Svc.Delete(r.Context(), c.TenantID, id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) verify(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	conn, err := h.Svc.Verify(r.Context(), c.TenantID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, conn)
}

func (h Handlers) sync(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	conn, err := h.Svc.Get(r.Context(), c.TenantID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.Svc.SyncNow(r.Context(), c.TenantID, conn.ProjectID, id); err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (h Handlers) runs(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	items, err := h.Svc.Runs(r.Context(), c.TenantID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
