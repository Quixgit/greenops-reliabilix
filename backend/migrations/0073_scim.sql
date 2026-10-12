-- +goose Up
-- SCIM 2.0 user provisioning (ADR-0056). The customer's identity provider pushes people with a bearer token (an API key of role 'scim');
-- a pushed person becomes a member when they first sign in with the same verified e-mail. Deprovisioning removes the membership.
ALTER TABLE tenants.api_keys DROP CONSTRAINT api_keys_role_check;
ALTER TABLE tenants.api_keys ADD CONSTRAINT api_keys_role_check CHECK (role IN ('ci','viewer','agent','scim'));

CREATE TABLE tenants.scim_users (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES tenants.organizations(id) ON DELETE CASCADE,
    external_id     text NOT NULL DEFAULT '' CHECK (char_length(external_id) <= 256),
    email           text NOT NULL CHECK (char_length(email) BETWEEN 3 AND 254),
    display_name    text NOT NULL DEFAULT '' CHECK (char_length(display_name) <= 200),
    role            text NOT NULL CHECK (role IN ('admin','engineer','viewer','billing')),
    active          boolean NOT NULL DEFAULT true,
    user_id         uuid REFERENCES tenants.users(id) ON DELETE SET NULL, -- set once the person has signed in and is a member
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX scim_users_email ON tenants.scim_users (organization_id, lower(email));
SELECT platform.enable_tenant_rls('tenants.scim_users', 'organization_id');
GRANT SELECT, INSERT, UPDATE, DELETE ON tenants.scim_users TO greenops_api;
GRANT SELECT, UPDATE, DELETE ON tenants.scim_users TO greenops_fanout;

-- First sign-in of a person nobody invited: if an identity provider pushed this verified e-mail, join that organization with the pushed
-- role. An existing membership is never changed. Returns the organization, or NULL when nothing was pushed.
-- +goose StatementBegin
CREATE FUNCTION tenants.claim_provisioned(p_sub text, p_email text) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE s tenants.scim_users%ROWTYPE; uid uuid;
BEGIN
  IF p_email IS NULL OR p_email = '' THEN RETURN NULL; END IF;
  SELECT * INTO s FROM tenants.scim_users
   WHERE lower(email) = lower(p_email) AND active AND user_id IS NULL ORDER BY created_at, id LIMIT 1 FOR UPDATE;
  IF NOT FOUND THEN RETURN NULL; END IF;
  INSERT INTO tenants.users (auth_sub, email) VALUES (p_sub, p_email)
    ON CONFLICT (auth_sub) DO UPDATE SET email = EXCLUDED.email RETURNING id INTO uid;
  INSERT INTO tenants.memberships (organization_id, user_id, role) VALUES (s.organization_id, uid, s.role)
    ON CONFLICT (organization_id, user_id) DO NOTHING;
  UPDATE tenants.scim_users SET user_id = uid, updated_at = now() WHERE id = s.id;
  INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target)
    VALUES (s.organization_id, p_sub, 'member.joined', 'user:' || uid);
  RETURN s.organization_id;
END $$;
-- +goose StatementEnd
ALTER FUNCTION tenants.claim_provisioned(text, text) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION tenants.claim_provisioned(text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION tenants.claim_provisioned(text, text) TO greenops_api;

-- +goose Down
DROP FUNCTION tenants.claim_provisioned(text, text);
DROP TABLE tenants.scim_users;
DELETE FROM tenants.api_keys WHERE role = 'scim';
ALTER TABLE tenants.api_keys DROP CONSTRAINT api_keys_role_check;
ALTER TABLE tenants.api_keys ADD CONSTRAINT api_keys_role_check CHECK (role IN ('ci','viewer','agent'));
