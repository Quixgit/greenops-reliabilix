-- +goose Up
CREATE TABLE projects.projects (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid NOT NULL REFERENCES tenants.organizations(id),
    name            text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    functional_unit text NOT NULL CHECK (char_length(functional_unit) BETWEEN 1 AND 60),  -- SCI "R"
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);
SELECT platform.enable_tenant_rls('projects.projects', 'tenant_id');

-- +goose Down
DROP TABLE projects.projects;
