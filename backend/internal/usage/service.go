// Package usage stores and queries normalized usage records.
package usage

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	uhttp "github.com/quixgit/greenops-reliabilix/backend/internal/usage/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/usage/repository"
)

type Module struct{ h uhttp.Handlers }

func New(pool *pgxpool.Pool) *Module {
	return &Module{h: uhttp.Handlers{Repo: repository.Postgres{Pool: pool}}}
}

func (*Module) Name() string          { return "usage" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }
