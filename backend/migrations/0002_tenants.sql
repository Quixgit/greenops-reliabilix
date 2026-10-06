-- +goose Up
-- The organization IS the tenant: its id is the tenant_id used everywhere else.
CREATE TABLE tenants.organizations (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name               text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    plan               text NOT NULL DEFAULT 'free',
    white_label_config jsonb,
    created_at         timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE tenants.users (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    auth_sub   text NOT NULL UNIQUE,    -- Auth0 subject; credentials live in Auth0 only
    email      text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE tenants.memberships (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES tenants.organizations(id),
    user_id         uuid NOT NULL REFERENCES tenants.users(id),
    role            text NOT NULL CHECK (role IN ('owner','admin','engineer','viewer','billing')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, user_id)
);
CREATE TABLE tenants.invitations (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES tenants.organizations(id),
    email           text NOT NULL,
    role            text NOT NULL CHECK (role IN ('admin','engineer','viewer','billing')),
    token_hash      text NOT NULL UNIQUE,         -- sha256 of the one-time token; the token itself is never stored
    created_by      text NOT NULL,
    expires_at      timestamptz NOT NULL,
    accepted_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);
-- CI / automation access. Only a hash is stored; the key is shown once at creation.
CREATE TABLE tenants.api_keys (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES tenants.organizations(id),
    name            text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    prefix          text NOT NULL,                 -- first chars, for display
    key_hash        text NOT NULL UNIQUE,
    role            text NOT NULL CHECK (role IN ('ci','viewer')),
    created_by      text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    last_used_at    timestamptz,
    revoked_at      timestamptz
);
SELECT platform.enable_tenant_rls('tenants.organizations', 'id');
SELECT platform.enable_tenant_rls('tenants.memberships', 'organization_id');
SELECT platform.enable_tenant_rls('tenants.invitations', 'organization_id');
SELECT platform.enable_tenant_rls('tenants.api_keys', 'organization_id');
-- A user row is visible to the tenant only through a membership in it.
ALTER TABLE tenants.users ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.users FORCE ROW LEVEL SECURITY;
CREATE POLICY member_visibility ON tenants.users USING (EXISTS (
    SELECT 1 FROM tenants.memberships m
    WHERE m.user_id = users.id AND m.organization_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid));

GRANT USAGE ON SCHEMA tenants, audit TO greenops_fanout;
GRANT SELECT, INSERT, UPDATE ON tenants.organizations, tenants.users, tenants.memberships, tenants.invitations TO greenops_fanout;
GRANT SELECT, UPDATE ON tenants.api_keys TO greenops_fanout;

-- +goose Down
DROP TABLE tenants.api_keys, tenants.invitations, tenants.memberships, tenants.users, tenants.organizations;
