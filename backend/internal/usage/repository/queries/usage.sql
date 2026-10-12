-- name: UpsertUsageRecord :batchexec
-- account_ref is the connection's account (the key of the line together with the provider): two accounts of one
-- provider never overwrite each other, and connecting an account again updates its lines instead of doubling them.
INSERT INTO usage.usage_records (tenant_id, project_id, connection_id, account_ref, source, provider, resource_id, resource_type, service_name, service_category,
    region_id, consumed_quantity, consumed_unit, billed_cost, effective_cost, currency, recorded_at, charge_period_end)
VALUES (sqlc.arg(tenant_id), sqlc.arg(project_id), sqlc.narg(connection_id),
    COALESCE((SELECT c.account_ref FROM cloudaccounts.connections c WHERE c.tenant_id = sqlc.arg(tenant_id) AND c.id = sqlc.narg(connection_id)::uuid), ''),
    sqlc.arg(source), sqlc.arg(provider), sqlc.arg(resource_id), sqlc.arg(resource_type),
    sqlc.arg(service_name), sqlc.arg(service_category), sqlc.arg(region_id), sqlc.narg(consumed_quantity), sqlc.narg(consumed_unit),
    sqlc.arg(billed_cost), sqlc.arg(effective_cost), sqlc.arg(currency), sqlc.arg(recorded_at), sqlc.arg(charge_period_end))
ON CONFLICT (tenant_id, project_id, provider, account_ref, service_name, region_id, resource_id, consumed_unit, recorded_at)
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
-- Daily rollup per workload dimension: the input of the carbon engine. Cost is converted to USD at the rate of its
-- day (finops.convert_cost), so a bill in euro still gets a cost-based estimate; an amount in a currency without a
-- rate keeps its own currency and the engine skips it rather than guess.
WITH g AS (
  SELECT provider, service_name, service_category, region_id, consumed_unit, currency::text AS currency,
         date_trunc('day', recorded_at)::timestamptz AS day,
         COALESCE(sum(consumed_quantity), 0)::float8 AS quantity,
         sum(billed_cost) AS cost,
         (count(consumed_quantity) > 0)::bool AS has_quantity
  FROM usage.usage_records
  WHERE tenant_id = sqlc.arg(tenant_id) AND project_id = sqlc.arg(project_id)
    AND recorded_at >= sqlc.arg(from_ts) AND recorded_at < sqlc.arg(to_ts)
    AND provider <> 'saas' -- a vendor outside the clouds has cost only: no region, no energy model, so no carbon is derived from it (ADR-0062)
  GROUP BY provider, service_name, service_category, region_id, consumed_unit, currency, day
)
SELECT g.provider, g.service_name, g.service_category, g.region_id, g.consumed_unit,
       (CASE WHEN fx.usd IS NULL THEN g.currency ELSE 'USD' END)::text AS currency,
       g.day, g.quantity, COALESCE(fx.usd, g.cost)::float8 AS billed_cost, g.has_quantity
FROM g
LEFT JOIN LATERAL (SELECT finops.convert_cost(g.cost, g.currency, 'USD', g.day::date) AS usd) fx ON true
ORDER BY g.day, g.service_name;
