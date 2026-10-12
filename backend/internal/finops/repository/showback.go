package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

// reportingCTE computes the reporting currency of the window once (the same rule as every other cost query: the organization's choice, else the
// currency most of the bill is in) and the whole bill in it. $1 tenant, $2 project or null, $3 from, $4 to.
const reportingCTE = `
	cur AS (SELECT finops.reporting_currency($1::uuid, (SELECT d.currency FROM usage.usage_records d
	    WHERE d.tenant_id = $1::uuid AND ($2::uuid IS NULL OR d.project_id = $2::uuid) AND d.recorded_at >= $3 AND d.recorded_at < $4
	    GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1))::text AS c),
	bill AS (
	  SELECT COALESCE(sum(CASE WHEN g.currency::text = (SELECT c FROM cur) THEN g.cost ELSE finops.convert_cost(g.cost, g.currency::text, (SELECT c FROM cur), g.day) END), 0)::float8 AS total
	  FROM (SELECT currency, recorded_at::date AS day, sum(billed_cost) AS cost FROM usage.usage_records
	        WHERE tenant_id = $1::uuid AND ($2::uuid IS NULL OR project_id = $2::uuid) AND recorded_at >= $3 AND recorded_at < $4
	        GROUP BY 1, 2) g)`

func (r Postgres) TagKeys(ctx context.Context, tenantID string, project *string, from, to time.Time) (out []domain.TagKeyInfo, currency string, err error) {
	out = []domain.TagKeyInfo{}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `WITH `+reportingCTE+`
			SELECT t.tag_key, count(DISTINCT t.tag_value)::int, COALESCE((SELECT c FROM cur), ''),
			       COALESCE(sum(CASE WHEN t.currency::text = (SELECT c FROM cur) THEN t.billed_cost ELSE finops.convert_cost(t.billed_cost, t.currency::text, (SELECT c FROM cur), t.day) END), 0)::float8,
			       (SELECT total FROM bill)
			FROM usage.tagged_cost t
			WHERE t.tenant_id = $1::uuid AND ($2::uuid IS NULL OR t.project_id = $2::uuid) AND t.day >= $3::date AND t.day < $4::date
			GROUP BY t.tag_key ORDER BY 4 DESC, 1`, tenantID, project, from, to)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var k domain.TagKeyInfo
			var tagged, total float64
			if err := rows.Scan(&k.Key, &k.Values, &currency, &tagged, &total); err != nil {
				return err
			}
			if total > 0 {
				k.Coverage = min(100, tagged/total*100)
			}
			out = append(out, k)
		}
		return rows.Err()
	})
	return
}

func (r Postgres) Showback(ctx context.Context, tenantID string, project *string, tag string, from, to time.Time) (sb domain.Showback, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		var lines []domain.TagCost
		var currency string
		var total float64
		rows, err := tx.Query(ctx, `WITH `+reportingCTE+`
			SELECT t.tag_value, t.service_name, COALESCE((SELECT c FROM cur), ''), (SELECT total FROM bill),
			       COALESCE(sum(CASE WHEN t.currency::text = (SELECT c FROM cur) THEN t.billed_cost ELSE finops.convert_cost(t.billed_cost, t.currency::text, (SELECT c FROM cur), t.day) END), 0)::float8
			FROM usage.tagged_cost t
			WHERE t.tenant_id = $1::uuid AND ($2::uuid IS NULL OR t.project_id = $2::uuid) AND t.day >= $3::date AND t.day < $4::date AND t.tag_key = $5
			GROUP BY 1, 2`, tenantID, project, from, to, tag)
		if err != nil {
			return err
		}
		for rows.Next() {
			var l domain.TagCost
			if err := rows.Scan(&l.Value, &l.Service, &currency, &total, &l.Cost); err != nil {
				rows.Close()
				return err
			}
			lines = append(lines, l)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(lines) == 0 { // nothing carries this key: the bill alone, in its reporting currency
			e := tx.QueryRow(ctx, `WITH `+reportingCTE+` SELECT COALESCE((SELECT c FROM cur), ''), (SELECT total FROM bill)`, tenantID, project, from, to).Scan(&currency, &total)
			if e != nil {
				return e
			}
		}
		sb = domain.BuildShowback(tag, currency, lines, total)
		var first *time.Time
		if e := tx.QueryRow(ctx, `SELECT min(day)::timestamptz FROM usage.tagged_cost WHERE tenant_id = $1::uuid AND ($2::uuid IS NULL OR project_id = $2::uuid) AND tag_key = $3`,
			tenantID, project, tag).Scan(&first); e == nil && first != nil {
			sb.TagsFrom = first.UTC().Format(time.DateOnly)
		}
		return nil
	})
	return
}
