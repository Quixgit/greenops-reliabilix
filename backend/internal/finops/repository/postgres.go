// Package repository is the PostgreSQL adapter for cost aggregates.
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

type Postgres struct{ Pool *pgxpool.Pool }

// Summary aggregates cost by service for the window and totals the previous
// window of equal length. Phase 1 assumes a single billing currency: when
// several exist, only the dominant one is reported (FX normalization: phase 2).
func (r Postgres) Summary(ctx context.Context, tenantID string, project *string, from, to time.Time) (domain.Summary, error) {
	s := domain.Summary{ByService: []domain.Line{}}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT service_name, currency, sum(cost) FROM usage.usage_records
			WHERE tenant_id = $1 AND ($4::uuid IS NULL OR project_id = $4) AND recorded_at >= $2 AND recorded_at < $3
			GROUP BY service_name, currency ORDER BY sum(cost) DESC LIMIT 50`, tenantID, from, to, project)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var l domain.Line
			var cur string
			if err := rows.Scan(&l.Service, &cur, &l.Cost); err != nil {
				return err
			}
			if s.Currency == "" {
				s.Currency = cur
			}
			if cur == s.Currency {
				s.TotalCost += l.Cost
				s.ByService = append(s.ByService, l)
				s.HasData = true
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		prevFrom := from.Add(-to.Sub(from))
		return tx.QueryRow(ctx, `SELECT COALESCE(sum(cost),0) FROM usage.usage_records
			WHERE tenant_id = $1 AND ($4::uuid IS NULL OR project_id = $4) AND recorded_at >= $2 AND recorded_at < $3
			AND ($5 = '' OR currency = $5)`, tenantID, prevFrom, from, project, s.Currency).Scan(&s.PreviousTotalCost)
	})
	return s, err
}
