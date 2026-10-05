// Package finops is the cost-side domain module (spend, budgets, anomalies).
package finops

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	fhttp "github.com/quixgit/greenops-reliabilix/backend/internal/finops/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/repository"
)

type Module struct{ h fhttp.Handlers }

func New(pool *pgxpool.Pool) *Module {
	return &Module{h: fhttp.Handlers{Repo: repository.Postgres{Pool: pool}}}
}

func (*Module) Name() string          { return "finops" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }
