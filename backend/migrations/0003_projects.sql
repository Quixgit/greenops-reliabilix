-- +goose Up
CREATE TABLE projects.projects (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid NOT NULL REFERENCES tenants.organizations(id),
    name            text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    functional_unit text NOT NULL CHECK (char_length(functional_unit) BETWEEN 1 AND 60),  -- SCI "R", e.g. "api_request"
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);
-- How many functional units the project served in a period (SCI denominator R). Reported by the
-- customer (API/UI); without it the SCI score is NULL rather than guessed.
CREATE TABLE projects.functional_unit_counts (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id   uuid NOT NULL REFERENCES projects.projects(id) ON DELETE CASCADE,
    period_start date NOT NULL,
    period_end   date NOT NULL CHECK (period_end >= period_start),
    units        numeric(24,3) NOT NULL CHECK (units > 0),
    UNIQUE (project_id, period_start, period_end)
);
-- Per-project policy: data-residency allow-list (compliance for region_shift) and CI gate thresholds.
CREATE TABLE projects.policies (
    project_id          uuid PRIMARY KEY REFERENCES projects.projects(id) ON DELETE CASCADE,
    tenant_id           uuid NOT NULL REFERENCES tenants.organizations(id),
    allowed_regions     text[] NOT NULL DEFAULT '{}',   -- empty = residency unknown => no region_shift is ever proposed
    ci_carbon_warn_kg   numeric(18,3),
    ci_carbon_block_kg  numeric(18,3),
    ci_cost_warn        numeric(18,2),
    ci_cost_block       numeric(18,2),
    updated_at          timestamptz NOT NULL DEFAULT now()
);
SELECT platform.enable_tenant_rls('projects.projects', 'tenant_id');
SELECT platform.enable_tenant_rls('projects.functional_unit_counts', 'tenant_id');
SELECT platform.enable_tenant_rls('projects.policies', 'tenant_id');

-- +goose Down
DROP TABLE projects.policies, projects.functional_unit_counts, projects.projects;
