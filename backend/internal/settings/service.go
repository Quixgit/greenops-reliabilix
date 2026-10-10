// Package settings is the configuration engine's module (ADR-0023): storage, inheritance and the management API of
// settings. The catalog of settings is platform/settings; domains read values through settings.Reader.
package settings

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/settings/application"
	shttp "github.com/quixgit/greenops-reliabilix/backend/internal/settings/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/settings/repository"
)

type Module struct {
	h   shttp.Handlers
	Svc *application.Service
}

func New(pool *pgxpool.Pool, log *slog.Logger) *Module {
	svc := &application.Service{Repo: repository.Postgres{Pool: pool}, Log: log}
	return &Module{h: shttp.Handlers{Svc: svc}, Svc: svc}
}

func (*Module) Name() string          { return "settings" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }
