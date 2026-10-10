-- name: ListProjects :many
SELECT id, tenant_id, name, functional_unit, created_at, team_id
FROM projects.projects WHERE tenant_id = sqlc.arg(tenant_id) ORDER BY created_at DESC LIMIT 500;

-- name: GetProject :one
SELECT id, tenant_id, name, functional_unit, created_at, team_id
FROM projects.projects WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: CreateProject :one
INSERT INTO projects.projects (tenant_id, name, functional_unit)
VALUES (sqlc.arg(tenant_id), sqlc.arg(name), sqlc.arg(functional_unit))
RETURNING id, tenant_id, name, functional_unit, created_at, team_id;

-- name: UpsertFunctionalUnits :one
INSERT INTO projects.functional_unit_counts (tenant_id, project_id, period_start, period_end, units)
VALUES (sqlc.arg(tenant_id), sqlc.arg(project_id), sqlc.arg(period_start), sqlc.arg(period_end), sqlc.arg(units))
ON CONFLICT (project_id, period_start, period_end) DO UPDATE SET units = EXCLUDED.units
RETURNING id;

-- name: ListFunctionalUnits :many
SELECT id, project_id, period_start, period_end, units
FROM projects.functional_unit_counts
WHERE tenant_id = sqlc.arg(tenant_id) AND project_id = sqlc.arg(project_id)
ORDER BY period_start DESC LIMIT 200;

-- name: FunctionalUnitsForDay :one
-- R for a single day: every reported range covering the day contributes units spread evenly over its days.
SELECT COALESCE(sum(units / (period_end - period_start + 1)), 0)::float8 AS units
FROM projects.functional_unit_counts
WHERE tenant_id = sqlc.arg(tenant_id) AND project_id = sqlc.arg(project_id)
  AND period_start <= sqlc.arg(day)::date AND period_end >= sqlc.arg(day)::date;

-- name: GetPolicy :one
SELECT project_id, tenant_id, allowed_regions, ci_carbon_warn_kg, ci_carbon_block_kg, ci_cost_warn, ci_cost_block, updated_at, alert_carbon_monthly_kg, auto_apply_types
FROM projects.policies WHERE tenant_id = sqlc.arg(tenant_id) AND project_id = sqlc.arg(project_id);

-- name: UpsertPolicy :one
INSERT INTO projects.policies (project_id, tenant_id, allowed_regions, ci_carbon_warn_kg, ci_carbon_block_kg, ci_cost_warn, ci_cost_block, alert_carbon_monthly_kg, auto_apply_types)
VALUES (sqlc.arg(project_id), sqlc.arg(tenant_id), sqlc.arg(allowed_regions), sqlc.narg(ci_carbon_warn_kg), sqlc.narg(ci_carbon_block_kg), sqlc.narg(ci_cost_warn), sqlc.narg(ci_cost_block), sqlc.narg(alert_carbon_monthly_kg), sqlc.arg(auto_apply_types))
ON CONFLICT (project_id) DO UPDATE SET allowed_regions = EXCLUDED.allowed_regions, ci_carbon_warn_kg = EXCLUDED.ci_carbon_warn_kg,
  ci_carbon_block_kg = EXCLUDED.ci_carbon_block_kg, ci_cost_warn = EXCLUDED.ci_cost_warn, ci_cost_block = EXCLUDED.ci_cost_block,
  alert_carbon_monthly_kg = EXCLUDED.alert_carbon_monthly_kg, auto_apply_types = EXCLUDED.auto_apply_types, updated_at = now()
RETURNING project_id, tenant_id, allowed_regions, ci_carbon_warn_kg, ci_carbon_block_kg, ci_cost_warn, ci_cost_block, updated_at, alert_carbon_monthly_kg, auto_apply_types;


-- name: ListTeams :many
SELECT id, name, created_at FROM projects.teams WHERE tenant_id = sqlc.arg(tenant_id) ORDER BY name LIMIT 200;

-- name: CreateTeam :one
INSERT INTO projects.teams (tenant_id, name) VALUES (sqlc.arg(tenant_id), sqlc.arg(name)) RETURNING id, name, created_at;

-- name: DeleteTeam :execrows
DELETE FROM projects.teams WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: SetProjectTeam :execrows
UPDATE projects.projects SET team_id = sqlc.narg(team_id)
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: ListEnvironments :many
SELECT e.id, e.project_id, e.name, e.kind, e.created_at,
       COALESCE(array_agg(ec.connection_id ORDER BY ec.connection_id) FILTER (WHERE ec.connection_id IS NOT NULL), '{}')::uuid[] AS connection_ids
