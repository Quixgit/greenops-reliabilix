-- +goose Up
CREATE TABLE tenants (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    plan       text NOT NULL DEFAULT 'free',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE memberships (
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    user_sub  text NOT NULL,   -- OIDC subject; identities live in the IdP
    role      text NOT NULL CHECK (role IN ('owner','admin','engineer','viewer','billing')),
    PRIMARY KEY (tenant_id, user_sub)
);
-- Append-only audit trail (who did what, when).
CREATE TABLE audit_logs (
    id         bigserial PRIMARY KEY,
    tenant_id  uuid NOT NULL,
    actor_sub  text NOT NULL,
    action     text NOT NULL,
    target     text NOT NULL,
    request_id text,
    at         timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE memberships FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON memberships
    USING      (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON audit_logs
    USING      (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);
-- When provisioning DB roles: REVOKE UPDATE, DELETE, TRUNCATE ON audit_logs FROM greenops_app;

-- +goose Down
DROP TABLE audit_logs;
DROP TABLE memberships;
DROP TABLE tenants;
