package app

import (
	"context"
	"errors"

	ingestapp "github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/application"
	usageapp "github.com/quixgit/greenops-reliabilix/backend/internal/usage/application"
	usagedomain "github.com/quixgit/greenops-reliabilix/backend/internal/usage/domain"
)

// importStore lets the ingestion module keep the records of uploaded cost files in the usage module without either importing the other.
type importStore struct{ usage usageapp.Service }

func fromUsage(i usagedomain.Import) ingestapp.ImportRecord {
	return ingestapp.ImportRecord{ID: i.ID, ProjectID: i.ProjectID, Vendor: i.Vendor, Filename: i.Filename, Currency: i.Currency, Status: i.Status, Error: i.Error,
		RowsRead: i.RowsRead, RowsImported: i.RowsImported, RowsSkipped: i.RowsSkipped, Services: i.Services, From: i.From, To: i.To, TotalCost: i.TotalCost,
		Warnings: i.Warnings, CreatedBy: i.CreatedBy, CreatedAt: i.CreatedAt, FinishedAt: i.FinishedAt}
}

func (s importStore) CreateImport(ctx context.Context, tenantID string, r ingestapp.ImportRecord) (ingestapp.ImportRecord, error) {
	i, err := s.usage.CreateImport(ctx, tenantID, usagedomain.Import{ProjectID: r.ProjectID, Vendor: r.Vendor, Filename: r.Filename, Currency: r.Currency, CreatedBy: r.CreatedBy})
	if errors.Is(err, usagedomain.ErrImportProject) {
		return ingestapp.ImportRecord{}, ingestapp.ErrNotFound
	}
	return fromUsage(i), err
}

func (s importStore) GetImport(ctx context.Context, tenantID, id string) (ingestapp.ImportRecord, error) {
	i, err := s.usage.GetImport(ctx, tenantID, id)
	return fromUsage(i), err
}

func (s importStore) ListImports(ctx context.Context, tenantID string, limit int) ([]ingestapp.ImportRecord, error) {
	items, err := s.usage.ListImports(ctx, tenantID, limit)
	out := make([]ingestapp.ImportRecord, 0, len(items))
	for _, i := range items {
		out = append(out, fromUsage(i))
	}
	return out, err
}

func (s importStore) FinishImport(ctx context.Context, tenantID, id string, res *ingestapp.ImportOutcome, failure string) error {
	if res == nil {
		return s.usage.FinishImport(ctx, tenantID, id, nil, failure)
	}
	return s.usage.FinishImport(ctx, tenantID, id, &usagedomain.ImportResult{RowsRead: res.RowsRead, RowsImported: res.RowsImported, RowsSkipped: res.RowsSkipped,
		Services: res.Services, From: res.From, To: res.To, Currency: res.Currency, TotalCost: res.TotalCost, Warnings: res.Warnings}, failure)
}

func (s importStore) MarkImportDeleting(ctx context.Context, tenantID, id string) error {
	return s.usage.MarkImportDeleting(ctx, tenantID, id)
}

func (s importStore) DeleteImport(ctx context.Context, tenantID, id string) error {
	return s.usage.DeleteImport(ctx, tenantID, id)
}
