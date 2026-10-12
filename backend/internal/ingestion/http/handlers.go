// Package http exposes the upload of vendor cost files (ADR-0062).
package http

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

type Handlers struct{ Imports application.Imports }

func (h Handlers) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermCloudRead)).Get("/imports", h.list)
	r.With(auth.Require(auth.PermCloudWrite)).Post("/imports/cost", h.upload)
	r.With(auth.Require(auth.PermCloudWrite)).Delete("/imports/{id}", h.remove)
}

func (h Handlers) list(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	items, err := h.Imports.List(r.Context(), c.TenantID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

// upload takes the file as the request body (CSV, text), with the project, vendor and optional currency and file name in the query.
func (h Handlers) upload(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	project, ok := httpx.OptionalUUID(r, "project_id")
	if !ok || project == nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "project_id is required")
		return
	}
	q := r.URL.Query()
	vendor := strings.TrimSpace(q.Get("vendor"))
	if vendor == "" {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "vendor is required")
		return
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			httpx.WriteProblem(w, r, http.StatusRequestEntityTooLarge, "file too large", "The file may be at most 8 MB.")
			return
		}
		httpx.WriteProblem(w, r, http.StatusBadRequest, "the file could not be read", "")
		return
	}
	rec, prev, err := h.Imports.Start(r.Context(), c.TenantID, *project, c.Subject, q.Get("filename"), vendor, q.Get("currency"), data)
	var bad application.ErrInvalidFile
	switch {
	case errors.As(err, &bad):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "the file cannot be used", bad.Reason)
	case errors.Is(err, application.ErrNotFound):
		httpx.NotFound(w, r)
	case err != nil:
		httpx.Internal(w, r, err)
	default:
		var from, to *string
		if !prev.From.IsZero() {
			f, t := prev.From.Format("2006-01-02"), prev.To.Format("2006-01-02")
			from, to = &f, &t
		}
		httpx.WriteJSON(w, http.StatusAccepted, map[string]any{"import": rec, "preview": map[string]any{
			"rows_read": prev.RowsRead, "rows_skipped": prev.RowsSkipped, "services": prev.Services, "from": from, "to": to,
			"currency": prev.Currency, "total_cost": prev.Total, "columns": prev.Columns, "warnings": prev.Warnings}})
	}
}

func (h Handlers) remove(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	if err := h.Imports.Remove(r.Context(), c.TenantID, id); err != nil {
		if errors.Is(err, application.ErrNotFound) {
			httpx.WriteProblem(w, r, http.StatusConflict, "cannot be removed now", "The import is unknown, still being read, or already being removed.")
			return
		}
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
