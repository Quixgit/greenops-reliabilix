-- name: ListProjects :many
SELECT id, tenant_id, name, functional_unit, created_at
FROM projects.projects WHERE tenant_id = sqlc.arg(tenant_id) ORDER BY created_at DESC LIMIT 500;

-- name: GetProject :one
SELECT id, tenant_id, name, functional_unit, created_at
FROM projects.projects WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: CreateProject :one
INSERT INTO projects.projects (tenant_id, name, functional_unit)
VALUES (sqlc.arg(tenant_id), sqlc.arg(name), sqlc.arg(functional_unit))
RETURNING id, tenant_id, name, functional_unit, created_at;

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
SELECT project_id, tenant_id, allowed_regions, ci_carbon_warn_kg, ci_carbon_block_kg, ci_cost_warn, ci_cost_block, updated_at
FROM projects.policies WHERE tenant_id = sqlc.arg(tenant_id) AND project_id = sqlc.arg(project_id);

-- name: UpsertPolicy :one
INSERT INTO projects.policies (project_id, tenant_id, allowed_regions, ci_carbon_warn_kg, ci_carbon_block_kg, ci_cost_warn, ci_cost_block)
VALUES (sqlc.arg(project_id), sqlc.arg(tenant_id), sqlc.arg(allowed_regions), sqlc.narg(ci_carbon_warn_kg), sqlc.narg(ci_carbon_block_kg), sqlc.narg(ci_cost_warn), sqlc.narg(ci_cost_block))
ON CONFLICT (project_id) DO UPDATE SET allowed_regions = EXCLUDED.allowed_regions, ci_carbon_warn_kg = EXCLUDED.ci_carbon_warn_kg,
  ci_carbon_block_kg = EXCLUDED.ci_carbon_block_kg, ci_cost_warn = EXCLUDED.ci_cost_warn, ci_cost_block = EXCLUDED.ci_cost_block, updated_at = now()
RETURNING project_id, tenant_id, allowed_regions, ci_carbon_warn_kg, ci_carbon_block_kg, ci_cost_warn, ci_cost_block, updated_at;

