-- name: DashboardTrend :many
WITH cost_d AS (
  SELECT date_trunc('day', u.recorded_at) AS d, COALESCE(sum(finops.convert_cost(u.billed_cost, u.currency::text, finops.reporting_currency(sqlc.arg(tenant_id), (SELECT d.currency FROM usage.usage_records d
    WHERE d.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR d.project_id = sqlc.narg(project_id))
      AND d.recorded_at >= sqlc.arg(from_ts) AND d.recorded_at < sqlc.arg(to_ts)
    GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1)), u.recorded_at::date)), 0) AS cost FROM usage.usage_records u
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
SELECT u.service_name, u.service_category, COALESCE(sum(finops.convert_cost(u.billed_cost, u.currency::text, finops.reporting_currency(sqlc.arg(tenant_id), (SELECT d.currency FROM usage.usage_records d
    WHERE d.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR d.project_id = sqlc.narg(project_id))
      AND d.recorded_at >= sqlc.arg(from_ts) AND d.recorded_at < sqlc.arg(to_ts)
    GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1)), u.recorded_at::date)), 0)::float8 AS cost FROM usage.usage_records u
WHERE u.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR u.project_id = sqlc.narg(project_id))
  AND u.recorded_at >= sqlc.arg(from_ts) AND u.recorded_at < sqlc.arg(to_ts)
  AND (sqlc.arg(category)::text = '' OR u.service_category = sqlc.arg(category)::text)
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
-- Reference data (no tenant): per region, the figure of the most trusted FRESH source (platform setting carbon.provider_priority;
-- a source whose newest reading is older than its accepted age ranks behind every fresh one): the median of its readings in the
-- six hours up to its newest one, so one spike hour is not shown as the region's state; and that same source's figure from
-- >= 24h earlier for the change column.
WITH newest AS (
  SELECT DISTINCT ON (region_id) region_id, provider, g_co2e_per_kwh AS g, ts FROM carbon.grid_intensity
  WHERE NOT is_forecast AND measure = 'average' AND provider = ANY(carbon.provider_priority())
  ORDER BY region_id, (ts < now() - carbon.source_max_age(provider)), array_position(carbon.provider_priority(), provider), ts DESC
), latest AS (
  SELECT n.region_id, n.provider, n.ts,
         COALESCE((SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY gi.g_co2e_per_kwh) FROM carbon.grid_intensity gi
                    WHERE gi.region_id = n.region_id AND gi.provider = n.provider AND NOT gi.is_forecast AND gi.measure = 'average'
                      AND gi.ts > n.ts - interval '6 hours' AND gi.ts <= n.ts), n.g)::float8 AS g
  FROM newest n
), prev AS (
  SELECT DISTINCT ON (gi.region_id) gi.region_id,
         (SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY x.g_co2e_per_kwh) FROM carbon.grid_intensity x
           WHERE x.region_id = gi.region_id AND x.provider = gi.provider AND NOT x.is_forecast AND x.measure = 'average'
             AND x.ts > gi.ts - interval '6 hours' AND x.ts <= gi.ts)::float8 AS g
  FROM carbon.grid_intensity gi
  JOIN latest l ON l.region_id = gi.region_id AND gi.provider = l.provider AND gi.ts <= l.ts - interval '24 hours'
  WHERE NOT gi.is_forecast AND gi.measure = 'average' ORDER BY gi.region_id, gi.ts DESC
)
SELECT l.region_id, l.g::float8 AS g_per_kwh, l.ts AS at, p.g AS prev_g_per_kwh
FROM latest l LEFT JOIN prev p USING (region_id) ORDER BY l.g;

-- name: DashboardActivity :many
SELECT id, action, target, created_at, metadata FROM audit.audit_logs
WHERE tenant_id = sqlc.arg(tenant_id) AND action = ANY(sqlc.arg(actions)::text[])
ORDER BY created_at DESC LIMIT sqlc.arg(row_limit);

-- name: DashboardRegionCost :many
SELECT u.provider, u.region_id, COALESCE(sum(finops.convert_cost(u.billed_cost, u.currency::text, finops.reporting_currency(sqlc.arg(tenant_id), (SELECT d.currency FROM usage.usage_records d
    WHERE d.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR d.project_id = sqlc.narg(project_id))
      AND d.recorded_at >= sqlc.arg(from_ts) AND d.recorded_at < sqlc.arg(to_ts)
    GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1)), u.recorded_at::date)), 0)::float8 AS cost FROM usage.usage_records u
WHERE u.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR u.project_id = sqlc.narg(project_id))
  AND u.recorded_at >= sqlc.arg(from_ts) AND u.recorded_at < sqlc.arg(to_ts)
GROUP BY 1, 2;

