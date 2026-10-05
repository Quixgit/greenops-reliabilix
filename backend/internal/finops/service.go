// Package finops is the cost-side domain module: spend summary, budgets and anomalies.
package finops

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/application"
	fhttp "github.com/quixgit/greenops-reliabilix/backend/internal/finops/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/repository"
)

type Module struct{ h fhttp.Handlers }

func New(pool *pgxpool.Pool) *Module {
	return &Module{h: fhttp.Handlers{Svc: application.Service{Repo: repository.Postgres{Pool: pool}}}}
}

func (*Module) Name() string          { return "finops" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }
