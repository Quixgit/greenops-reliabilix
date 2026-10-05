// Package cloudaccounts is the cloud connections domain module (AWS/Azure/GCP).
package cloudaccounts

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	chttp "github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/providers/aws"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

type Module struct {
	h   chttp.Handlers
	log *slog.Logger
}

func New(pool *pgxpool.Pool, log *slog.Logger) *Module {
	svc := application.Service{
		Repo:      repository.Postgres{Pool: pool},
		Providers: domain.NewRegistry(aws.Provider{}), // phase 2: azure, gcp
	}
	return &Module{h: chttp.Handlers{Svc: svc}, log: log}
}

func (*Module) Name() string          { return "cloudaccounts" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }

// RegisterJobs implements queue.JobRegistrar.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	mux.HandleFunc(queue.TaskSyncAll, m.syncAll)
	mux.HandleFunc(queue.TaskSyncAWSAccount, m.syncAWS)
}

// TODO: list healthy connections with the privileged worker role and enqueue one sync_aws_account per connection.
func (m *Module) syncAll(_ context.Context, _ *asynq.Task) error {
	m.log.Warn("sync_all not implemented yet")
	return nil
}

// TODO: assume role -> Cost Explorer -> store raw in S3 -> hand to ingestion.
func (m *Module) syncAWS(_ context.Context, t *asynq.Task) error {
	if _, err := queue.Decode(t); err != nil {
		return err
	}
	observability.CloudSyncTotal.WithLabelValues("aws", "skipped").Inc()
	m.log.Warn("sync_aws_account not implemented yet")
	return nil
}
