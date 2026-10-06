// Package ingestion normalizes raw provider data into FOCUS records and persists them (phase 1 core path:
// cloudaccounts -> ingestion -> usage -> carbon).
package ingestion

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/normalizers/awsce"
	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/normalizers/awsceusage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
)

type Module struct{ Svc application.Service }

// New wires the normalizer registry. archive may be nil (raw payloads are then not kept).
func New(store application.UsageStore, archive storage.Store, log *slog.Logger) *Module {
	return &Module{Svc: application.Service{
		Normalizers: map[string]application.Normalizer{
			"aws/cost_explorer":       awsce.Normalizer{},
			"aws/cost_explorer_usage": awsceusage.Normalizer{},
		},
		Store: store, Archive: archive, Log: log,
	}}
}

func (*Module) Name() string        { return "ingestion" }
func (*Module) Routes(r chi.Router) {}
