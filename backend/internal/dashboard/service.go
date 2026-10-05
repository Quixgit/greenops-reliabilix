// Package dashboard serves the read models of the Overview screen.
package dashboard

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dhttp "github.com/quixgit/greenops-reliabilix/backend/internal/dashboard/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/dashboard/repository"
)

type Module struct{ h dhttp.Handlers }

func New(pool *pgxpool.Pool) *Module {
	return &Module{h: dhttp.Handlers{Repo: repository.Postgres{Pool: pool}}}
}

func (*Module) Name() string          { return "dashboard" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }
