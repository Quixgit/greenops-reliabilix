-- name: DashboardTrend :many
WITH cost_d AS (
  SELECT date_trunc('day', u.recorded_at) AS d, sum(u.billed_cost) AS cost FROM usage.usage_records u
  WHERE u.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR u.project_id = sqlc.narg(project_id))
    AND u.recorded_at >= sqlc.arg(from_ts) AND u.recorded_at < sqlc.arg(to_ts) GROUP BY 1
), carbon_d AS (
  SELECT date_trunc('day', x.period_start) AS d, sum(x.carbon_kg_co2e) AS co2e FROM carbon.calculations x
  WHERE x.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR x.project_id = sqlc.narg(project_id))
    AND x.methodology_version = sqlc.arg(methodology_version)
    AND x.period_start >= sqlc.arg(from_ts) AND x.period_start < sqlc.arg(to_ts) GROUP BY 1
)
SELECT COALESCE(cost_d.d, carbon_d.d)::timestamptz AS day, COALESCE(cost_d.cost, 0)::float8 AS cost, COALESCE(carbon_d.co2e, 0)::float8 AS carbon_kg_co2e
FROM cost_d FULL JOIN carbon_d ON cost_d.d = carbon_d.d ORDER BY 1;

-- name: DashboardProviders :many
SELECT provider, sum(carbon_kg_co2e)::float8 AS carbon_kg_co2e FROM carbon.calculations
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND methodology_version = sqlc.arg(methodology_version)
  AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts)
GROUP BY provider HAVING sum(carbon_kg_co2e) > 0 ORDER BY 2 DESC;

-- name: DashboardServiceCost :many
SELECT service_name, service_category, sum(billed_cost)::float8 AS cost FROM usage.usage_records
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND recorded_at >= sqlc.arg(from_ts) AND recorded_at < sqlc.arg(to_ts)
  AND (sqlc.arg(category)::text = '' OR service_category = sqlc.arg(category)::text)
GROUP BY 1, 2;

-- name: DashboardServiceCarbon :many
SELECT service_name, service_category, date_trunc('day', period_start)::timestamptz AS day, sum(carbon_kg_co2e)::float8 AS carbon_kg_co2e
FROM carbon.calculations
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND methodology_version = sqlc.arg(methodology_version)
  AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts)
  AND (sqlc.arg(category)::text = '' OR service_category = sqlc.arg(category)::text)
GROUP BY 1, 2, 3 ORDER BY 3;

-- name: DashboardRegions :many
-- Reference data (no tenant): latest reading per region and the one >= 24h older, for the change column.
WITH latest AS (
  SELECT DISTINCT ON (region_id) region_id, g_co2e_per_kwh AS g, ts FROM carbon.grid_intensity
  WHERE provider = 'electricitymaps' AND NOT is_forecast ORDER BY region_id, ts DESC
), prev AS (
  SELECT DISTINCT ON (gi.region_id) gi.region_id, gi.g_co2e_per_kwh AS g FROM carbon.grid_intensity gi
  JOIN latest l ON l.region_id = gi.region_id AND gi.ts <= l.ts - interval '24 hours'
  WHERE gi.provider = 'electricitymaps' AND NOT gi.is_forecast ORDER BY gi.region_id, gi.ts DESC
)
SELECT l.region_id, l.g::float8 AS g_per_kwh, l.ts AS at, p.g AS prev_g_per_kwh
FROM latest l LEFT JOIN prev p USING (region_id) ORDER BY l.g;

-- name: DashboardActivity :many
SELECT id, action, target, created_at FROM audit.audit_logs
WHERE tenant_id = sqlc.arg(tenant_id) AND action = ANY(sqlc.arg(actions)::text[])
ORDER BY created_at DESC LIMIT sqlc.arg(row_limit);
