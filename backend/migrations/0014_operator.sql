-- +goose Up
-- Platform operator console (ADR-0020). Operators are people who run the platform, not members of a tenant. Who
-- is an operator is decided here, in the database, and only the migration owner can change it (there is no
-- runtime grant on platform.operators): a compromised API process cannot make anyone an operator.
-- Cross-tenant reads and the few operator actions are narrow SECURITY DEFINER functions owned by greenops_fanout
-- (BYPASSRLS) that return metadata only: never payloads, addresses, tokens or usage data.
CREATE TABLE platform.operators (
    auth_sub   text PRIMARY KEY CHECK (char_length(auth_sub) BETWEEN 1 AND 255),
    email      text NOT NULL DEFAULT '' CHECK (char_length(email) <= 320),
    role       text NOT NULL CHECK (role IN ('viewer','admin')),   -- viewer: read only; admin: may also retry/disable
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Append-only record of what operators looked at and did (reads included: they see metadata of every tenant).
CREATE TABLE platform.operator_access_log (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    at           timestamptz NOT NULL DEFAULT now(),
    operator_sub text NOT NULL,
    action       text NOT NULL CHECK (char_length(action) <= 300)
);
CREATE INDEX ON platform.operator_access_log (at DESC);

-- ---- identity ---------------------------------------------------------------------------------------------
-- +goose StatementBegin
CREATE FUNCTION platform.operator_role(p_sub text) RETURNS text
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT o.role FROM platform.operators o WHERE o.auth_sub = p_sub
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION platform.operator_log(p_sub text, p_action text) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM platform.operators WHERE auth_sub = p_sub) THEN
    RAISE EXCEPTION 'not an operator' USING ERRCODE = 'P0001';
  END IF;
  INSERT INTO platform.operator_access_log (operator_sub, action) VALUES (p_sub, left(p_action, 300));
END $$;
-- +goose StatementEnd

