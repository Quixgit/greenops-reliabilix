// Package ingestion normalizes raw provider data into FOCUS records and persists them (phase 1 core path:
// cloudaccounts -> ingestion -> usage -> carbon).
package ingestion

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/application"
	ihttp "github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/normalizers/awsce"
	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/normalizers/awsceusage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/normalizers/awsexport"
	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/normalizers/azurecost"
	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/normalizers/azureexport"
	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/normalizers/costcsv"
	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/normalizers/gcpbilling"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
)

type Module struct {
	Svc     application.Service
	imports *application.Imports
}

// EnableImports turns on the upload of vendor cost files: routes and the two jobs. Before the routes are served.
func (m *Module) EnableImports(store application.ImportStore, files storage.Store, q queue.Enqueuer, tagKeys func(ctx context.Context, tenantID string) []string) {
	m.imports = &application.Imports{Store: store, Files: files, Queue: q, Pipeline: m.Svc, TagKeys: tagKeys}
}

// RegisterJobs implements queue.JobRegistrar.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	if m.imports == nil {
		return
	}
	mux.HandleFunc(queue.TaskImportCost, func(ctx context.Context, t *asynq.Task) error {
		p, err := queue.Decode(t)
		if err != nil {
			return err
		}
		return m.imports.RunImport(ctx, p.TenantID, p.ProjectID, p.RefID)
	})
	mux.HandleFunc(queue.TaskDeleteImport, func(ctx context.Context, t *asynq.Task) error {
		p, err := queue.Decode(t)
		if err != nil {
			return err
		}
		return m.imports.RunDelete(ctx, p.TenantID, p.RefID)
	})
}

// New wires the normalizer registry. archive may be nil (raw payloads are then not kept).
func New(store application.UsageStore, archive storage.Store, log *slog.Logger) *Module {
	return &Module{Svc: application.Service{
		Normalizers: map[string]application.Normalizer{
			"aws/cost_explorer":         awsce.Normalizer{},
			"aws/cost_explorer_usage":   awsceusage.Normalizer{},
			"aws/focus_export":          awsexport.Normalizer{},
			"azure/focus_export":        azureexport.Normalizer{},
			"azure/cost_management_api": azurecost.Normalizer{},
			"gcp/bigquery_export":       gcpbilling.Normalizer{},
			"saas/cost_csv":             costcsv.Normalizer{},
		},
		Store: store, Archive: archive, Log: log,
	}}
}

func (*Module) Name() string { return "ingestion" }
func (m *Module) Routes(r chi.Router) {
	if m.imports != nil {
		ihttp.Handlers{Imports: *m.imports}.Routes(r)
	}
}
