-- name: CostByService :many
-- Cost per service in the DOMINANT currency of the window (largest total absolute cost). Amounts in other currencies
-- are converted at the day's ECB reference rate; an amount without a rate is left out (never guessed).
SELECT u.service_name, finops.reporting_currency(sqlc.arg(tenant_id), (SELECT d.currency FROM usage.usage_records d
    WHERE d.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR d.project_id = sqlc.narg(project_id))
      AND d.recorded_at >= sqlc.arg(from_ts) AND d.recorded_at < sqlc.arg(to_ts)
    GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1))::text AS currency,
       COALESCE(sum(finops.convert_cost(u.billed_cost, u.currency::text, finops.reporting_currency(sqlc.arg(tenant_id), (SELECT d.currency FROM usage.usage_records d
    WHERE d.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR d.project_id = sqlc.narg(project_id))
      AND d.recorded_at >= sqlc.arg(from_ts) AND d.recorded_at < sqlc.arg(to_ts)
    GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1)), u.recorded_at::date)), 0)::float8 AS cost
FROM usage.usage_records u
WHERE u.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR u.project_id = sqlc.narg(project_id))
  AND u.recorded_at >= sqlc.arg(from_ts) AND u.recorded_at < sqlc.arg(to_ts)
GROUP BY u.service_name ORDER BY cost DESC LIMIT 100;

-- name: TotalCost :one
SELECT COALESCE(sum(CASE WHEN sqlc.arg(currency)::text = '' THEN billed_cost
                       ELSE finops.convert_cost(billed_cost, currency::text, sqlc.arg(currency)::text, recorded_at::date) END), 0)::float8 AS cost
FROM usage.usage_records
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND recorded_at >= sqlc.arg(from_ts) AND recorded_at < sqlc.arg(to_ts);

-- name: CostDays :one
SELECT count(DISTINCT date_trunc('day', recorded_at))::int AS days
FROM usage.usage_records
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND recorded_at >= sqlc.arg(from_ts) AND recorded_at < sqlc.arg(to_ts);

-- name: ListBudgets :many
SELECT id, tenant_id, project_id, amount, currency::text AS currency, period, created_at
FROM finops.budgets WHERE tenant_id = sqlc.arg(tenant_id) ORDER BY created_at DESC LIMIT 200;

-- name: CreateBudget :one
INSERT INTO finops.budgets (tenant_id, project_id, amount, currency, period)
VALUES (sqlc.arg(tenant_id), sqlc.narg(project_id), sqlc.arg(amount), sqlc.arg(currency), sqlc.arg(period))
RETURNING id, tenant_id, project_id, amount, currency::text AS currency, period, created_at;

-- name: DeleteBudget :execrows
DELETE FROM finops.budgets WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: Anomalies :many
-- Days whose cost exceeds the trailing mean by > 3 standard deviations (needs >= 7 prior days). One currency only:
-- the dominant one of the window, other currencies converted at the day's reference rate.
WITH d AS (
  SELECT date_trunc('day', u.recorded_at)::date AS day, COALESCE(sum(finops.convert_cost(u.billed_cost, u.currency::text, finops.reporting_currency(sqlc.arg(tenant_id), (SELECT d.currency FROM usage.usage_records d
    WHERE d.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR d.project_id = sqlc.narg(project_id))
      AND d.recorded_at >= sqlc.arg(from_ts) AND d.recorded_at < sqlc.arg(to_ts)
    GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1)), u.recorded_at::date)), 0)::float8 AS cost
  FROM usage.usage_records u
  WHERE u.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR u.project_id = sqlc.narg(project_id))
    AND u.recorded_at >= sqlc.arg(from_ts) AND u.recorded_at < sqlc.arg(to_ts)
  GROUP BY 1
), s AS (
  SELECT day, cost,
         avg(cost) OVER w AS mean, stddev_samp(cost) OVER w AS sd, count(*) OVER w AS n
  FROM d WINDOW w AS (ORDER BY day ROWS BETWEEN 14 PRECEDING AND 1 PRECEDING)
)
SELECT day::timestamptz AS day, cost, mean::float8 AS mean, sd::float8 AS stddev
FROM s WHERE n >= 7 AND sd > 0 AND cost > mean + 3 * sd ORDER BY day DESC LIMIT 20;

-- name: UnconvertedCost :many
-- Currencies with amounts in the window that have no reference rate for the reporting currency: they are left out of the
-- totals, and the overview says which and how many, so a total is never silently short.
SELECT u.currency::text AS currency, count(*)::int AS rows
FROM usage.usage_records u
WHERE u.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR u.project_id = sqlc.narg(project_id))
  AND u.recorded_at >= sqlc.arg(from_ts) AND u.recorded_at < sqlc.arg(to_ts)
  AND finops.convert_cost(1, u.currency::text, finops.reporting_currency(sqlc.arg(tenant_id), (SELECT d.currency FROM usage.usage_records d
    WHERE d.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR d.project_id = sqlc.narg(project_id))
      AND d.recorded_at >= sqlc.arg(from_ts) AND d.recorded_at < sqlc.arg(to_ts)
    GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1)), u.recorded_at::date) IS NULL
GROUP BY 1 ORDER BY 2 DESC;
