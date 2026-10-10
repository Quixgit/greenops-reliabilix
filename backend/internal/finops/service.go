// Package finops is the cost-side domain module: spend summary, budgets and anomalies.
package finops

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/application"
	fhttp "github.com/quixgit/greenops-reliabilix/backend/internal/finops/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/repository"
)

type Module struct {
	h fhttp.Handlers
	// Svc is exported for the composition root, which adapts it to the ports of other modules.
	Svc application.Service
}

func New(pool *pgxpool.Pool) *Module {
	svc := application.Service{Repo: repository.Postgres{Pool: pool}}
	return &Module{h: fhttp.Handlers{Svc: svc}, Svc: svc}
}

func (*Module) Name() string          { return "finops" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }
