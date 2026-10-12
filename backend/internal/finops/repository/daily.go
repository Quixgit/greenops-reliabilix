package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

// DailyCostByProvider is the cost per cloud and day of [from, to) in the given currency, or, when it is empty, in the reporting currency (the
// organization's choice, else the dominant one of the window). Days without data are absent; an amount in a currency without a rate is left out.
func (r Postgres) DailyCostByProvider(ctx context.Context, tenantID string, project *string, currency string, from, to time.Time) (out []domain.ProviderDays, cur string, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH cur AS (SELECT COALESCE(NULLIF($5::text, ''), finops.reporting_currency($1::uuid, (SELECT d.currency FROM usage.usage_records d
			    WHERE d.tenant_id = $1::uuid AND ($2::uuid IS NULL OR d.project_id = $2::uuid) AND d.recorded_at >= $3 AND d.recorded_at < $4
			    GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1)))::text AS c)
			SELECT u.provider, u.recorded_at::date, (SELECT c FROM cur),
			       COALESCE(sum((CASE WHEN u.currency::text = (SELECT c FROM cur) THEN u.billed_cost ELSE finops.convert_cost(u.billed_cost, u.currency::text, (SELECT c FROM cur), u.recorded_at::date) END)), 0)::float8
			FROM usage.usage_records u
			WHERE u.tenant_id = $1::uuid AND ($2::uuid IS NULL OR u.project_id = $2::uuid) AND u.recorded_at >= $3 AND u.recorded_at < $4
			GROUP BY u.provider, u.recorded_at::date ORDER BY 1, 2`, tenantID, project, from, to, currency)
		if err != nil {
			return err
		}
		defer rows.Close()
		idx := map[string]int{}
		for rows.Next() {
			var prov string
			var p domain.DayPoint
			var c *string
			if err := rows.Scan(&prov, &p.Day, &c, &p.Cost); err != nil {
				return err
			}
			if c != nil {
				cur = *c
			}
			i, ok := idx[prov]
			if !ok {
				i = len(out)
				idx[prov] = i
				out = append(out, domain.ProviderDays{Provider: prov})
			}
			out[i].Days = append(out[i].Days, p)
		}
		return rows.Err()
	})
	return
}

// AnomalyDrivers names what moved on an anomalous day: per service and region, the cost of the day against the daily average of the
// 14 days before it, the biggest increases first. A pair with nothing before it is marked new.
func (r Postgres) AnomalyDrivers(ctx context.Context, tenantID string, project *string, provider string, d time.Time, limit int) (out []domain.Driver, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH w AS (SELECT $3::date AS d),
			cur AS (SELECT finops.reporting_currency($1::uuid, (SELECT x.currency FROM usage.usage_records x, w
			    WHERE x.tenant_id = $1::uuid AND ($2::uuid IS NULL OR x.project_id = $2::uuid) AND x.recorded_at >= w.d - 14 AND x.recorded_at < w.d + 1
			    GROUP BY x.currency ORDER BY sum(abs(x.billed_cost)) DESC, x.currency LIMIT 1))::text AS c),
			base_days AS (SELECT GREATEST(1, count(DISTINCT x.recorded_at::date)) AS n FROM usage.usage_records x, w
			    WHERE x.tenant_id = $1::uuid AND ($2::uuid IS NULL OR x.project_id = $2::uuid) AND ($5::text = '' OR x.provider = $5::text) AND x.recorded_at >= w.d - 14 AND x.recorded_at < w.d)
			SELECT service_name, region_id, today, expected FROM (
			  SELECT u.service_name, u.region_id,
			         COALESCE(sum(CASE WHEN u.recorded_at::date = w.d THEN (CASE WHEN u.currency::text = (SELECT c FROM cur) THEN u.billed_cost ELSE finops.convert_cost(u.billed_cost, u.currency::text, (SELECT c FROM cur), u.recorded_at::date) END) END), 0)::float8 AS today,
			         (COALESCE(sum(CASE WHEN u.recorded_at::date < w.d THEN (CASE WHEN u.currency::text = (SELECT c FROM cur) THEN u.billed_cost ELSE finops.convert_cost(u.billed_cost, u.currency::text, (SELECT c FROM cur), u.recorded_at::date) END) END), 0) / (SELECT n FROM base_days))::float8 AS expected
			  FROM usage.usage_records u, w
			  WHERE u.tenant_id = $1::uuid AND ($2::uuid IS NULL OR u.project_id = $2::uuid) AND ($5::text = '' OR u.provider = $5::text) AND u.recorded_at >= w.d - 14 AND u.recorded_at < w.d + 1
			  GROUP BY u.service_name, u.region_id) x
			WHERE today - expected > 0
			ORDER BY today - expected DESC LIMIT $4`, tenantID, project, d, limit, provider)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var x domain.Driver
			if err := rows.Scan(&x.Service, &x.Region, &x.Cost, &x.Expected); err != nil {
				return err
			}
			x.Delta = x.Cost - x.Expected
			x.New = x.Expected < 0.01
			out = append(out, x)
		}
		return rows.Err()
	})
	return
}

// AnomaliesByProvider lists the days whose cost of one cloud exceeds that cloud's trailing mean by more than three standard deviations (at least
// seven prior days of that cloud). Each cloud is judged on its own rhythm: a cloud that returns after a gap, or a small one next to a big one,
// does not hide or fake a spike in the sum. Amounts are in the reporting currency of the window.
func (r Postgres) AnomaliesByProvider(ctx context.Context, tenantID string, project *string, from, to time.Time) (out []domain.Anomaly, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH cur AS (SELECT finops.reporting_currency($1::uuid, (SELECT d.currency FROM usage.usage_records d
			    WHERE d.tenant_id = $1::uuid AND ($2::uuid IS NULL OR d.project_id = $2::uuid) AND d.recorded_at >= $3 AND d.recorded_at < $4
			    GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1))::text AS c),
			d AS (
			  SELECT u.provider, u.recorded_at::date AS day,
			         COALESCE(sum((CASE WHEN u.currency::text = (SELECT c FROM cur) THEN u.billed_cost ELSE finops.convert_cost(u.billed_cost, u.currency::text, (SELECT c FROM cur), u.recorded_at::date) END)), 0)::float8 AS cost
			  FROM usage.usage_records u
			  WHERE u.tenant_id = $1::uuid AND ($2::uuid IS NULL OR u.project_id = $2::uuid) AND u.recorded_at >= $3 AND u.recorded_at < $4
			  GROUP BY 1, 2
			), s AS (
			  SELECT provider, day, cost, avg(cost) OVER w AS mean, stddev_samp(cost) OVER w AS sd, count(*) OVER w AS n
			  FROM d WINDOW w AS (PARTITION BY provider ORDER BY day ROWS BETWEEN 14 PRECEDING AND 1 PRECEDING)
			)
			SELECT provider, day, cost, mean::float8, sd::float8 FROM s
			WHERE n >= 7 AND sd > 0 AND cost > mean + 3 * sd ORDER BY day DESC, provider LIMIT 20`, tenantID, project, from, to)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a domain.Anomaly
			if err := rows.Scan(&a.Provider, &a.Day, &a.Cost, &a.Mean, &a.Stddev); err != nil {
				return err
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	if out == nil {
		out = []domain.Anomaly{}
	}
	return
}
