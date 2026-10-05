// Package transport exposes the project service over HTTP.
package transport

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/quixgit/greenops-reliabilix/pkg/auth"
	"github.com/quixgit/greenops-reliabilix/pkg/httpx"
	"github.com/quixgit/greenops-reliabilix/services/project/internal/domain"
)

type Handler struct{ Repo domain.Repository }

func (h Handler) Register(mux *http.ServeMux) {
	mux.Handle("GET /v1/projects", auth.Require(auth.PermProjectRead)(http.HandlerFunc(h.list)))
	mux.Handle("POST /v1/projects", auth.Require(auth.PermProjectWrite)(http.HandlerFunc(h.create)))
}

func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	ps, err := h.Repo.List(r.Context(), c.TenantID) // tenant always comes from the token, never from input
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": ps})
}

func (h Handler) create(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var in struct {
		Name string `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	p, err := domain.NewProject(hex.EncodeToString(b), c.TenantID, in.Name, time.Now().UTC())
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid project", err.Error())
		return
	}
	switch err := h.Repo.Create(r.Context(), p); {
	case errors.Is(err, domain.ErrDuplicate):
		httpx.WriteProblem(w, r, http.StatusConflict, "project already exists", "")
	case err != nil:
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
	default:
		httpx.WriteJSON(w, http.StatusCreated, p)
	}
}
