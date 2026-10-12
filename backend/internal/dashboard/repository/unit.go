package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/dashboard/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/methodology"
)

// UnitEconomics reads, per project and month, the cost and carbon of the days the customer reported units for. The window is the last
// `months` calendar months including the current one; the currency is the reporting one of the window.
func (r Postgres) UnitEconomics(ctx context.Context, tenantID string, project *string, months int) (out domain.UnitEconomics, err error) {
	now := time.Now().UTC()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -(months - 1), 0)
	to := now.Truncate(24*time.Hour).AddDate(0, 0, 1)
	out.Projects = []domain.UnitProject{}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH cur AS (SELECT finops.reporting_currency($1::uuid, (SELECT d.currency FROM usage.usage_records d
			    WHERE d.tenant_id = $1::uuid AND ($2::uuid IS NULL OR d.project_id = $2::uuid) AND d.recorded_at >= $3 AND d.recorded_at < $4
			    GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1))::text AS c),
			carbon_d AS (
			  SELECT x.project_id, date_trunc('day', x.period_start) AS day, sum(x.carbon_kg_co2e) AS kg, max(x.functional_units) AS units
			  FROM carbon.calculations x
			  WHERE x.tenant_id = $1::uuid AND ($2::uuid IS NULL OR x.project_id = $2::uuid) AND x.methodology_version = $5
			    AND x.period_start >= $3 AND x.period_start < $4
			  GROUP BY 1, 2 HAVING max(x.functional_units) IS NOT NULL
			), cost_d AS (
			  SELECT u.project_id, date_trunc('day', u.recorded_at) AS day,
			         COALESCE(sum((CASE WHEN u.currency::text = (SELECT c FROM cur) THEN u.billed_cost ELSE finops.convert_cost(u.billed_cost, u.currency::text, (SELECT c FROM cur), u.recorded_at::date) END)), 0) AS cost
			  FROM usage.usage_records u
			  WHERE u.tenant_id = $1::uuid AND ($2::uuid IS NULL OR u.project_id = $2::uuid) AND u.recorded_at >= $3 AND u.recorded_at < $4
			  GROUP BY 1, 2
			)
			SELECT c.project_id::text, p.name, p.functional_unit, to_char(date_trunc('month', c.day), 'YYYY-MM'), count(*)::int,
			       sum(c.units)::float8, COALESCE(sum(k.cost), 0)::float8, (sum(c.kg) * 1000)::float8, (SELECT c FROM cur)
			FROM carbon_d c JOIN projects.projects p ON p.id = c.project_id LEFT JOIN cost_d k ON k.project_id = c.project_id AND k.day = c.day
			GROUP BY 1, 2, 3, 4 ORDER BY 2, 1, 4`, tenantID, project, from, to, methodology.Version)
		if err != nil {
			return err
		}
		defer rows.Close()
		idx := map[string]int{}
		for rows.Next() {
			var id, name, unit, month string
			var m domain.UnitMonth
			var cur *string
			if err := rows.Scan(&id, &name, &unit, &month, &m.Days, &m.Units, &m.Cost, &m.CarbonG, &cur); err != nil {
				return err
			}
			if cur != nil {
				out.Currency = *cur
			}
			m.Month = month
			i, ok := idx[id]
			if !ok {
				i = len(out.Projects)
				idx[id] = i
				out.Projects = append(out.Projects, domain.UnitProject{ProjectID: id, Name: name, Unit: unit, Months: []domain.UnitMonth{}})
			}
			out.Projects[i].Months = append(out.Projects[i].Months, m)
		}
		return rows.Err()
	})
	out.Finish()
	return
}
