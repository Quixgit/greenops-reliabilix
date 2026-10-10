// Package http exposes the data-lifecycle endpoints: export and scheduled erasure of an organization.
package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/lifecycle/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/lifecycle/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

type Handlers struct{ Svc application.Service }

func (h Handlers) Routes(r chi.Router) {
	owner := auth.Require(auth.PermTenantData)
	r.With(owner).Get("/tenant/export", h.export)
	r.With(owner).Get("/tenant/deletion", h.status)
	r.With(owner).Post("/tenant/deletion", h.schedule)
	r.With(owner).Delete("/tenant/deletion", h.cancel)
}

// people reports false for machine credentials: erasing an organization is a decision made by a person.
func person(w http.ResponseWriter, r *http.Request) (auth.Claims, bool) {
	c, _ := auth.FromContext(r.Context())
	if c.KeyID != "" {
		httpx.WriteProblem(w, r, http.StatusForbidden, "forbidden", "")
		return c, false
	}
	return c, true
}

func (h Handlers) export(w http.ResponseWriter, r *http.Request) {
	c, ok := person(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", `attachment; filename="reliabilix-organization-`+time.Now().UTC().Format("20060102")+`.ndjson"`)
	w.Header().Set("Cache-Control", "no-store")
	err := h.Svc.Export(r.Context(), c.TenantID, func(line []byte) error {
		_, err := w.Write(line)
		return err
	})
	if err != nil {
		// The status is already sent; end the stream with an error marker a reader can detect.
		_, _ = w.Write([]byte(`{"error":"export interrupted"}` + "\n"))
		httpx.ReportError(r, err)
	}
}

func (h Handlers) status(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	d, err := h.Svc.Status(r.Context(), c.TenantID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

func (h Handlers) schedule(w http.ResponseWriter, r *http.Request) {
	c, ok := person(w, r)
	if !ok {
		return
	}
	var in struct {
		ConfirmName string `json:"confirm_name"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	d, err := h.Svc.ScheduleDeletion(r.Context(), c.TenantID, c.Subject, in.ConfirmName)
	switch {
	case errors.Is(err, domain.ErrNameMismatch):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "name does not match", "Type the organization's name exactly to confirm.")
	case errors.Is(err, domain.ErrAlreadyScheduled):
		httpx.WriteProblem(w, r, http.StatusConflict, "already scheduled", "")
	case err != nil:
		httpx.Internal(w, r, err)
	default:
		httpx.WriteJSON(w, http.StatusAccepted, d)
	}
}

func (h Handlers) cancel(w http.ResponseWriter, r *http.Request) {
	c, ok := person(w, r)
	if !ok {
		return
	}
	switch err := h.Svc.CancelDeletion(r.Context(), c.TenantID); {
	case errors.Is(err, domain.ErrNothingScheduled):
		httpx.NotFound(w, r)
	case err != nil:
		httpx.Internal(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