FROM projects.environments e
LEFT JOIN projects.environment_connections ec ON ec.environment_id = e.id
WHERE e.tenant_id = sqlc.arg(tenant_id)
GROUP BY e.id ORDER BY e.project_id, e.name LIMIT 500;

-- name: CreateEnvironment :one
INSERT INTO projects.environments (tenant_id, project_id, name, kind)
VALUES (sqlc.arg(tenant_id), sqlc.arg(project_id), sqlc.arg(name), sqlc.arg(kind))
RETURNING id, project_id, name, kind, created_at;

-- name: DeleteEnvironment :execrows
DELETE FROM projects.environments WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: GetEnvironment :one
SELECT id, project_id FROM projects.environments WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: ClearEnvironmentConnections :exec
DELETE FROM projects.environment_connections WHERE tenant_id = sqlc.arg(tenant_id) AND environment_id = sqlc.arg(environment_id);

-- name: AddEnvironmentConnections :execrows
-- Only connections of the environment's own project can be assigned; a connection that is already in another
-- environment moves here.
INSERT INTO projects.environment_connections (tenant_id, environment_id, connection_id)
SELECT sqlc.arg(tenant_id), sqlc.arg(environment_id), c.id
FROM cloudaccounts.connections c
WHERE c.tenant_id = sqlc.arg(tenant_id) AND c.project_id = sqlc.arg(project_id) AND c.id = ANY(sqlc.arg(connection_ids)::uuid[])
ON CONFLICT (connection_id) DO UPDATE SET environment_id = EXCLUDED.environment_id;

-- name: ListAllocationRules :many
SELECT id, name, source_project_id, service_name, created_at
FROM projects.allocation_rules WHERE tenant_id = sqlc.arg(tenant_id) ORDER BY created_at LIMIT 500;

-- name: ListAllocationTargets :many
SELECT rule_id, project_id, weight::float8 AS weight FROM projects.allocation_targets
WHERE tenant_id = sqlc.arg(tenant_id) ORDER BY rule_id, project_id;

-- name: CreateAllocationRule :one
INSERT INTO projects.allocation_rules (tenant_id, name, source_project_id, service_name)
VALUES (sqlc.arg(tenant_id), sqlc.arg(name), sqlc.arg(source_project_id), sqlc.arg(service_name))
RETURNING id, name, source_project_id, service_name, created_at;

-- name: CreateAllocationTarget :exec
INSERT INTO projects.allocation_targets (rule_id, tenant_id, project_id, weight)
VALUES (sqlc.arg(rule_id), sqlc.arg(tenant_id), sqlc.arg(project_id), sqlc.arg(weight));

-- name: DeleteAllocationRule :execrows
DELETE FROM projects.allocation_rules WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: AllocationCurrency :one
-- The currency the allocation report is shown in: the organization's reporting currency, else the dominant one of the window.
SELECT COALESCE(finops.reporting_currency(sqlc.arg(tenant_id), (SELECT d.currency FROM usage.usage_records d
    WHERE d.tenant_id = sqlc.arg(tenant_id) AND d.recorded_at >= sqlc.arg(from_ts) AND d.recorded_at < sqlc.arg(to_ts)
    GROUP BY d.currency ORDER BY sum(abs(d.billed_cost)) DESC, d.currency LIMIT 1)), '')::text AS currency;

-- name: ProjectServiceCost :many
-- Cost per project, environment and service in one currency at the rate of each day. An amount without a rate is left
-- out of the sum and counted in `excluded`, never guessed.
SELECT u.project_id, ec.environment_id, u.service_name,
       COALESCE(sum(finops.convert_cost(u.billed_cost, u.currency::text, sqlc.arg(currency)::text, u.recorded_at::date)), 0)::float8 AS cost,
       (count(*) FILTER (WHERE finops.convert_cost(u.billed_cost, u.currency::text, sqlc.arg(currency)::text, u.recorded_at::date) IS NULL))::int AS excluded
FROM usage.usage_records u
LEFT JOIN projects.environment_connections ec ON ec.connection_id = u.connection_id AND ec.tenant_id = u.tenant_id
WHERE u.tenant_id = sqlc.arg(tenant_id) AND u.recorded_at >= sqlc.arg(from_ts) AND u.recorded_at < sqlc.arg(to_ts)
GROUP BY u.project_id, ec.environment_id, u.service_name;
