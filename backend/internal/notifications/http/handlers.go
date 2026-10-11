// Package http exposes the notification endpoints: destinations, deliveries and the catalog of what can be
// configured.
package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

type Handlers struct{ Svc application.Service }

func (h Handlers) Routes(r chi.Router) {
	read, write := auth.Require(auth.PermNotificationRead), auth.Require(auth.PermNotificationWrite)
	r.With(read).Get("/notifications/catalog", h.catalog)
	r.With(read).Get("/notifications/destinations", h.list)
	r.With(write).Post("/notifications/destinations", h.create)
	r.With(write).Delete("/notifications/destinations/{id}", h.delete)
	r.With(write).Post("/notifications/destinations/{id}/enable", h.setEnabled(true))
	r.With(write).Post("/notifications/destinations/{id}/disable", h.setEnabled(false))
	r.With(write).Post("/notifications/destinations/{id}/test", h.test)
	r.With(read).Get("/notifications/deliveries", h.deliveries)
	// The bell: for every member, whatever role; the organization's display settings decide what is in it.
	r.With(auth.Require(auth.PermProjectRead)).Get("/inbox", h.inbox)
	r.With(auth.Require(auth.PermProjectRead)).Post("/inbox/read", h.inboxRead)
}

func (h Handlers) inbox(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	items, unread, err := h.Svc.InboxFeed(r.Context(), c.TenantID, c.Subject, 30)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "unread": unread,
		"sound": h.Svc.Setting().Bool(r.Context(), c.TenantID, settings.KeyNotifSound)})
}

func (h Handlers) inboxRead(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	if err := h.Svc.InboxRead(r.Context(), c.TenantID, c.Subject); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, domain.ErrNotConfigured):
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "notifications unavailable", err.Error())
	case errors.Is(err, entitlements.ErrLimitReached):
		httpx.WriteProblem(w, r, http.StatusPaymentRequired, "plan limit reached", err.Error())
	case errors.Is(err, domain.ErrLimit):
		httpx.WriteProblem(w, r, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, domain.ErrInvalid), errors.Is(err, domain.ErrUnknownKind):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid destination", err.Error())
	default:
		httpx.Internal(w, r, err)
	}
}

func id(w http.ResponseWriter, r *http.Request) (string, bool) {
	v, ok := httpx.PathUUID(r, "id")
	if !ok {
		httpx.NotFound(w, r)
	}
	return v, ok
}

func (h Handlers) catalog(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, h.Svc.Catalog(r.Context()))
}

func (h Handlers) list(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	items, err := h.Svc.ListDestinations(r.Context(), c.TenantID)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) create(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		Kind        domain.Kind     `json:"kind"`
		Name        string          `json:"name"`
		Target      string          `json:"target"`
		EventTypes  []string        `json:"event_types"`
		MinSeverity notify.Severity `json:"min_severity"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", "body must be a JSON object with kind, name and target")
		return
	}
	out, err := h.Svc.CreateDestination(r.Context(), c.TenantID, c, application.CreateInput{Kind: in.Kind, Name: in.Name, Target: in.Target,
		EventTypes: in.EventTypes, MinSeverity: in.MinSeverity})
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h Handlers) delete(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	did, ok := id(w, r)
	if !ok {
		return
	}
	if err := h.Svc.DeleteDestination(r.Context(), c.TenantID, did); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) setEnabled(enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.FromContext(r.Context())
		did, ok := id(w, r)
		if !ok {
			return
		}
		d, err := h.Svc.SetEnabled(r.Context(), c.TenantID, did, enabled)
		if err != nil {
			fail(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, d)
	}
}

func (h Handlers) test(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	did, ok := id(w, r)
	if !ok {
		return
	}
	deliveryID, err := h.Svc.Test(r.Context(), c.TenantID, did)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"delivery_id": deliveryID})
}

func (h Handlers) deliveries(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "limit must be 1-100")
			return
		}
		limit = n
	}
	items, err := h.Svc.ListDeliveries(r.Context(), c.TenantID, limit)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