-- name: DashboardRegionCarbon :many
SELECT provider, region_id, sum(energy_kwh)::float8 AS energy_kwh, sum(carbon_kg_co2e)::float8 AS carbon_kg_co2e FROM carbon.calculations
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id))
  AND methodology_version = sqlc.arg(methodology_version)
  AND period_start >= sqlc.arg(from_ts) AND period_start < sqlc.arg(to_ts)
GROUP BY 1, 2;

-- name: DashboardPotential :one
-- What the open and approved recommendations could save per month, as estimated (not yet verified), and how many of
-- them have no cost estimate (their carbon effect is still counted; the cost is never guessed).
SELECT count(*) FILTER (WHERE r.status = 'open')::int AS open_count,
       count(*) FILTER (WHERE status = 'approved')::int AS approved_count,
       COALESCE(sum(carbon_reduction_kg_month) FILTER (WHERE status IN ('open','approved')), 0)::float8 AS potential_carbon_kg_month,
       COALESCE(sum(-estimated_cost_impact) FILTER (WHERE status IN ('open','approved') AND estimated_cost_impact IS NOT NULL), 0)::float8 AS potential_saving_month,
       count(*) FILTER (WHERE status IN ('open','approved') AND estimated_cost_impact IS NULL)::int AS cost_not_estimated,
       count(*) FILTER (WHERE status = 'open' AND r.type = ANY(COALESCE(pol.auto_apply_types, '{}')))::int AS auto_count
FROM recommendations.recommendations r
LEFT JOIN projects.policies pol ON pol.project_id = r.project_id
WHERE r.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR r.project_id = sqlc.narg(project_id));

-- name: DashboardVerified :one
-- What applied recommendations were MEASURED to save (per 30 days), from the impact verification.
SELECT count(*) FILTER (WHERE outcome IN ('realized','partial'))::int AS verified_count,
       count(*) FILTER (WHERE outcome = 'not_realized')::int AS not_realized_count,
       COALESCE(sum(measured_carbon_kg_month) FILTER (WHERE outcome IN ('realized','partial')), 0)::float8 AS verified_carbon_kg_month,
       COALESCE(sum(measured_cost_month) FILTER (WHERE outcome IN ('realized','partial')), 0)::float8 AS verified_saving_month
FROM recommendations.impact_verifications
WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id));

-- name: DashboardEfficiency :one
-- Carbon and cost per unit of work, over the (project, day) pairs where the customer reported functional units: never
-- extrapolated to days without them. `units` lists the distinct unit names, so a mixed answer can be refused.
WITH carbon_d AS (
  SELECT x.project_id, date_trunc('day', x.period_start) AS day, sum(x.carbon_kg_co2e) AS kg, max(x.functional_units) AS r
  FROM carbon.calculations x
  WHERE x.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR x.project_id = sqlc.narg(project_id))
    AND x.methodology_version = sqlc.arg(methodology_version)
    AND x.period_start >= sqlc.arg(from_ts) AND x.period_start < sqlc.arg(to_ts)
  GROUP BY 1, 2 HAVING max(x.functional_units) IS NOT NULL
), cost_d AS (
  SELECT u.project_id, date_trunc('day', u.recorded_at) AS day, COALESCE(sum(finops.convert_cost(u.billed_cost, u.currency::text, finops.reporting_currency(sqlc.arg(tenant_id), (SELECT d.currency FROM usage.usage_records d
      WHERE d.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR d.project_id = sqlc.narg(project_id))
        AND d.recorded_at >= sqlc.arg(from_ts) AND d.recorded_at < sqlc.arg(to_ts)
      GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1)), u.recorded_at::date)), 0) AS cost FROM usage.usage_records u
  WHERE u.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR u.project_id = sqlc.narg(project_id))
    AND u.recorded_at >= sqlc.arg(from_ts) AND u.recorded_at < sqlc.arg(to_ts)
  GROUP BY 1, 2
)
SELECT COALESCE(sum(c.kg) * 1000, 0)::float8 AS carbon_g,
       COALESCE(sum(c.r), 0)::float8 AS units,
       COALESCE(sum(k.cost), 0)::float8 AS cost,
       COALESCE((SELECT array_agg(DISTINCT p.functional_unit ORDER BY p.functional_unit)
                 FROM projects.projects p WHERE p.id IN (SELECT project_id FROM carbon_d)), '{}')::text[] AS unit_names,
       count(*)::int AS days
FROM carbon_d c LEFT JOIN cost_d k ON k.project_id = c.project_id AND k.day = c.day;

-- name: DashboardRegionResidency :many
-- For each (cloud, region) the account uses in the period: the data-residency allow-list of every project using it.
-- An empty list means residency is unknown for that project, so no move out of the region is proposed (the same rule
-- the recommendation engine follows).
SELECT u.provider, u.region_id, u.project_id::text AS project_id, COALESCE(p.allowed_regions, '{}')::text[] AS allowed_regions
FROM (SELECT DISTINCT x.provider, x.region_id, x.project_id FROM usage.usage_records x
      WHERE x.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR x.project_id = sqlc.narg(project_id))
        AND x.recorded_at >= sqlc.arg(from_ts) AND x.recorded_at < sqlc.arg(to_ts)) u
