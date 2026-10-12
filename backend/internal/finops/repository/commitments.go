package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

// CommittableDaily is the compute and database spend per provider and day in the reporting currency (the organization's choice, else the
// dominant one of the window), by the billed and by the amortized price. An amount in a currency without a rate is left out, as in every
// other total. The window end is exclusive.
func (r Postgres) CommittableDaily(ctx context.Context, tenantID string, project *string, from, to time.Time) (days []domain.DayCost, currency string, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH cur AS (
			  SELECT finops.reporting_currency($1::uuid, (SELECT d.currency FROM usage.usage_records d
			    WHERE d.tenant_id = $1::uuid AND ($2::uuid IS NULL OR d.project_id = $2::uuid) AND d.recorded_at >= $3 AND d.recorded_at < $4
			    GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1))::text AS c)
			SELECT u.provider, u.recorded_at::date AS day, (SELECT c FROM cur),
			       COALESCE(sum((CASE WHEN u.currency::text = (SELECT c FROM cur) THEN u.billed_cost ELSE finops.convert_cost(u.billed_cost, u.currency::text, (SELECT c FROM cur), u.recorded_at::date) END)), 0)::float8,
			       COALESCE(sum((CASE WHEN u.currency::text = (SELECT c FROM cur) THEN u.effective_cost ELSE finops.convert_cost(u.effective_cost, u.currency::text, (SELECT c FROM cur), u.recorded_at::date) END)), 0)::float8
			FROM usage.usage_records u
			WHERE u.tenant_id = $1::uuid AND ($2::uuid IS NULL OR u.project_id = $2::uuid) AND u.recorded_at >= $3 AND u.recorded_at < $4
			  AND u.service_category IN ('compute', 'database')
			GROUP BY u.provider, u.recorded_at::date ORDER BY u.provider, day`, tenantID, project, from, to)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var d domain.DayCost
			var cur *string
			if err := rows.Scan(&d.Provider, &d.Day, &cur, &d.Committable, &d.Effective); err != nil {
				return err
			}
			if cur != nil {
				currency = *cur
			}
			days = append(days, d)
		}
		return rows.Err()
	})
	return
}
