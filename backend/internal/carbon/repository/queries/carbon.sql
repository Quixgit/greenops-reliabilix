-- name: UpsertCalculation :batchexec
INSERT INTO carbon.calculations (tenant_id, project_id, provider, region_id, service_name, service_category, method, period_start, period_end,
    energy_kwh, intensity_g_per_kwh, embodied_g, carbon_kg_co2e, functional_units, sci_score, methodology_version, intensity_source, intensity_basis)
VALUES (sqlc.arg(tenant_id), sqlc.arg(project_id), sqlc.arg(provider), sqlc.arg(region_id), sqlc.arg(service_name), sqlc.arg(service_category), sqlc.arg(method),
    sqlc.arg(period_start), sqlc.arg(period_end), sqlc.arg(energy_kwh), sqlc.arg(intensity_g_per_kwh), sqlc.arg(embodied_g), sqlc.arg(carbon_kg_co2e),
    sqlc.narg(functional_units), sqlc.narg(sci_score), sqlc.arg(methodology_version), sqlc.narg(intensity_source), sqlc.narg(intensity_basis))
ON CONFLICT (tenant_id, project_id, provider, service_name, region_id, method, methodology_version, period_start)
DO UPDATE SET energy_kwh = EXCLUDED.energy_kwh, intensity_g_per_kwh = EXCLUDED.intensity_g_per_kwh, embodied_g = EXCLUDED.embodied_g,
              carbon_kg_co2e = EXCLUDED.carbon_kg_co2e, functional_units = EXCLUDED.functional_units, sci_score = EXCLUDED.sci_score,
              period_end = EXCLUDED.period_end, intensity_source = EXCLUDED.intensity_source, intensity_basis = EXCLUDED.intensity_basis, calculated_at = now();

-- name: DeleteCalculationsInWindow :exec
-- Rows of one methodology version in [from, to): the window is recomputed as a whole, so rows that the
-- engine no longer produces (e.g. a cost-based row superseded by measured usage) must not linger.
DELETE FROM carbon.calculations
WHERE tenant_id = sqlc.arg(tenant_id) AND project_id = sqlc.arg(project_id)
  AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts)
  AND methodology_version = sqlc.arg(methodology_version);

-- name: CarbonTotals :one
-- SCI over a window = total carbon / total functional units, counting R once per (project, day) and
-- only on days where R was reported (never extrapolated). Per-row sci_score is that row's share of its day's SCI.
WITH daily AS (
  SELECT project_id, date_trunc('day', period_start) AS day,
         sum(energy_kwh) AS energy, sum(carbon_kg_co2e) AS kg, max(functional_units) AS r, count(*) AS n
  FROM carbon.calculations
  WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
    AND methodology_version = sqlc.arg(methodology_version)
    AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts)
  GROUP BY project_id, day
)
SELECT COALESCE(sum(energy), 0)::float8 AS energy_kwh,
       COALESCE(sum(kg), 0)::float8 AS carbon_kg_co2e,
       COALESCE(sum(n), 0)::bigint AS row_count,
       COALESCE(sum(kg * 1000) FILTER (WHERE r IS NOT NULL), 0)::float8 AS sci_grams,
       COALESCE(sum(r), 0)::float8 AS sci_units,
       count(DISTINCT day)::int AS days
FROM daily;

-- name: CarbonTrend :many
SELECT date_trunc('day', period_start)::timestamptz AS day, sum(carbon_kg_co2e)::float8 AS carbon_kg_co2e
FROM carbon.calculations
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND methodology_version = sqlc.arg(methodology_version)
  AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts)
GROUP BY day ORDER BY day;

-- name: SaveGridIntensity :exec
INSERT INTO carbon.grid_intensity (provider, region_id, ts, g_co2e_per_kwh, is_forecast, measure, resolution)
VALUES (sqlc.arg(provider), sqlc.arg(region_id), sqlc.arg(ts), sqlc.arg(g_co2e_per_kwh), sqlc.arg(is_forecast), sqlc.arg(measure), sqlc.arg(resolution))
ON CONFLICT (provider, region_id, ts, is_forecast, measure) DO UPDATE SET g_co2e_per_kwh = EXCLUDED.g_co2e_per_kwh, resolution = EXCLUDED.resolution
WHERE carbon.grid_intensity.resolution = 'monthly'; -- a published monthly figure may be revised; an hourly reading never is

