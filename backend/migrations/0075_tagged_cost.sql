-- +goose Up
-- Cost by cloud tag (ADR-0061): for every day and service, the cost of the charges that carried one value of one tag key. It is a view of
-- the same money as usage.usage_records (never added to it), kept small: only the organization's allocation keys are stored, one row per
-- day, service, key and value, so the table grows with the number of distinct tag values, not with the number of resources.
CREATE TABLE usage.tagged_cost (
    tenant_id      uuid NOT NULL REFERENCES tenants.organizations(id) ON DELETE CASCADE,
    project_id     uuid NOT NULL,
    source_key     text NOT NULL,                         -- the connection (or import) the figures came from: a re-read replaces its own rows
    provider       text NOT NULL,
    service_name   text NOT NULL,
    currency       char(3) NOT NULL,
    day            date NOT NULL,
    tag_key        text NOT NULL CHECK (char_length(tag_key) BETWEEN 1 AND 64),
    tag_value      text NOT NULL CHECK (char_length(tag_value) BETWEEN 1 AND 128),
    billed_cost    numeric(18,6) NOT NULL,
    effective_cost numeric(18,6) NOT NULL,
    updated_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, project_id, source_key, provider, service_name, currency, day, tag_key, tag_value),
    FOREIGN KEY (tenant_id, project_id) REFERENCES projects.projects (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX tagged_cost_by_key ON usage.tagged_cost (tenant_id, tag_key, day);
SELECT platform.enable_tenant_rls('usage.tagged_cost', 'tenant_id');
GRANT SELECT ON usage.tagged_cost TO greenops_api;
GRANT SELECT, INSERT, UPDATE, DELETE ON usage.tagged_cost TO greenops_worker;
GRANT SELECT, DELETE ON usage.tagged_cost TO greenops_fanout;

-- +goose Down
DROP TABLE usage.tagged_cost;
