-- +goose Up
-- ---- Narrow cross-tenant SECURITY DEFINER functions (owner: greenops_fanout, BYPASSRLS) -------------
-- Everything here either resolves *who is calling* (before any tenant is known) or fans jobs out.

-- Which tenant/role does this OIDC subject act as? (the DB, not the token, is authoritative for roles)
-- +goose StatementBegin
CREATE FUNCTION tenants.resolve_membership(p_sub text, p_wanted uuid)
RETURNS TABLE (tenant_id uuid, role text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT m.organization_id, m.role
  FROM tenants.memberships m JOIN tenants.users u ON u.id = m.user_id
  WHERE u.auth_sub = p_sub AND (p_wanted IS NULL OR m.organization_id = p_wanted)
  ORDER BY m.created_at, m.id LIMIT 1
$$;
-- +goose StatementEnd

-- Self-service onboarding: creates the user, a new organization and an owner membership. Idempotent.
-- +goose StatementBegin
CREATE FUNCTION tenants.onboard(p_sub text, p_email text, p_org_name text) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE uid uuid; oid uuid;
BEGIN
  INSERT INTO tenants.users (auth_sub, email) VALUES (p_sub, p_email)
    ON CONFLICT (auth_sub) DO UPDATE SET email = EXCLUDED.email RETURNING id INTO uid;
  SELECT m.organization_id INTO oid FROM tenants.memberships m WHERE m.user_id = uid ORDER BY m.created_at LIMIT 1;
  IF oid IS NULL THEN
    INSERT INTO tenants.organizations (name) VALUES (p_org_name) RETURNING id INTO oid;
    INSERT INTO tenants.memberships (organization_id, user_id, role) VALUES (oid, uid, 'owner');
    INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target) VALUES (oid, p_sub, 'tenant.onboarded', 'tenant:' || oid);
  END IF;
  RETURN oid;
END $$;
-- +goose StatementEnd

-- Accepts an invitation by token hash. Raises on unknown/expired/used tokens or an email mismatch.
-- +goose StatementBegin
CREATE FUNCTION tenants.accept_invitation(p_token_hash text, p_sub text, p_email text) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE inv tenants.invitations%ROWTYPE; uid uuid;
BEGIN
  SELECT * INTO inv FROM tenants.invitations WHERE token_hash = p_token_hash FOR UPDATE;
  IF NOT FOUND OR inv.accepted_at IS NOT NULL OR inv.expires_at < now() OR lower(inv.email) <> lower(p_email) THEN
    RAISE EXCEPTION 'invalid invitation' USING ERRCODE = 'P0001';
  END IF;
  INSERT INTO tenants.users (auth_sub, email) VALUES (p_sub, p_email)
    ON CONFLICT (auth_sub) DO UPDATE SET email = EXCLUDED.email RETURNING id INTO uid;
  INSERT INTO tenants.memberships (organization_id, user_id, role) VALUES (inv.organization_id, uid, inv.role)
    ON CONFLICT (organization_id, user_id) DO UPDATE SET role = EXCLUDED.role;
  UPDATE tenants.invitations SET accepted_at = now() WHERE id = inv.id;
  INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target)
    VALUES (inv.organization_id, p_sub, 'member.joined', 'user:' || uid);
  RETURN inv.organization_id;
END $$;
-- +goose StatementEnd

-- API-key authentication: tenant + role for a key hash (revoked keys resolve to nothing).
-- +goose StatementBegin
CREATE FUNCTION tenants.resolve_api_key(p_hash text) RETURNS TABLE (tenant_id uuid, role text, key_id uuid)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
BEGIN
  RETURN QUERY UPDATE tenants.api_keys k SET last_used_at = now()
    WHERE k.key_hash = p_hash AND k.revoked_at IS NULL RETURNING k.organization_id, k.role, k.id;
END $$;
-- +goose StatementEnd