LEFT JOIN projects.policies p ON p.project_id = u.project_id;

-- name: DashboardRegionEC2Cost :many
-- Cost per region and price-index family: virtual machine compute (EC2, Azure Virtual Machines, Compute Engine) and Azure's managed
-- PostgreSQL and Redis, the only costs the region price indexes describe,
-- in the dominant currency.
SELECT u.region_id, CASE
       WHEN u.provider = 'aws' AND lower(u.service_name) LIKE '%elastic compute cloud%' THEN 'aws'
       WHEN u.provider = 'aws' AND lower(u.service_name) IN ('amazon relational database service', 'amazon rds') THEN 'aws:rds'
       WHEN u.provider = 'aws' THEN 'aws:elasticache'
       WHEN u.provider = 'gcp' AND lower(u.service_name) = 'compute engine' THEN 'gcp'
       WHEN u.provider = 'gcp' AND lower(u.service_name) = 'cloud sql' THEN 'gcp:cloudsql'
       WHEN u.provider = 'gcp' THEN 'gcp:memorystore'
       WHEN lower(u.service_name) = 'virtual machines' THEN 'azure'
       WHEN lower(u.service_name) = 'azure database for postgresql' THEN 'azure:postgresql' ELSE 'azure:redis' END AS family, COALESCE(sum(finops.convert_cost(u.billed_cost, u.currency::text, finops.reporting_currency(sqlc.arg(tenant_id), (SELECT d.currency FROM usage.usage_records d
    WHERE d.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR d.project_id = sqlc.narg(project_id))
      AND d.recorded_at >= sqlc.arg(from_ts) AND d.recorded_at < sqlc.arg(to_ts)
    GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1)), u.recorded_at::date)), 0)::float8 AS cost FROM usage.usage_records u
WHERE u.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR u.project_id = sqlc.narg(project_id))
  AND u.recorded_at >= sqlc.arg(from_ts) AND u.recorded_at < sqlc.arg(to_ts)
  AND ((u.provider = 'aws' AND (lower(u.service_name) LIKE '%elastic compute cloud%' OR lower(u.service_name) IN ('amazon relational database service', 'amazon rds', 'amazon elasticache', 'amazon elasticache service')))
    OR (u.provider = 'azure' AND lower(u.service_name) IN ('virtual machines', 'azure database for postgresql', 'redis cache', 'azure cache for redis'))
    OR (u.provider = 'gcp' AND lower(u.service_name) IN ('compute engine', 'cloud sql', 'cloud memorystore for redis', 'memorystore for redis', 'memorystore')))
GROUP BY 1, 2;

-- name: DashboardPriceIndex :many
SELECT provider AS family, region_id, idx::float8 AS idx FROM finops.region_price_index WHERE provider IN ('aws', 'azure', 'gcp', 'azure:postgresql', 'azure:redis', 'aws:rds', 'aws:elasticache', 'gcp:cloudsql', 'gcp:memorystore');

-- name: DashboardConnectionCost :many
-- Billed cost and data coverage of every connection over the period, in the organization's reporting currency (the
-- same conversion as the service table). A line belongs to the connection of its provider and account.
SELECT c.id::text AS connection_id,
       COALESCE(sum(finops.convert_cost(u.billed_cost, u.currency::text, finops.reporting_currency(sqlc.arg(tenant_id), (SELECT d.currency FROM usage.usage_records d
         WHERE d.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR d.project_id = sqlc.narg(project_id))
           AND d.recorded_at >= sqlc.arg(from_ts) AND d.recorded_at < sqlc.arg(to_ts)
         GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1)), u.recorded_at::date)), 0)::float8 AS cost,
       count(DISTINCT u.recorded_at::date)::int AS days,
       max(u.recorded_at)::timestamptz AS last_day
FROM usage.usage_records u
JOIN cloudaccounts.connections c ON c.tenant_id = u.tenant_id AND c.provider = u.provider AND c.account_ref = u.account_ref
WHERE u.tenant_id = sqlc.arg(tenant_id) AND (sqlc.narg(project_id)::uuid IS NULL OR u.project_id = sqlc.narg(project_id))
  AND u.recorded_at >= sqlc.arg(from_ts) AND u.recorded_at < sqlc.arg(to_ts)
GROUP BY c.id;

-- name: DashboardGridSources :many
-- Reference data (no tenant): every source of grid carbon intensity the platform has readings from, with the newest
-- reading and how many regions it covers. A source without readings is not listed.
SELECT provider, max(ts)::timestamptz AS last_at, count(DISTINCT region_id)::int AS regions
FROM carbon.grid_intensity WHERE NOT is_forecast GROUP BY provider ORDER BY max(ts) DESC;
