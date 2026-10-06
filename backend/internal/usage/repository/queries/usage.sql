-- name: UpsertUsageRecord :batchexec
INSERT INTO usage.usage_records (tenant_id, project_id, connection_id, source, provider, resource_id, resource_type, service_name, service_category,
    region_id, consumed_quantity, consumed_unit, billed_cost, effective_cost, currency, recorded_at, charge_period_end)
VALUES (sqlc.arg(tenant_id), sqlc.arg(project_id), sqlc.narg(connection_id), sqlc.arg(source), sqlc.arg(provider), sqlc.arg(resource_id), sqlc.arg(resource_type),
    sqlc.arg(service_name), sqlc.arg(service_category), sqlc.arg(region_id), sqlc.narg(consumed_quantity), sqlc.narg(consumed_unit),
    sqlc.arg(billed_cost), sqlc.arg(effective_cost), sqlc.arg(currency), sqlc.arg(recorded_at), sqlc.arg(charge_period_end))
ON CONFLICT (tenant_id, project_id, provider, service_name, region_id, resource_id, consumed_unit, recorded_at)
DO UPDATE SET billed_cost = EXCLUDED.billed_cost, effective_cost = EXCLUDED.effective_cost, consumed_quantity = EXCLUDED.consumed_quantity,
              charge_period_end = EXCLUDED.charge_period_end, currency = EXCLUDED.currency, connection_id = EXCLUDED.connection_id;

-- name: ListUsage :many
SELECT id, project_id, provider, source, resource_id, resource_type, service_name, service_category, region_id,
       consumed_quantity, consumed_unit, billed_cost, effective_cost, currency, recorded_at, charge_period_end
FROM usage.usage_records
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND recorded_at >= sqlc.arg(from_ts) AND recorded_at < sqlc.arg(to_ts)
ORDER BY recorded_at DESC, service_name LIMIT sqlc.arg(row_limit);

-- name: AggregateDaily :many
-- Daily rollup per workload dimension: the input of the carbon engine.
SELECT provider, service_name, service_category, region_id, consumed_unit, currency::text AS currency,
       date_trunc('day', recorded_at)::timestamptz AS day,
       COALESCE(sum(consumed_quantity), 0)::float8 AS quantity,
       sum(billed_cost)::float8 AS billed_cost,
       (count(consumed_quantity) > 0)::bool AS has_quantity
FROM usage.usage_records
WHERE tenant_id = sqlc.arg(tenant_id) AND project_id = sqlc.arg(project_id)
  AND recorded_at >= sqlc.arg(from_ts) AND recorded_at < sqlc.arg(to_ts)
GROUP BY provider, service_name, service_category, region_id, consumed_unit, currency, day
ORDER BY day, service_name;