-- ---- cross-tenant reads (metadata only) -------------------------------------------------------------------
-- +goose StatementBegin
CREATE FUNCTION platform.operator_tenants(p_limit int)
RETURNS TABLE (tenant_id uuid, name text, plan text, created_at timestamptz, members int, projects int, connections int,
               connections_in_error int, destinations int, failed_deliveries_7d int)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT o.id, o.name, o.plan, o.created_at,
    (SELECT count(*)::int FROM tenants.memberships m WHERE m.organization_id = o.id),
    (SELECT count(*)::int FROM projects.projects p WHERE p.tenant_id = o.id),
    (SELECT count(*)::int FROM cloudaccounts.connections c WHERE c.tenant_id = o.id),
    (SELECT count(*)::int FROM cloudaccounts.connections c WHERE c.tenant_id = o.id AND c.sync_status = 'error'),
    (SELECT count(*)::int FROM notifications.destinations d WHERE d.tenant_id = o.id),
    (SELECT count(*)::int FROM notifications.deliveries x WHERE x.tenant_id = o.id AND x.status = 'failed' AND x.created_at > now() - interval '7 days')
  FROM tenants.organizations o ORDER BY o.created_at DESC LIMIT least(greatest(p_limit, 1), 500)
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION notifications.operator_deliveries(p_status text, p_limit int)
RETURNS TABLE (tenant_id uuid, tenant_name text, id uuid, destination_id uuid, kind text, destination_name text, event_type text,
               severity text, status text, attempts int, next_attempt_at timestamptz, last_error text, created_at timestamptz, sent_at timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT x.tenant_id, o.name, x.id, x.destination_id, x.kind, x.destination_name, x.event_type, x.severity, x.status, x.attempts,
         x.next_attempt_at, x.last_error, x.created_at, x.sent_at
  FROM notifications.deliveries x JOIN tenants.organizations o ON o.id = x.tenant_id
  WHERE p_status IS NULL OR x.status = p_status
  ORDER BY x.created_at DESC LIMIT least(greatest(p_limit, 1), 200)
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION notifications.operator_stats(p_hours int) RETURNS TABLE (kind text, status text, n bigint)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT x.kind, x.status, count(*) FROM notifications.deliveries x
  WHERE x.created_at > now() - make_interval(hours => least(greatest(p_hours, 1), 24 * 30))
  GROUP BY x.kind, x.status ORDER BY x.kind, x.status
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION cloudaccounts.operator_unhealthy(p_limit int)
RETURNS TABLE (tenant_id uuid, tenant_name text, provider text, account_ref text, sync_status text, last_error text, last_sync_at timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT c.tenant_id, o.name, c.provider, c.account_ref, c.sync_status, c.last_error, c.last_sync_at
  FROM cloudaccounts.connections c JOIN tenants.organizations o ON o.id = c.tenant_id
  WHERE c.sync_status = 'error' ORDER BY c.last_sync_at NULLS FIRST LIMIT least(greatest(p_limit, 1), 200)
$$;
-- +goose StatementEnd

-- ---- operator actions (each one is audited in the affected tenant's own audit log) ------------------------
-- +goose StatementBegin
CREATE FUNCTION notifications.operator_retry(p_actor text, p_tenant uuid, p_delivery uuid) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE n int;
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  UPDATE notifications.deliveries SET status = 'pending', attempts = 0, next_attempt_at = now(), lease_until = NULL
   WHERE id = p_delivery AND tenant_id = p_tenant AND status = 'failed' AND destination_id IS NOT NULL;
  GET DIAGNOSTICS n = ROW_COUNT;
  IF n = 0 THEN RETURN false; END IF;
  INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target)
    VALUES (p_tenant, 'operator:' || p_actor, 'operator.delivery_retried', 'delivery:' || p_delivery);
  RETURN true;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION notifications.operator_set_enabled(p_actor text, p_tenant uuid, p_destination uuid, p_enabled boolean) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE n int;
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  UPDATE notifications.destinations SET enabled = p_enabled WHERE id = p_destination AND tenant_id = p_tenant;
  GET DIAGNOSTICS n = ROW_COUNT;
  IF n = 0 THEN RETURN false; END IF;
  INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target)
    VALUES (p_tenant, 'operator:' || p_actor,
            CASE WHEN p_enabled THEN 'operator.destination_enabled' ELSE 'operator.destination_disabled' END, 'destination:' || p_destination);
  RETURN true;
END $$;
-- +goose StatementEnd

-- ---- ownership and grants ---------------------------------------------------------------------------------
GRANT USAGE ON SCHEMA platform, notifications, cloudaccounts, projects, tenants, audit TO greenops_fanout;
GRANT SELECT ON platform.operators TO greenops_fanout;
GRANT INSERT ON platform.operator_access_log TO greenops_fanout;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA platform TO greenops_fanout;
GRANT SELECT ON tenants.organizations, tenants.memberships TO greenops_fanout;
GRANT SELECT ON notifications.destinations TO greenops_fanout;
GRANT UPDATE (enabled) ON notifications.destinations TO greenops_fanout;
GRANT UPDATE (status, attempts, next_attempt_at, lease_until) ON notifications.deliveries TO greenops_fanout;

ALTER FUNCTION platform.operator_role(text)                                        OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_log(text, text)                                   OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_tenants(int)                                      OWNER TO greenops_fanout;
ALTER FUNCTION notifications.operator_deliveries(text, int)                        OWNER TO greenops_fanout;
ALTER FUNCTION notifications.operator_stats(int)                                   OWNER TO greenops_fanout;
ALTER FUNCTION cloudaccounts.operator_unhealthy(int)                               OWNER TO greenops_fanout;
ALTER FUNCTION notifications.operator_retry(text, uuid, uuid)                      OWNER TO greenops_fanout;
ALTER FUNCTION notifications.operator_set_enabled(text, uuid, uuid, boolean)       OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.operator_role(text), platform.operator_log(text, text), platform.operator_tenants(int),
  notifications.operator_deliveries(text, int), notifications.operator_stats(int), cloudaccounts.operator_unhealthy(int),
  notifications.operator_retry(text, uuid, uuid), notifications.operator_set_enabled(text, uuid, uuid, boolean) FROM PUBLIC;
-- Only the API process serves the console; the worker never calls these.
GRANT EXECUTE ON FUNCTION platform.operator_role(text), platform.operator_log(text, text), platform.operator_tenants(int),
  notifications.operator_deliveries(text, int), notifications.operator_stats(int), cloudaccounts.operator_unhealthy(int),
  notifications.operator_retry(text, uuid, uuid), notifications.operator_set_enabled(text, uuid, uuid, boolean) TO greenops_api;

-- +goose Down
DROP FUNCTION notifications.operator_set_enabled(text, uuid, uuid, boolean), notifications.operator_retry(text, uuid, uuid),
  cloudaccounts.operator_unhealthy(int), notifications.operator_stats(int), notifications.operator_deliveries(text, int),
  platform.operator_tenants(int), platform.operator_log(text, text), platform.operator_role(text);
DROP TABLE platform.operator_access_log;
DROP TABLE platform.operators;
