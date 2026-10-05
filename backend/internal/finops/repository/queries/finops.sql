-- name: CostByService :many
-- Dominant-currency handling happens in Go; here: cost per service and currency.
SELECT service_name, currency::text AS currency, sum(billed_cost)::float8 AS cost
FROM usage.usage_records
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND recorded_at >= sqlc.arg(from_ts) AND recorded_at < sqlc.arg(to_ts)
GROUP BY service_name, currency ORDER BY cost DESC LIMIT 100;

-- name: TotalCost :one
SELECT COALESCE(sum(billed_cost), 0)::float8 AS cost
FROM usage.usage_records
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND (sqlc.arg(currency)::text = '' OR currency = sqlc.arg(currency)::text)
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
-- Days whose cost exceeds the trailing mean by > 3 standard deviations (needs >= 7 prior days).
WITH d AS (
  SELECT date_trunc('day', recorded_at)::date AS day, sum(billed_cost)::float8 AS cost
  FROM usage.usage_records
  WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
    AND recorded_at >= sqlc.arg(from_ts) AND recorded_at < sqlc.arg(to_ts)
  GROUP BY 1
), s AS (
  SELECT day, cost,
         avg(cost) OVER w AS mean, stddev_samp(cost) OVER w AS sd, count(*) OVER w AS n
  FROM d WINDOW w AS (ORDER BY day ROWS BETWEEN 14 PRECEDING AND 1 PRECEDING)
)
SELECT day::timestamptz AS day, cost, mean::float8 AS mean, sd::float8 AS stddev
FROM s WHERE n >= 7 AND sd > 0 AND cost > mean + 3 * sd ORDER BY day DESC LIMIT 20;
