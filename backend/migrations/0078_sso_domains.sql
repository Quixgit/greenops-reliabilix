-- +goose Up
-- Company single sign-on by e-mail domain (ADR-0064). An organization claims a domain, proves it owns it with a DNS TXT record, and names the identity-provider
-- connection (an Auth0 enterprise connection, SAML or OIDC) its people sign in with. A verified domain routes the sign-in of its addresses to that
-- connection and, when jit_role is set, lets a person who signs in with a verified address of the domain join as that role without an invitation.
CREATE TABLE tenants.sso_domains (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid NOT NULL REFERENCES tenants.organizations(id) ON DELETE CASCADE,
    domain       text NOT NULL CHECK (domain ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$' AND char_length(domain) <= 253),
    connection   text NOT NULL CHECK (connection ~ '^[A-Za-z0-9][A-Za-z0-9_.-]{0,99}$'),
    token        text NOT NULL,                    -- the value the TXT record must carry
    jit_role     text CHECK (jit_role IN ('viewer', 'engineer', 'billing')), -- never owner or admin
    verified_at  timestamptz,
    created_by   text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, domain)
);
-- A domain belongs to one organization once verified (two organizations may try, only the one that proves it keeps it).
CREATE UNIQUE INDEX sso_domains_verified ON tenants.sso_domains (domain) WHERE verified_at IS NOT NULL;
SELECT platform.enable_tenant_rls('tenants.sso_domains', 'tenant_id');
GRANT SELECT, INSERT, UPDATE, DELETE ON tenants.sso_domains TO greenops_api;
GRANT SELECT, DELETE ON tenants.sso_domains TO greenops_fanout;

-- The sign-in page asks, before anyone is signed in, where an address signs in: the connection of a verified domain. Returns nothing else.
-- +goose StatementBegin
CREATE FUNCTION tenants.sso_connection_for(p_email text) RETURNS text
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT connection FROM tenants.sso_domains
   WHERE verified_at IS NOT NULL AND domain = lower(split_part(p_email, '@', 2)) AND position('@' IN p_email) > 1 LIMIT 1
$$;
-- +goose StatementEnd
ALTER FUNCTION tenants.sso_connection_for(text) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION tenants.sso_connection_for(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION tenants.sso_connection_for(text) TO greenops_api;
GRANT SELECT ON tenants.sso_domains TO greenops_fanout;

-- First sign-in of a person nobody invited or provisioned, with a verified address in a verified domain that allows it: join with the domain's role.
-- An existing membership is never changed. Returns the organization, or NULL.
-- +goose StatementBegin
CREATE FUNCTION tenants.claim_sso_domain(p_sub text, p_email text) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE s tenants.sso_domains%ROWTYPE; uid uuid;
BEGIN
  IF p_email IS NULL OR position('@' IN p_email) < 2 THEN RETURN NULL; END IF;
  SELECT * INTO s FROM tenants.sso_domains
   WHERE verified_at IS NOT NULL AND jit_role IS NOT NULL AND domain = lower(split_part(p_email, '@', 2)) LIMIT 1;
  IF NOT FOUND THEN RETURN NULL; END IF;
  INSERT INTO tenants.users (auth_sub, email) VALUES (p_sub, p_email)
    ON CONFLICT (auth_sub) DO UPDATE SET email = EXCLUDED.email RETURNING id INTO uid;
  INSERT INTO tenants.memberships (organization_id, user_id, role) VALUES (s.tenant_id, uid, s.jit_role)
    ON CONFLICT (organization_id, user_id) DO NOTHING;
  INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target)
    VALUES (s.tenant_id, p_sub, 'member.joined', 'user:' || uid);
  RETURN s.tenant_id;
END $$;
-- +goose StatementEnd
ALTER FUNCTION tenants.claim_sso_domain(text, text) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION tenants.claim_sso_domain(text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION tenants.claim_sso_domain(text, text) TO greenops_api;

-- +goose Down
DROP FUNCTION tenants.claim_sso_domain(text, text);
DROP FUNCTION tenants.sso_connection_for(text);
DROP TABLE tenants.sso_domains;
