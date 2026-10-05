-- +goose Up
CREATE TABLE audit.audit_logs (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id     uuid NOT NULL REFERENCES tenants.organizations(id),
    actor_user_id text NOT NULL,
    action        text NOT NULL,
    target        text NOT NULL,
    request_id    text,
    metadata      jsonb NOT NULL DEFAULT '{}',
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON audit.audit_logs (tenant_id, created_at DESC);
SELECT platform.enable_tenant_rls('audit.audit_logs', 'tenant_id');

-- +goose Down
DROP TABLE audit.audit_logs;
