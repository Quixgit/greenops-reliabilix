-- name: UpsertCalculation :batchexec
INSERT INTO carbon.calculations (tenant_id, project_id, provider, region_id, service_name, service_category, method, period_start, period_end,
    energy_kwh, intensity_g_per_kwh, embodied_g, carbon_kg_co2e, functional_units, sci_score, methodology_version)
VALUES (sqlc.arg(tenant_id), sqlc.arg(project_id), sqlc.arg(provider), sqlc.arg(region_id), sqlc.arg(service_name), sqlc.arg(service_category), sqlc.arg(method),
    sqlc.arg(period_start), sqlc.arg(period_end), sqlc.arg(energy_kwh), sqlc.arg(intensity_g_per_kwh), sqlc.arg(embodied_g), sqlc.arg(carbon_kg_co2e),
    sqlc.narg(functional_units), sqlc.narg(sci_score), sqlc.arg(methodology_version))
ON CONFLICT (tenant_id, project_id, provider, service_name, region_id, method, methodology_version, period_start)
DO UPDATE SET energy_kwh = EXCLUDED.energy_kwh, intensity_g_per_kwh = EXCLUDED.intensity_g_per_kwh, embodied_g = EXCLUDED.embodied_g,
              carbon_kg_co2e = EXCLUDED.carbon_kg_co2e, functional_units = EXCLUDED.functional_units, sci_score = EXCLUDED.sci_score,
              period_end = EXCLUDED.period_end, calculated_at = now();

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
INSERT INTO carbon.grid_intensity (provider, region_id, ts, g_co2e_per_kwh, is_forecast)
VALUES (sqlc.arg(provider), sqlc.arg(region_id), sqlc.arg(ts), sqlc.arg(g_co2e_per_kwh), sqlc.arg(is_forecast))
ON CONFLICT DO NOTHING;

-- name: IntensityAt :one
-- Latest stored (non-forecast) reading at or before ts for a region.
SELECT g_co2e_per_kwh FROM carbon.grid_intensity
WHERE region_id = sqlc.arg(region_id) AND NOT is_forecast AND ts <= sqlc.arg(ts)
ORDER BY ts DESC LIMIT 1;

-- name: LatestIntensity :one
SELECT g_co2e_per_kwh FROM carbon.grid_intensity
WHERE region_id = sqlc.arg(region_id) AND NOT is_forecast ORDER BY ts DESC LIMIT 1;
