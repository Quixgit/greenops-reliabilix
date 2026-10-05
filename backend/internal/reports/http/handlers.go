// Package http exposes the report endpoints.
package http

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/domain"
)

type Handlers struct{ Svc application.Service }

func (h Handlers) Routes(r chi.Router) {
	read, write := auth.Require(auth.PermReportRead), auth.Require(auth.PermReportWrite)
	r.With(read).Get("/reports", h.list)
	r.With(write).Post("/reports", h.create)
	r.With(read).Get("/reports/{id}", h.get)
	r.With(read).Get("/reports/{id}/download", h.download)
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, domain.ErrInvalidReport):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid report request", "kind carbon|sci|finops, format csv|json|pdf, period of at most 400 days")
	case errors.Is(err, domain.ErrNotReady):
		httpx.WriteProblem(w, r, http.StatusConflict, "report is not ready", "")
	default:
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
	}
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

func (h Handlers) get(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	rep, err := h.Svc.Get(r.Context(), c.TenantID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rep)
}

func (h Handlers) create(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		ProjectID   *string       `json:"project_id"`
		Kind        domain.Kind   `json:"kind"`
		Format      domain.Format `json:"format"`
		PeriodStart string        `json:"period_start"`
		PeriodEnd   string        `json:"period_end"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || (in.ProjectID != nil && !httpx.ParseUUID(*in.ProjectID)) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	start, err1 := time.Parse(time.DateOnly, in.PeriodStart)
	end, err2 := time.Parse(time.DateOnly, in.PeriodEnd)
	if err1 != nil || err2 != nil {
		fail(w, r, domain.ErrInvalidReport)
		return
	}
	rep, err := h.Svc.Create(r.Context(), c.TenantID, c.Subject, domain.Report{ProjectID: in.ProjectID, Kind: in.Kind, Format: in.Format, PeriodStart: start, PeriodEnd: end})
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, rep)
}

func (h Handlers) download(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	d, err := h.Svc.Open(r.Context(), c.TenantID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	defer func() { _ = d.Body.Close() }()
	w.Header().Set("Content-Type", d.ContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+d.Filename+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, d.Body)
}
