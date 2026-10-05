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
    UNIQUE (organization_id, user_id)
);
SELECT platform.enable_tenant_rls('tenants.organizations', 'id');
SELECT platform.enable_tenant_rls('tenants.memberships', 'organization_id');

-- +goose Down
DROP TABLE tenants.memberships; DROP TABLE tenants.users; DROP TABLE tenants.organizations;
