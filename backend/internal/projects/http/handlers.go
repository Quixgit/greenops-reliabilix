// Package http exposes the project endpoints.
package http

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/domain"
)

type Handlers struct{ Svc application.Service }

func (h Handlers) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermProjectRead)).Get("/projects", h.list)
	r.With(auth.Require(auth.PermProjectWrite)).Post("/projects", h.create)
}

func (h Handlers) list(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context()) // tenant always from the verified token
	ps, err := h.Svc.List(r.Context(), c.TenantID)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": ps})
}

func (h Handlers) create(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		Name           string `json:"name"`
		FunctionalUnit string `json:"functional_unit"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	p, err := h.Svc.Create(r.Context(), c.TenantID, in.Name, in.FunctionalUnit)
	switch {
	case errors.Is(err, domain.ErrInvalidName), errors.Is(err, domain.ErrInvalidUnit):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid project", err.Error())
	case errors.Is(err, domain.ErrDuplicate):
		httpx.WriteProblem(w, r, http.StatusConflict, "project already exists", "")
	case err != nil:
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
	default:
		httpx.WriteJSON(w, http.StatusCreated, p)
	}
}
