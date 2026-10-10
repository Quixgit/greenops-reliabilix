-- name: InsertRecommendation :one
INSERT INTO recommendations.recommendations (tenant_id, project_id, type, title, provider, service_name, current_region, recommended_region,
    estimated_carbon_reduction_pct, carbon_reduction_kg_month, estimated_cost_impact, cost_basis, confidence, compliance_check, fingerprint, details)
VALUES (sqlc.arg(tenant_id), sqlc.arg(project_id), sqlc.arg(type), sqlc.arg(title), sqlc.arg(provider), sqlc.arg(service_name), sqlc.arg(current_region),
    sqlc.arg(recommended_region), sqlc.arg(estimated_carbon_reduction_pct), sqlc.arg(carbon_reduction_kg_month), sqlc.narg(estimated_cost_impact),
    sqlc.arg(cost_basis), sqlc.arg(confidence), sqlc.arg(compliance_check), sqlc.arg(fingerprint), sqlc.arg(details))
ON CONFLICT (tenant_id, fingerprint) DO NOTHING
RETURNING id;

-- name: ListRecommendations :many
SELECT * FROM recommendations.recommendations
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.arg(status)::text = '' OR status = sqlc.arg(status)::text)
  AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
ORDER BY carbon_reduction_kg_month DESC, created_at DESC LIMIT sqlc.arg(row_limit);

-- name: GetRecommendation :one
SELECT * FROM recommendations.recommendations WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id) FOR UPDATE;

-- name: SetRecommendationStatus :exec
UPDATE recommendations.recommendations
SET status = sqlc.arg(status), decided_by = sqlc.arg(decided_by), decided_at = now(),
    applied_at = CASE WHEN sqlc.arg(status)::text = 'applied' THEN now() ELSE applied_at END
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: WorkloadsByRegion :many
-- Input of the region_shift generator: last-window carbon per workload (read-only view of the carbon schema).
SELECT provider, service_name, region_id,
       sum(energy_kwh)::float8 AS energy_kwh, sum(carbon_kg_co2e)::float8 AS carbon_kg
FROM carbon.calculations
WHERE tenant_id = sqlc.arg(tenant_id) AND project_id = sqlc.arg(project_id) AND methodology_version = sqlc.arg(methodology_version)
  AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts)
GROUP BY provider, service_name, region_id;

-- name: WorkloadCost :many
SELECT provider, service_name, region_id, sum(billed_cost)::float8 AS cost
FROM usage.usage_records
WHERE tenant_id = sqlc.arg(tenant_id) AND project_id = sqlc.arg(project_id)
  AND recorded_at >= sqlc.arg(from_ts) AND recorded_at < sqlc.arg(to_ts)
GROUP BY provider, service_name, region_id;

-- name: LatestIntensities :many
SELECT DISTINCT ON (region_id) region_id, g_co2e_per_kwh::float8 AS g_per_kwh, ts
FROM carbon.grid_intensity WHERE NOT is_forecast ORDER BY region_id, ts DESC;

-- name: PriceIndex :many
SELECT region_id, idx::float8 AS idx FROM finops.region_price_index WHERE provider = sqlc.arg(provider);

-- name: GetPolicyRegions :one
SELECT allowed_regions FROM projects.policies WHERE tenant_id = sqlc.arg(tenant_id) AND project_id = sqlc.arg(project_id);

-- name: WorkloadDays :one
SELECT count(DISTINCT date_trunc('day', period_start))::int AS days FROM carbon.calculations
WHERE tenant_id = sqlc.arg(tenant_id) AND project_id = sqlc.arg(project_id) AND methodology_version = sqlc.arg(methodology_version)
  AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts);

-- name: AppliedAwaitingVerification :many
SELECT r.* FROM recommendations.recommendations r
WHERE r.tenant_id = sqlc.arg(tenant_id) AND r.project_id = sqlc.arg(project_id) AND r.status = 'applied'
  AND NOT EXISTS (SELECT 1 FROM recommendations.impact_verifications v WHERE v.tenant_id = r.tenant_id AND v.recommendation_id = r.id)
ORDER BY r.applied_at;

-- name: DailyCost :many
SELECT date_trunc('day', recorded_at)::timestamptz AS day, sum(billed_cost)::float8 AS cost, max(currency)::text AS currency
FROM usage.usage_records
WHERE tenant_id = sqlc.arg(tenant_id) AND project_id = sqlc.arg(project_id)
  AND (sqlc.arg(service)::text = '' OR service_name = sqlc.arg(service)::text)
  AND recorded_at >= sqlc.arg(from_ts) AND recorded_at < sqlc.arg(to_ts)
GROUP BY 1 ORDER BY 1;

-- name: DailyCarbon :many
SELECT date_trunc('day', period_start)::timestamptz AS day, sum(carbon_kg_co2e)::float8 AS carbon_kg
FROM carbon.calculations
WHERE tenant_id = sqlc.arg(tenant_id) AND project_id = sqlc.arg(project_id) AND methodology_version = sqlc.arg(methodology_version)
  AND (sqlc.arg(service)::text = '' OR service_name = sqlc.arg(service)::text)
  AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts)
GROUP BY 1 ORDER BY 1;

-- name: InsertVerification :exec
INSERT INTO recommendations.impact_verifications (tenant_id, recommendation_id, project_id, outcome, method, baseline_from, baseline_to,
    after_from, after_to, baseline_days, after_days, measured_carbon_kg_month, measured_cost_month, expected_carbon_kg_month,
    expected_cost_month, realization_pct, currency, note, verified_at)
SELECT r.tenant_id, r.id, r.project_id, sqlc.arg(outcome), sqlc.arg(method), sqlc.arg(baseline_from), sqlc.arg(baseline_to),
    sqlc.arg(after_from), sqlc.arg(after_to), sqlc.arg(baseline_days), sqlc.arg(after_days), sqlc.narg(measured_carbon_kg_month),
    sqlc.narg(measured_cost_month), sqlc.arg(expected_carbon_kg_month), sqlc.narg(expected_cost_month), sqlc.narg(realization_pct),
    sqlc.narg(currency), sqlc.arg(note), sqlc.arg(verified_at)
FROM recommendations.recommendations r WHERE r.tenant_id = sqlc.arg(tenant_id) AND r.id = sqlc.arg(id)
ON CONFLICT (tenant_id, recommendation_id) DO NOTHING;

-- name: MarkVerified :execrows
UPDATE recommendations.recommendations SET status = 'verified'
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id) AND status = 'applied';

-- name: ListVerifications :many
SELECT * FROM recommendations.impact_verifications WHERE tenant_id = sqlc.arg(tenant_id) AND recommendation_id = ANY(sqlc.arg(ids)::uuid[]);
