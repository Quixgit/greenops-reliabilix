// Package http exposes an organization's settings: the schema with effective values, changes, reset and history.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	"github.com/quixgit/greenops-reliabilix/backend/internal/settings/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/settings/domain"
)

type Handlers struct{ Svc *application.Service }

func (h Handlers) Routes(r chi.Router) {
	read, write := auth.Require(auth.PermSettingsRead), auth.Require(auth.PermSettingsWrite)
	r.With(read).Get("/settings", h.get)
	r.With(write).Patch("/settings", h.patch)
	r.With(write).Post("/settings/reset", h.reset)
	r.With(read).Get("/settings/history", h.history)
}

// View is the API answer shared by tenant and operator handlers.
type View struct {
	Groups []settings.Group `json:"groups"`
	Items  []settings.Item  `json:"items"`
}

func Fail(w http.ResponseWriter, r *http.Request, err error) {
	var inv application.InvalidError
	switch {
	case errors.As(err, &inv):
		httpx.WriteValidation(w, r, "invalid settings", inv.Problems)
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r)
	default:
		httpx.Internal(w, r, err)
	}
}

func (h Handlers) view(w http.ResponseWriter, r *http.Request, status int) {
	c, _ := auth.FromContext(r.Context())
	items, err := h.Svc.TenantItems(r.Context(), c.TenantID, c.Can(auth.PermSettingsWrite))
	if err != nil {
		Fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, status, View{Groups: h.Svc.Groups(), Items: items})
}

func (h Handlers) get(w http.ResponseWriter, r *http.Request) { h.view(w, r, http.StatusOK) }

func (h Handlers) patch(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		Values map[string]json.RawMessage `json:"values"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || len(in.Values) == 0 || len(in.Values) > 100 {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", `body must be {"values": {"<key>": <value>, ...}}`)
		return
	}
	if err := h.Svc.SetTenant(r.Context(), c.TenantID, c.Subject, in.Values); err != nil {
		Fail(w, r, err)
		return
	}
	h.view(w, r, http.StatusOK)
}

func (h Handlers) reset(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		Keys []string `json:"keys"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || len(in.Keys) == 0 || len(in.Keys) > 100 {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request", `body must be {"keys": ["<key>", ...]}`)
		return
	}
	if err := h.Svc.ResetTenant(r.Context(), c.TenantID, c.Subject, in.Keys); err != nil {
		Fail(w, r, err)
		return
	}
	h.view(w, r, http.StatusOK)
}

func (h Handlers) history(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid query", "limit must be 1-200")
			return
		}
		limit = n
	}
	items, err := h.Svc.TenantHistory(r.Context(), c.TenantID, limit)
	if err != nil {
		Fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