-- name: IntensityDayStats :one
-- Mean and count of one source's stored (non-forecast) AVERAGE-intensity readings inside [from, to). Sources are never
-- averaged together: the caller asks one source at a time, in policy order.
SELECT COALESCE(avg(g_co2e_per_kwh), 0)::float8 AS g, count(*)::int AS n FROM carbon.grid_intensity
WHERE region_id = sqlc.arg(region_id) AND provider = sqlc.arg(provider) AND measure = 'average' AND NOT is_forecast AND resolution = 'hourly'
  AND ts >= sqlc.arg(from_ts) AND ts < sqlc.arg(to_ts);

-- name: IntensityLatest :one
-- Latest stored reading of one source at or before `before` (NULL = the latest overall).
SELECT g_co2e_per_kwh::float8 AS g, ts FROM carbon.grid_intensity
WHERE region_id = sqlc.arg(region_id) AND provider = sqlc.arg(provider) AND measure = 'average' AND NOT is_forecast AND resolution = 'hourly'
  AND (sqlc.narg(before)::timestamptz IS NULL OR ts <= sqlc.narg(before))
ORDER BY ts DESC LIMIT 1;

-- name: IntensityHistory :many
-- Daily mean, extremes and reading count per source: the raw material of the history chart.
SELECT date_trunc('day', ts)::timestamptz AS day, provider, avg(g_co2e_per_kwh)::float8 AS mean,
       min(g_co2e_per_kwh)::float8 AS lo, max(g_co2e_per_kwh)::float8 AS hi, count(*)::int AS n
FROM carbon.grid_intensity
WHERE region_id = sqlc.arg(region_id) AND measure = 'average' AND NOT is_forecast AND resolution = 'hourly' AND ts >= sqlc.arg(from_ts) AND ts < sqlc.arg(to_ts)
GROUP BY 1, 2 ORDER BY 1, 2;

-- name: TenantRegions :many
-- Regions that have calculations for the organization in the window, largest emitter first.
SELECT region_id, sum(carbon_kg_co2e)::float8 AS carbon_kg_co2e FROM carbon.calculations
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND methodology_version = sqlc.arg(methodology_version) AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts)
GROUP BY region_id ORDER BY 2 DESC;

-- name: IntensityBasisBreakdown :many
-- Where the intensities of a window came from: carbon by source and basis. NULL source/basis = calculated before the
-- origin was recorded.
SELECT COALESCE(intensity_source, '')::text AS source, COALESCE(intensity_basis, '')::text AS basis, sum(carbon_kg_co2e)::float8 AS carbon_kg_co2e
FROM carbon.calculations
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND methodology_version = sqlc.arg(methodology_version) AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts)
GROUP BY 1, 2 ORDER BY 3 DESC;

-- name: IntensityMonthly :one
-- One source's published monthly figure for the month that contains `day` (ts is the first day of the month).
SELECT g_co2e_per_kwh::float8 AS g FROM carbon.grid_intensity
WHERE region_id = sqlc.arg(region_id) AND provider = sqlc.arg(provider) AND measure = 'average' AND NOT is_forecast AND resolution = 'monthly'
  AND ts = date_trunc('month', sqlc.arg(day)::timestamptz);

-- name: IntensityMonthlyHistory :many
SELECT ts AS month, provider, g_co2e_per_kwh::float8 AS g FROM carbon.grid_intensity
WHERE region_id = sqlc.arg(region_id) AND measure = 'average' AND NOT is_forecast AND resolution = 'monthly'
  AND ts >= date_trunc('month', sqlc.arg(from_ts)::timestamptz) AND ts < sqlc.arg(to_ts)
ORDER BY ts, provider;
