-- +goose Up
-- Time-limited support access. An organization owner or admin lets the platform's operators look at the working detail of the
-- organization (project names and policies, connection accounts and their last error, recent activity) for a bounded time. Without a
-- grant an operator sees counts only, as before; with one, every look is recorded where the organization can read it.
CREATE TABLE tenants.support_grants (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid NOT NULL REFERENCES tenants.organizations(id) ON DELETE CASCADE,
    granted_by text NOT NULL,
    granted_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    note       text NOT NULL DEFAULT '' CHECK (char_length(note) <= 300),
    CHECK (expires_at > granted_at AND expires_at <= granted_at + interval '7 days')
);
CREATE INDEX support_grants_tenant ON tenants.support_grants (tenant_id, expires_at DESC);
SELECT platform.enable_tenant_rls('tenants.support_grants', 'tenant_id');
GRANT SELECT, INSERT, UPDATE ON tenants.support_grants TO greenops_api;
GRANT SELECT ON tenants.support_grants TO greenops_fanout;

-- Every look an operator took while a grant was active. Written only by the function below; the organization reads it.
CREATE TABLE tenants.support_views (
    id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants.organizations(id) ON DELETE CASCADE,
    grant_id  uuid NOT NULL REFERENCES tenants.support_grants(id) ON DELETE CASCADE,
    operator  text NOT NULL CHECK (char_length(operator) <= 120),
    what      text NOT NULL,
    at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX support_views_tenant ON tenants.support_views (tenant_id, at DESC);
SELECT platform.enable_tenant_rls('tenants.support_views', 'tenant_id');
GRANT SELECT ON tenants.support_views TO greenops_api;
GRANT INSERT ON tenants.support_views TO greenops_fanout;

-- Whether support access is open, and until when: no organization data.
-- +goose StatementBegin
CREATE FUNCTION platform.operator_support_status(p_tenant uuid) RETURNS jsonb
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT COALESCE(
    (SELECT jsonb_build_object('active', true, 'expires_at', g.expires_at, 'granted_at', g.granted_at, 'granted_by', g.granted_by, 'note', g.note)
       FROM tenants.support_grants g
      WHERE g.tenant_id = p_tenant AND g.revoked_at IS NULL AND g.expires_at > now()
      ORDER BY g.expires_at DESC LIMIT 1),
    jsonb_build_object('active', false))
$$;
-- +goose StatementEnd

-- The working detail of an organization. It refuses without an active grant, and records the look before it answers.
-- +goose StatementBegin
CREATE FUNCTION platform.operator_support_overview(p_actor text, p_tenant uuid, p_label text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE v_grant uuid; v_expires timestamptz;
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  SELECT g.id, g.expires_at INTO v_grant, v_expires FROM tenants.support_grants g
   WHERE g.tenant_id = p_tenant AND g.revoked_at IS NULL AND g.expires_at > now() ORDER BY g.expires_at DESC LIMIT 1;
  IF v_grant IS NULL THEN RAISE EXCEPTION 'no active support access' USING ERRCODE = 'P0002'; END IF;
  INSERT INTO tenants.support_views (tenant_id, grant_id, operator, what) VALUES (p_tenant, v_grant, left(coalesce(nullif(p_label, ''), p_actor), 120), 'overview');
  RETURN jsonb_build_object(
    'expires_at', v_expires,
    'projects', (SELECT coalesce(jsonb_agg(jsonb_build_object('id', p.id, 'name', p.name, 'functional_unit', p.functional_unit,
                  'allowed_regions', coalesce(to_jsonb(pol.allowed_regions), '[]'::jsonb), 'created_at', p.created_at) ORDER BY p.created_at), '[]'::jsonb)
                  FROM projects.projects p LEFT JOIN projects.policies pol ON pol.project_id = p.id WHERE p.tenant_id = p_tenant),
    'connections', (SELECT coalesce(jsonb_agg(jsonb_build_object('id', c.id, 'provider', c.provider, 'account_ref', c.account_ref, 'status', c.sync_status,
                  'last_error', left(c.last_error, 200), 'last_sync_at', c.last_sync_at, 'synced_through', c.synced_through) ORDER BY c.created_at), '[]'::jsonb)
                  FROM cloudaccounts.connections c WHERE c.tenant_id = p_tenant),
    'budgets', (SELECT count(*) FROM finops.budgets WHERE tenant_id = p_tenant),
    'recommendations_open', (SELECT count(*) FROM recommendations.recommendations WHERE tenant_id = p_tenant AND status = 'open'),
    'recent_activity', (SELECT coalesce(jsonb_agg(jsonb_build_object('at', a.created_at, 'actor', a.actor_user_id, 'action', a.action, 'target', a.target)), '[]'::jsonb)
                  FROM (SELECT created_at, actor_user_id, action, target FROM audit.audit_logs WHERE tenant_id = p_tenant ORDER BY created_at DESC, id DESC LIMIT 30) a));
END
$$;
-- +goose StatementEnd

ALTER FUNCTION platform.operator_support_status(uuid) OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_support_overview(text, uuid, text) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.operator_support_status(uuid), platform.operator_support_overview(text, uuid, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.operator_support_status(uuid), platform.operator_support_overview(text, uuid, text) TO greenops_api;
GRANT USAGE, SELECT ON SEQUENCE tenants.support_views_id_seq TO greenops_fanout;
-- the erasure of an organization removes its rows from every tenant table, these two included
GRANT DELETE ON tenants.support_views, tenants.support_grants TO greenops_fanout;

-- +goose Down
DROP FUNCTION platform.operator_support_overview(text, uuid, text);
DROP FUNCTION platform.operator_support_status(uuid);
DROP TABLE tenants.support_views;
DROP TABLE tenants.support_grants;
