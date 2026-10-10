-- +goose Up
-- Custom roles (ADR-0037): an organization defines its own roles as named sets of permissions and assigns or withdraws
-- them per member. The built-in roles stay; a member holds either a built-in role or a custom one. When a custom role is
-- deleted, its members fall back to the built-in role stored on the membership.
CREATE TABLE tenants.custom_roles (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES tenants.organizations(id),
    name            text NOT NULL CHECK (char_length(name) BETWEEN 2 AND 60),
    description     text NOT NULL DEFAULT '' CHECK (char_length(description) <= 200),
    permissions     text[] NOT NULL CHECK (cardinality(permissions) BETWEEN 1 AND 60),
    created_by      text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX custom_roles_name ON tenants.custom_roles (organization_id, lower(name));
SELECT platform.enable_tenant_rls('tenants.custom_roles', 'organization_id');
GRANT SELECT, INSERT, UPDATE, DELETE ON tenants.custom_roles TO greenops_api;
GRANT SELECT, DELETE ON tenants.custom_roles TO greenops_fanout; -- DELETE: organization erasure (ADR-0027)

ALTER TABLE tenants.memberships ADD COLUMN custom_role_id uuid REFERENCES tenants.custom_roles(id) ON DELETE SET NULL;

-- A member with a custom role resolves to the role "custom"; the application then loads the permission set.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION tenants.resolve_membership(p_sub text, p_wanted uuid)
RETURNS TABLE (tenant_id uuid, role text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT m.organization_id, CASE WHEN m.custom_role_id IS NOT NULL THEN 'custom' ELSE m.role END
  FROM tenants.memberships m JOIN tenants.users u ON u.id = m.user_id
  WHERE u.auth_sub = p_sub AND (p_wanted IS NULL OR m.organization_id = p_wanted)
  ORDER BY m.created_at, m.id LIMIT 1
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION tenants.membership_permissions(p_sub text, p_tenant uuid) RETURNS text[]
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT cr.permissions
  FROM tenants.memberships m JOIN tenants.users u ON u.id = m.user_id JOIN tenants.custom_roles cr ON cr.id = m.custom_role_id
  WHERE u.auth_sub = p_sub AND m.organization_id = p_tenant
$$;
-- +goose StatementEnd
ALTER FUNCTION tenants.membership_permissions(text, uuid) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION tenants.membership_permissions(text, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION tenants.membership_permissions(text, uuid) TO greenops_api;

-- +goose Down
DROP FUNCTION tenants.membership_permissions(text, uuid);
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION tenants.resolve_membership(p_sub text, p_wanted uuid)
RETURNS TABLE (tenant_id uuid, role text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT m.organization_id, m.role
  FROM tenants.memberships m JOIN tenants.users u ON u.id = m.user_id
  WHERE u.auth_sub = p_sub AND (p_wanted IS NULL OR m.organization_id = p_wanted)
  ORDER BY m.created_at, m.id LIMIT 1
$$;
-- +goose StatementEnd
ALTER TABLE tenants.memberships DROP COLUMN custom_role_id;
DROP TABLE tenants.custom_roles;
