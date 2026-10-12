// Package http exposes the cloud-accounts endpoints.
package http

import (
	"errors"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
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
	r.With(read).Get("/cloud-accounts/commitments", h.commitments)
	r.With(read).Get("/cloud-accounts/{id}", h.get)
	r.With(write).Delete("/cloud-accounts/{id}", h.delete)
	r.With(write).Put("/cloud-accounts/{id}/export", h.setExport)
	r.With(write).Delete("/cloud-accounts/{id}/export", h.clearExport)
	r.With(write).Post("/cloud-accounts/{id}/verify", h.verify)
	r.With(write).Post("/cloud-accounts/{id}/sync", h.sync)
	r.With(read).Get("/cloud-accounts/{id}/sync-runs", h.runs)
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, application.ErrSecretsUnavailable):
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "secret storage unavailable", "The platform has no sealing key configured, so a client secret cannot be stored.")
	case errors.Is(err, entitlements.ErrLimitReached):
		httpx.WriteProblem(w, r, http.StatusPaymentRequired, "plan limit reached", err.Error())
	case errors.Is(err, domain.ErrProviderNotAllowed):
		httpx.WriteProblem(w, r, http.StatusForbidden, "provider not enabled", err.Error())
	case errors.Is(err, domain.ErrDuplicate):
		httpx.WriteProblem(w, r, http.StatusConflict, "cloud account already connected", "")
	case errors.Is(err, domain.ErrInvalidConnection):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid connection", err.Error())
	case errors.Is(err, domain.ErrUnsupportedProvider):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "provider not supported yet", "")
	case errors.Is(err, domain.ErrAccessDenied):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "access verification failed",
			"The platform could not read your billing data. Check the granted access: the AWS role (trust policy ExternalId, read permissions), the platform service account's read access to the BigQuery dataset, or the Azure service principal's Cost Management Reader role and secret.")
	case errors.Is(err, domain.ErrSubscriptionNotInExport):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "no rows for the subscription",
			"The export has no rows for this subscription: check the subscription id and that the export covers it.")
	case errors.Is(err, domain.ErrBigQueryDisabled):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "BigQuery is not enabled",
			"Enable the BigQuery API in your Google Cloud project and turn on Cloud Billing export to BigQuery, then verify again.")
	case errors.Is(err, domain.ErrOwnershipNotProven):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "ownership not proven",
			"Add the connection's label to the billing dataset (see the setup steps), then verify again.")
	case errors.Is(err, domain.ErrExportNotFound):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "billing export not found",
			"No billing export was found. Check that it is enabled and delivered (the first delivery can take up to 48 hours) and that the location is correct.")
	default:
		httpx.Internal(w, r, err)
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
		// ClientSecret is write-only: sealed on arrival, never returned.
		ClientSecret string `json:"client_secret"`
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
		Provider: in.Provider, AccountRef: in.AccountRef, CredentialRef: in.CredentialRef, ClientSecret: in.ClientSecret})
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

// setExport points the connection at a FOCUS data export in S3 (locations only, never credentials).
func (h Handlers) setExport(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	var in domain.ExportConfig
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	conn, err := h.Svc.SetExport(r.Context(), c.TenantID, id, &in)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, conn)
}

// clearExport switches the connection back to Cost Explorer.
func (h Handlers) clearExport(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	conn, err := h.Svc.SetExport(r.Context(), c.TenantID, id, nil)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, conn)
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

// commitments lists the Savings Plans and reservations the connected accounts hold, as last read (ADR-0054).
func (h Handlers) commitments(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	items, err := h.Svc.ExistingCommitments(r.Context(), c.TenantID)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
