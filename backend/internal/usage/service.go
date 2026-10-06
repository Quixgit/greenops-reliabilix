// Package usage stores and queries normalized FOCUS usage records.
package usage

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/usage/application"
	uhttp "github.com/quixgit/greenops-reliabilix/backend/internal/usage/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/usage/repository"
)

type Module struct {
	h   uhttp.Handlers
	Svc application.Service // consumed by the composition root (ingestion store, carbon source)
}

func New(pool *pgxpool.Pool) *Module {
	svc := application.Service{Repo: repository.Postgres{Pool: pool}}
	return &Module{h: uhttp.Handlers{Svc: svc}, Svc: svc}
}

func (*Module) Name() string          { return "usage" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }
