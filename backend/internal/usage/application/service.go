// Package application holds the usage use-cases.
package application

import (
	"context"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
	"github.com/quixgit/greenops-reliabilix/backend/internal/usage/domain"
)

type Service struct{ Repo domain.Repository }

func (s Service) Upsert(ctx context.Context, tenantID, projectID, connectionID string, recs []focus.Record) error {
	return s.Repo.Upsert(ctx, tenantID, projectID, connectionID, recs)
}

func (s Service) List(ctx context.Context, tenantID string, projectID *string, from, to time.Time, limit int) ([]domain.Record, error) {
	return s.Repo.List(ctx, tenantID, projectID, from, to, limit)
}

func (s Service) DailyAggregates(ctx context.Context, tenantID, projectID string, from, to time.Time) ([]domain.DailyAggregate, error) {
	return s.Repo.DailyAggregates(ctx, tenantID, projectID, from, to)
}

// ReplaceTagged lets the ingestion pipeline store cost by tag (it finds this method by its interface).
func (s Service) ReplaceTagged(ctx context.Context, tenantID, projectID, sourceKey string, from, to time.Time, rows []focus.TaggedCost) error {
	return s.Repo.ReplaceTagged(ctx, tenantID, projectID, sourceKey, from, to, rows)
}

// The imports of vendor cost files (ADR-0062): thin passes to the repository, which the ingestion module reaches through its own port.
func (s Service) CreateImport(ctx context.Context, tenantID string, i domain.Import) (domain.Import, error) {
	return s.Repo.CreateImport(ctx, tenantID, i)
}
func (s Service) GetImport(ctx context.Context, tenantID, id string) (domain.Import, error) {
	return s.Repo.GetImport(ctx, tenantID, id)
}
func (s Service) ListImports(ctx context.Context, tenantID string, limit int) ([]domain.Import, error) {
	return s.Repo.ListImports(ctx, tenantID, limit)
}
func (s Service) FinishImport(ctx context.Context, tenantID, id string, res *domain.ImportResult, failure string) error {
	return s.Repo.FinishImport(ctx, tenantID, id, res, failure)
}
func (s Service) MarkImportDeleting(ctx context.Context, tenantID, id string) error {
	return s.Repo.MarkImportDeleting(ctx, tenantID, id)
}
func (s Service) DeleteImport(ctx context.Context, tenantID, id string) error {
	return s.Repo.DeleteImport(ctx, tenantID, id)
}

// DailyPool and PoolServices let the Kubernetes module read what its clusters cost (through its own port, wired in the composition root).
func (s Service) DailyPool(ctx context.Context, tenantID, projectID string, services []string, from, to time.Time) ([]domain.PoolDay, string, error) {
	return s.Repo.DailyPool(ctx, tenantID, projectID, services, from, to)
}
func (s Service) PoolServices(ctx context.Context, tenantID, projectID string, from, to time.Time) ([]domain.PoolService, string, error) {
	return s.Repo.PoolServices(ctx, tenantID, projectID, from, to)
}
