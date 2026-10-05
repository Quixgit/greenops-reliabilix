// Package repository is the PostgreSQL adapter for stored usage.
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/usage/domain"
)

type Postgres struct{ Pool *pgxpool.Pool }

func (r Postgres) List(ctx context.Context, tenantID string, from, to time.Time, limit int) ([]domain.Record, error) {
	out := []domain.Record{}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, project_id, provider, resource_id, resource_type, service_name, service_category, region, usage_amount, usage_unit, cost, currency, recorded_at
			FROM usage.usage_records WHERE tenant_id = $1 AND recorded_at >= $2 AND recorded_at < $3
			ORDER BY recorded_at DESC LIMIT $4`, tenantID, from, to, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var x domain.Record
			if err := rows.Scan(&x.ID, &x.ProjectID, &x.Provider, &x.ResourceID, &x.ResourceType, &x.ServiceName, &x.ServiceCategory, &x.Region, &x.UsageAmount, &x.UsageUnit, &x.Cost, &x.Currency, &x.RecordedAt); err != nil {
				return err
			}
			out = append(out, x)
		}
		return rows.Err()
	})
	return out, err
}
