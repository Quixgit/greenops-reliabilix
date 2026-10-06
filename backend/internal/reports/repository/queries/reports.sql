-- name: InsertReport :one
INSERT INTO reports.reports (tenant_id, project_id, kind, format, period_start, period_end, requested_by)
VALUES (sqlc.arg(tenant_id), sqlc.narg(project_id), sqlc.arg(kind), sqlc.arg(format), sqlc.arg(period_start), sqlc.arg(period_end), sqlc.arg(requested_by))
RETURNING id, tenant_id, project_id, kind, format, period_start, period_end, status, object_key, error, requested_by, created_at, completed_at;

-- name: ListReports :many
SELECT id, tenant_id, project_id, kind, format, period_start, period_end, status, object_key, error, requested_by, created_at, completed_at
FROM reports.reports WHERE tenant_id = sqlc.arg(tenant_id) ORDER BY created_at DESC LIMIT 100;

-- name: GetReport :one
SELECT id, tenant_id, project_id, kind, format, period_start, period_end, status, object_key, error, requested_by, created_at, completed_at
FROM reports.reports WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: SetReportReady :exec
UPDATE reports.reports SET status = 'ready', object_key = sqlc.arg(object_key), error = NULL, completed_at = now()
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: SetReportFailed :exec
UPDATE reports.reports SET status = 'failed', error = sqlc.arg(error), completed_at = now()
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: ReportCarbonDaily :many
SELECT date_trunc('day', period_start)::timestamptz AS day, provider, service_name, region_id, method,
       sum(energy_kwh)::float8 AS energy_kwh, sum(carbon_kg_co2e)::float8 AS carbon_kg_co2e, methodology_version
FROM carbon.calculations
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts)
GROUP BY day, provider, service_name, region_id, method, methodology_version ORDER BY day, service_name;

-- name: ReportCostDaily :many
SELECT date_trunc('day', recorded_at)::timestamptz AS day, provider, service_name, service_category, region_id,
       sum(billed_cost)::float8 AS billed_cost, sum(effective_cost)::float8 AS effective_cost, currency::text AS currency
FROM usage.usage_records
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND recorded_at >= sqlc.arg(from_ts) AND recorded_at < sqlc.arg(to_ts)
GROUP BY day, provider, service_name, service_category, region_id, currency ORDER BY day, service_name;

-- name: ReportSCIDaily :many
SELECT p.name AS project, p.functional_unit, date_trunc('day', c.period_start)::timestamptz AS day,
       sum(c.carbon_kg_co2e)::float8 AS carbon_kg_co2e, max(c.functional_units)::float8 AS functional_units,
       c.methodology_version
FROM carbon.calculations c JOIN projects.projects p ON p.id = c.project_id AND p.tenant_id = c.tenant_id
WHERE c.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR c.project_id = sqlc.narg(project_id))
  AND c.period_start >= sqlc.arg(from_ts) AND c.period_start < sqlc.arg(to_ts)
GROUP BY p.name, p.functional_unit, day, c.methodology_version ORDER BY p.name, day;

-- name: ReportSustainabilityByMethod :many
SELECT method, sum(energy_kwh)::float8 AS energy_kwh, sum(carbon_kg_co2e)::float8 AS carbon_kg
FROM carbon.calculations
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND methodology_version = sqlc.arg(methodology_version)
  AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts)
GROUP BY method ORDER BY method;

-- name: ReportSustainabilityTopServices :many
SELECT provider, service_name, sum(carbon_kg_co2e)::float8 AS carbon_kg
FROM carbon.calculations
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND methodology_version = sqlc.arg(methodology_version)
  AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts)
GROUP BY provider, service_name ORDER BY carbon_kg DESC, service_name LIMIT 10;

-- name: ReportSustainabilityTopRegions :many
SELECT region_id, sum(carbon_kg_co2e)::float8 AS carbon_kg
FROM carbon.calculations
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND methodology_version = sqlc.arg(methodology_version)
  AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts)
GROUP BY region_id ORDER BY carbon_kg DESC, region_id LIMIT 10;

-- name: ReportSustainabilityRecommendations :many
-- Recommendations created up to the end of the window, by status: what is still open and what was done.
SELECT status, count(*)::int AS n, COALESCE(sum(carbon_reduction_kg_month), 0)::float8 AS kg_month
FROM recommendations.recommendations
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND created_at < sqlc.arg(to_ts)
GROUP BY status ORDER BY status;
