-- +goose Up
-- Cost files imported from vendors that are not one of the three clouds (Datadog, Snowflake ...; ADR-0062). One row per uploaded file; the lines
-- it made are usage records of the provider "saas" whose source is 'import:<id>', so a file can be taken back by its id.
CREATE TABLE usage.imports (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid NOT NULL REFERENCES tenants.organizations(id) ON DELETE CASCADE,
    project_id    uuid NOT NULL,
    vendor        text NOT NULL CHECK (char_length(vendor) BETWEEN 1 AND 60),
    filename      text NOT NULL DEFAULT '' CHECK (char_length(filename) <= 200),
    currency      char(3) NOT NULL,
    status        text NOT NULL DEFAULT 'processing' CHECK (status IN ('processing', 'done', 'failed', 'deleting')),
    error         text NOT NULL DEFAULT '' CHECK (char_length(error) <= 500),
    rows_read     integer NOT NULL DEFAULT 0,
    rows_imported integer NOT NULL DEFAULT 0,
    rows_skipped  integer NOT NULL DEFAULT 0,
    services      integer NOT NULL DEFAULT 0,
    from_day      date,
    to_day        date,
    total_cost    numeric(18,2) NOT NULL DEFAULT 0,
    warnings      text[] NOT NULL DEFAULT '{}',
    created_by    text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    finished_at   timestamptz,
    FOREIGN KEY (tenant_id, project_id) REFERENCES projects.projects (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX imports_by_tenant ON usage.imports (tenant_id, created_at DESC);
SELECT platform.enable_tenant_rls('usage.imports', 'tenant_id');
GRANT SELECT, INSERT, UPDATE ON usage.imports TO greenops_api;
GRANT SELECT, UPDATE, DELETE ON usage.imports TO greenops_worker;
GRANT SELECT, DELETE ON usage.imports TO greenops_fanout;
-- Taking an import back removes the lines it made (the worker, for its own organization only: row-level security).
GRANT DELETE ON usage.usage_records TO greenops_worker;

-- +goose Down
DROP TABLE usage.imports;
REVOKE DELETE ON usage.usage_records FROM greenops_worker;
