// Package kubernetes keeps the energy of Kubernetes workloads measured by a customer-side agent (ADR-0030). It is
// attribution inside the nodes' energy, which the cloud bill already contains: the figures are never added to totals.
package kubernetes

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes/domain"
	khttp "github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes/repository"
)

type Deps struct {
	Pool      *pgxpool.Pool
	Intensity domain.IntensitySource
}

type Module struct{ h khttp.Handlers }

func New(d Deps) *Module {
	return &Module{h: khttp.Handlers{Svc: application.Service{Repo: repository.Postgres{Pool: d.Pool}, Intensity: d.Intensity}}}
}

func (*Module) Name() string          { return "kubernetes" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }
