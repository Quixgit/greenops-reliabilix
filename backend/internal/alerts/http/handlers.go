// Package http exposes the alert endpoints.
package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/alerts/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/alerts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

type Handlers struct{ Svc application.Service }

func (h Handlers) Routes(r chi.Router) {
	read, write := auth.Require(auth.PermProjectRead), auth.Require(auth.PermAlertWrite)
	r.With(read).Get("/alerts", h.list)
	r.With(write).Post("/alerts/{id}/acknowledge", h.change("acknowledge"))
	r.With(write).Post("/alerts/{id}/mute", h.change("mute"))
	r.With(write).Post("/alerts/{id}/unmute", h.change("unmute"))
}

func (h Handlers) list(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	status := r.URL.Query().Get("status")
	switch status {
	case "", "new", "acknowledged", "muted", "resolved":
	default:
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid status", "")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	items, err := h.Svc.List(r.Context(), c.TenantID, status, limit)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) change(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.FromContext(r.Context())
		if c.KeyID != "" { // decisions about alerts are made by people
			httpx.WriteProblem(w, r, http.StatusForbidden, "forbidden", "")
			return
		}
		id, ok := httpx.PathUUID(r, "id")
		if !ok {
			httpx.NotFound(w, r)
			return
		}
		var (
			a   domain.Alert
			err error
		)
		switch action {
		case "acknowledge":
			a, err = h.Svc.Acknowledge(r.Context(), c.TenantID, id, c.Subject)
		case "mute":
			a, err = h.Svc.Mute(r.Context(), c.TenantID, id, c.Subject)
		default:
			a, err = h.Svc.Unmute(r.Context(), c.TenantID, id, c.Subject)
		}
		switch {
		case errors.Is(err, domain.ErrNotFound):
			httpx.NotFound(w, r)
		case errors.Is(err, domain.ErrInvalidTransition):
			httpx.WriteProblem(w, r, http.StatusConflict, "invalid status transition", err.Error())
		case err != nil:
			httpx.Internal(w, r, err)
		default:
			httpx.WriteJSON(w, http.StatusOK, a)
		}
	}
}