-- Job fan-out (scheduler): every connection / project, across tenants.
-- +goose StatementBegin
CREATE FUNCTION cloudaccounts.list_connections_for_sync()
RETURNS TABLE (tenant_id uuid, project_id uuid, connection_id uuid, provider text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT c.tenant_id, c.project_id, c.id, c.provider FROM cloudaccounts.connections c WHERE c.sync_status <> 'error'
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION projects.list_projects_for_jobs() RETURNS TABLE (tenant_id uuid, project_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT p.tenant_id, p.id FROM projects.projects p
$$;
-- +goose StatementEnd

GRANT USAGE ON SCHEMA tenants, projects, cloudaccounts, audit TO greenops_fanout;
GRANT SELECT ON projects.projects, cloudaccounts.connections TO greenops_fanout;
GRANT INSERT ON audit.audit_logs TO greenops_fanout;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA audit TO greenops_fanout;

ALTER FUNCTION tenants.resolve_membership(text, uuid)            OWNER TO greenops_fanout;
ALTER FUNCTION tenants.onboard(text, text, text)                 OWNER TO greenops_fanout;
ALTER FUNCTION tenants.accept_invitation(text, text, text)       OWNER TO greenops_fanout;
ALTER FUNCTION tenants.resolve_api_key(text)                     OWNER TO greenops_fanout;
ALTER FUNCTION cloudaccounts.list_connections_for_sync()         OWNER TO greenops_fanout;
ALTER FUNCTION projects.list_projects_for_jobs()                 OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION tenants.resolve_membership(text, uuid), tenants.onboard(text, text, text),
  tenants.accept_invitation(text, text, text), tenants.resolve_api_key(text),
  cloudaccounts.list_connections_for_sync(), projects.list_projects_for_jobs() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION tenants.resolve_membership(text, uuid), tenants.onboard(text, text, text),
  tenants.accept_invitation(text, text, text), tenants.resolve_api_key(text) TO greenops_api;
GRANT EXECUTE ON FUNCTION cloudaccounts.list_connections_for_sync(), projects.list_projects_for_jobs(),
  platform.ensure_month_partition(regclass, text, date) TO greenops_worker;

-- ---- Table privileges -------------------------------------------------------------------------------
GRANT USAGE ON SCHEMA tenants, projects, cloudaccounts, usage, carbon, finops, recommendations, reports, audit, platform
  TO greenops_api, greenops_worker;

-- api: owns the user-facing writes. Usage and carbon are READ-ONLY for the API; only the worker writes them.
GRANT SELECT, INSERT, UPDATE, DELETE ON projects.projects, projects.functional_unit_counts, projects.policies,
  cloudaccounts.connections, finops.budgets, tenants.invitations TO greenops_api;
GRANT SELECT, UPDATE, DELETE ON tenants.memberships TO greenops_api;
GRANT SELECT, INSERT, UPDATE ON tenants.api_keys TO greenops_api;
GRANT SELECT ON tenants.users, tenants.organizations, cloudaccounts.sync_runs, usage.usage_records,
  carbon.calculations, carbon.grid_intensity, finops.region_price_index TO greenops_api;
GRANT SELECT, UPDATE ON recommendations.recommendations TO greenops_api;
GRANT SELECT, INSERT ON reports.reports TO greenops_api;
GRANT SELECT, INSERT ON audit.audit_logs TO greenops_api;

-- worker: ingestion, calculation, recommendation generation, report rendering.
GRANT SELECT ON projects.projects, projects.functional_unit_counts, projects.policies, finops.budgets,
  finops.region_price_index TO greenops_worker;
GRANT SELECT, UPDATE ON cloudaccounts.connections TO greenops_worker;
GRANT SELECT, INSERT, UPDATE ON cloudaccounts.sync_runs TO greenops_worker;
GRANT SELECT, INSERT, UPDATE, DELETE ON usage.usage_records, carbon.calculations TO greenops_worker;
GRANT SELECT, INSERT, UPDATE ON carbon.grid_intensity TO greenops_worker;
GRANT SELECT, INSERT, UPDATE ON recommendations.recommendations TO greenops_worker;
GRANT SELECT, UPDATE ON reports.reports TO greenops_worker;
GRANT SELECT, INSERT ON audit.audit_logs TO greenops_worker;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA audit TO greenops_api, greenops_worker;
-- Audit is append-only for everyone at runtime: no UPDATE/DELETE grants exist.

-- +goose Down
DROP FUNCTION projects.list_projects_for_jobs(), cloudaccounts.list_connections_for_sync(), tenants.resolve_api_key(text),
  tenants.accept_invitation(text, text, text), tenants.onboard(text, text, text), tenants.resolve_membership(text, uuid);
