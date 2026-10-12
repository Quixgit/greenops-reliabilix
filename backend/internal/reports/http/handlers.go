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
	r.With(read).Get("/reports/options", h.options)
	r.With(read).Get("/reports/preview", h.preview)
	r.With(write).Post("/reports", h.create)
	r.With(read).Get("/reports/schedules", h.schedules)
	r.With(write).Post("/reports/schedules", h.createSchedule)
	r.With(write).Patch("/reports/schedules/{id}", h.updateSchedule)
	r.With(write).Delete("/reports/schedules/{id}", h.removeSchedule)
	r.With(read).Get("/reports/{id}", h.get)
	r.With(read).Get("/reports/{id}/download", h.download)
	r.With(write).Delete("/reports/{id}", h.remove)
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, domain.ErrFormatNotAllowed):
		httpx.WriteProblem(w, r, http.StatusForbidden, "report format not enabled", err.Error())
	case errors.Is(err, domain.ErrInvalidReport):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid report request", "kind carbon|sci|finops|sustainability, format csv|json|pdf, period of at most 400 days")
	case errors.Is(err, domain.ErrInvalidSchedule):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid report schedule", "kind carbon|sci|finops|sustainability, format csv|json|pdf, cadence weekly|monthly, at most 20 recipients")
	case errors.Is(err, domain.ErrRecipientNotMember):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "recipient is not a member", err.Error())
	case errors.Is(err, application.ErrSchedulesUnavailable):
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "report schedules unavailable", "")
	case errors.Is(err, domain.ErrNotReady):
		httpx.WriteProblem(w, r, http.StatusConflict, "report is not ready", "")
	default:
		httpx.Internal(w, r, err)
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

func (h Handlers) options(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"kinds":        []domain.Kind{domain.KindCarbon, domain.KindSCI, domain.KindFinops, domain.KindSustainability, domain.KindGHG},
		"formats":      h.Svc.Formats(r.Context(), c.TenantID),
		"max_days":     400,
		"pdf_rows":     2000,
		"preview_rows": application.PreviewRows,
	})
}

func (h Handlers) preview(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	q := r.URL.Query()
	var project *string
	if v := q.Get("project_id"); v != "" {
		if !httpx.ParseUUID(v) {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid project", "")
			return
		}
		project = &v
	}
	start, err1 := time.Parse(time.DateOnly, q.Get("period_start"))
	end, err2 := time.Parse(time.DateOnly, q.Get("period_end"))
	if err1 != nil || err2 != nil {
		fail(w, r, domain.ErrInvalidReport)
		return
	}
	p, err := h.Svc.Preview(r.Context(), c.TenantID, domain.Report{ProjectID: project, Kind: domain.Kind(q.Get("kind")), PeriodStart: start, PeriodEnd: end})
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

func (h Handlers) remove(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	if err := h.Svc.Delete(r.Context(), c.TenantID, id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) schedules(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	items, err := h.Svc.Schedules(r.Context(), c.TenantID)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) createSchedule(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		ProjectID  *string        `json:"project_id"`
		Kind       domain.Kind    `json:"kind"`
		Format     domain.Format  `json:"format"`
		Cadence    domain.Cadence `json:"cadence"`
		Recipients []string       `json:"recipients"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || (in.ProjectID != nil && !httpx.ParseUUID(*in.ProjectID)) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	s, err := h.Svc.CreateSchedule(r.Context(), c.TenantID, c.Subject, domain.Schedule{ProjectID: in.ProjectID, Kind: in.Kind, Format: in.Format, Cadence: in.Cadence, Recipients: in.Recipients})
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, s)
}

func (h Handlers) updateSchedule(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || in.Enabled == nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	s, err := h.Svc.SetScheduleEnabled(r.Context(), c.TenantID, id, *in.Enabled)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}

func (h Handlers) removeSchedule(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
		return
	}
	if err := h.Svc.DeleteSchedule(r.Context(), c.TenantID, id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
