-- +goose Up
-- Operator console, second round: announcements to every organization (platform updates, maintenance, notices), growth
-- metrics over time, one organization's overview and timeline, and an operator-requested erasure. Every function checks
-- the operator's role itself and works on metadata only (counts, ids, dates), as the console always has.

CREATE TABLE platform.announcements (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind           text NOT NULL CHECK (kind IN ('update','maintenance','notice','incident')),
    severity       text NOT NULL CHECK (severity IN ('info','warning','critical')),
    title          text NOT NULL CHECK (char_length(title) BETWEEN 3 AND 200),
    body           text NOT NULL DEFAULT '' CHECK (char_length(body) <= 2000),
    path           text NOT NULL DEFAULT '' CHECK (char_length(path) <= 200),
    audience_plans text[] NOT NULL DEFAULT '{}',
    email_copy     boolean NOT NULL DEFAULT false,
    created_by     text NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    recipients     integer NOT NULL DEFAULT 0,
    withdrawn_at   timestamptz,
    withdrawn_by   text
);
GRANT SELECT, INSERT, UPDATE ON platform.announcements TO greenops_fanout;
-- Announcements are written into the inboxes by the function below, which runs as this role.
GRANT INSERT ON notifications.inbox TO greenops_fanout;
GRANT SELECT ON tenants.deletion_requests, audit.audit_logs, carbon.gate_evaluations, platform.error_groups TO greenops_fanout;
GRANT INSERT ON tenants.deletion_requests, audit.audit_logs TO greenops_fanout;
GRANT DELETE ON tenants.deletion_requests TO greenops_fanout;

-- +goose StatementBegin
CREATE FUNCTION platform.operator_announce(p_actor text, p_kind text, p_severity text, p_title text, p_body text, p_path text,
    p_plans text[], p_email boolean) RETURNS TABLE (id uuid, recipients int, emails text[][])
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE aid uuid; n int;
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  INSERT INTO platform.announcements (kind, severity, title, body, path, audience_plans, email_copy, created_by)
    VALUES (p_kind, p_severity, btrim(p_title), btrim(coalesce(p_body, '')), coalesce(p_path, ''), coalesce(p_plans, '{}'), coalesce(p_email, false), p_actor)
    RETURNING platform.announcements.id INTO aid;
  -- Straight into every matching organization's inbox, whatever its notification switches say: an operator's notice
  -- about the platform itself is not an event the organization subscribed to.
  INSERT INTO notifications.inbox (tenant_id, type, severity, title, body, path, dedupe_key)
    SELECT o.id, 'platform.announcement', p_severity, btrim(p_title), btrim(coalesce(p_body, '')), coalesce(p_path, ''), 'announcement:' || aid
    FROM tenants.organizations o WHERE cardinality(coalesce(p_plans, '{}')) = 0 OR o.plan = ANY (p_plans)
    ON CONFLICT DO NOTHING;
  GET DIAGNOSTICS n = ROW_COUNT;
  UPDATE platform.announcements a SET recipients = n WHERE a.id = aid;
  RETURN QUERY SELECT aid, n, NULL::text[][];
END $$;
-- +goose StatementEnd

-- The addresses to e-mail for an announcement: owners and admins of the matching organizations.
-- +goose StatementBegin
CREATE FUNCTION platform.announcement_contacts(p_actor text, p_plans text[]) RETURNS TABLE (tenant_id uuid, emails text[])
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  RETURN QUERY
    SELECT o.id, coalesce((SELECT array_agg(DISTINCT u.email) FROM tenants.memberships m JOIN tenants.users u ON u.id = m.user_id
                            WHERE m.organization_id = o.id AND m.role IN ('owner','admin') AND u.email <> ''), '{}')
    FROM tenants.organizations o WHERE cardinality(coalesce(p_plans, '{}')) = 0 OR o.plan = ANY (p_plans);
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION platform.operator_announcements(p_limit int)
RETURNS TABLE (id uuid, kind text, severity text, title text, body text, path text, audience_plans text[], email_copy boolean,
               created_by text, created_at timestamptz, recipients int, withdrawn_at timestamptz, read_by bigint)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT a.id, a.kind, a.severity, a.title, a.body, a.path, a.audience_plans, a.email_copy, a.created_by, a.created_at, a.recipients, a.withdrawn_at,
         (SELECT count(DISTINCT s.tenant_id) FROM notifications.inbox_state s
           JOIN notifications.inbox i ON i.tenant_id = s.tenant_id AND i.dedupe_key = 'announcement:' || a.id AND s.read_until >= i.created_at)
  FROM platform.announcements a ORDER BY a.created_at DESC LIMIT least(greatest(p_limit, 1), 200)
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION platform.operator_withdraw_announcement(p_actor text, p_id uuid) RETURNS int
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE n int;
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  UPDATE platform.announcements SET withdrawn_at = now(), withdrawn_by = p_actor WHERE id = p_id AND withdrawn_at IS NULL;
  IF NOT FOUND THEN RETURN -1; END IF;
  DELETE FROM notifications.inbox WHERE dedupe_key = 'announcement:' || p_id;
  GET DIAGNOSTICS n = ROW_COUNT;
  RETURN n;
END $$;
-- +goose StatementEnd

-- Growth over the last p_days days, day by day, and the shape of the platform now.
-- +goose StatementBegin
CREATE FUNCTION platform.operator_metrics(p_days int) RETURNS jsonb
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE d int := least(greatest(p_days, 7), 180); since date := (current_date - (d - 1));
        series jsonb; totals jsonb; plans jsonb; subs jsonb;
BEGIN
  SELECT jsonb_agg(jsonb_build_object(
      'day', g.day,
      'organizations', (SELECT count(*) FROM tenants.organizations x WHERE x.created_at::date = g.day),
      'users',         (SELECT count(*) FROM tenants.users x WHERE x.created_at::date = g.day),
      'projects',      (SELECT count(*) FROM projects.projects x WHERE x.created_at::date = g.day),
      'connections',   (SELECT count(*) FROM cloudaccounts.connections x WHERE x.created_at::date = g.day),
      'reports',       (SELECT count(*) FROM reports.reports x WHERE x.created_at::date = g.day),
      'gate_checks',   (SELECT count(*) FROM carbon.gate_evaluations x WHERE x.created_at::date = g.day),
      'recommendations', (SELECT count(*) FROM recommendations.recommendations x WHERE x.created_at::date = g.day),
      'deliveries',    (SELECT count(*) FROM notifications.deliveries x WHERE x.created_at::date = g.day),
      'deliveries_failed', (SELECT count(*) FROM notifications.deliveries x WHERE x.created_at::date = g.day AND x.status = 'failed'),
      'errors',        (SELECT count(*) FROM platform.error_groups x WHERE x.first_seen::date = g.day),
      'active_organizations', (SELECT count(DISTINCT x.tenant_id) FROM audit.audit_logs x WHERE x.created_at::date = g.day)
    ) ORDER BY g.day)
  INTO series FROM generate_series(since, current_date, interval '1 day') AS g(day);
  SELECT jsonb_build_object(
      'organizations', (SELECT count(*) FROM tenants.organizations), 'users', (SELECT count(*) FROM tenants.users),
      'projects', (SELECT count(*) FROM projects.projects), 'connections', (SELECT count(*) FROM cloudaccounts.connections),
      'reports', (SELECT count(*) FROM reports.reports), 'recommendations', (SELECT count(*) FROM recommendations.recommendations),
      'open_errors', (SELECT count(*) FROM platform.error_groups WHERE status = 'open'),
      'announcements', (SELECT count(*) FROM platform.announcements WHERE withdrawn_at IS NULL),
      'pending_erasures', (SELECT count(*) FROM tenants.deletion_requests))
  INTO totals;
  SELECT coalesce(jsonb_agg(jsonb_build_object('plan', plan, 'organizations', n) ORDER BY n DESC), '[]') INTO plans
    FROM (SELECT plan, count(*) AS n FROM tenants.organizations GROUP BY plan) p;
  SELECT coalesce(jsonb_agg(jsonb_build_object('status', status, 'organizations', n)), '[]') INTO subs
    FROM (SELECT status, count(*) AS n FROM tenants.subscriptions GROUP BY status) s;
  RETURN jsonb_build_object('days', d, 'series', coalesce(series, '[]'), 'totals', totals, 'plans', plans, 'subscriptions', subs);
END $$;
-- +goose StatementEnd

-- One organization: what it has, when it started, when it was last active, and the trail of what happened.
-- +goose StatementBegin
CREATE FUNCTION platform.operator_org_overview(p_tenant uuid) RETURNS jsonb
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT jsonb_build_object(
    'id', o.id, 'name', o.name, 'plan', o.plan, 'created_at', o.created_at,
    'members', (SELECT count(*) FROM tenants.memberships WHERE organization_id = o.id),
    'invitations_open', (SELECT count(*) FROM tenants.invitations WHERE organization_id = o.id AND accepted_at IS NULL AND expires_at > now()),
    'api_keys', (SELECT count(*) FROM tenants.api_keys WHERE organization_id = o.id AND revoked_at IS NULL),
    'projects', (SELECT count(*) FROM projects.projects WHERE tenant_id = o.id),
    'connections', (SELECT count(*) FROM cloudaccounts.connections WHERE tenant_id = o.id),
    'connections_failing', (SELECT count(*) FROM cloudaccounts.connections WHERE tenant_id = o.id AND sync_status <> 'ok'),
    'reports', (SELECT count(*) FROM reports.reports WHERE tenant_id = o.id),
    'destinations', (SELECT count(*) FROM notifications.destinations WHERE tenant_id = o.id),
    'gate_checks', (SELECT count(*) FROM carbon.gate_evaluations WHERE tenant_id = o.id),
    'last_activity', (SELECT max(created_at) FROM audit.audit_logs WHERE tenant_id = o.id),
    'erasure_requested', (SELECT jsonb_build_object('at', requested_at, 'execute_after', execute_after, 'by', requested_by)
                           FROM tenants.deletion_requests WHERE tenant_id = o.id))
  FROM tenants.organizations o WHERE o.id = p_tenant
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION platform.operator_org_timeline(p_tenant uuid, p_limit int)
RETURNS TABLE (at timestamptz, actor text, action text, target text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT a.created_at, a.actor_user_id, a.action, a.target FROM audit.audit_logs a WHERE a.tenant_id = p_tenant
  ORDER BY a.created_at DESC, a.id DESC LIMIT least(greatest(p_limit, 1), 200)
$$;
-- +goose StatementEnd

-- An erasure the platform schedules for an organization (a legal request, an abandoned account). Same grace period and
-- the same erasure as the owner's own, so it can be cancelled until it runs.
-- +goose StatementBegin
CREATE FUNCTION platform.operator_request_erasure(p_actor text, p_tenant uuid, p_grace_days int, p_reason text) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  IF char_length(btrim(p_reason)) < 5 OR char_length(p_reason) > 300 THEN RAISE EXCEPTION 'a reason of 5-300 characters is required' USING ERRCODE = 'P0001'; END IF;
  IF NOT EXISTS (SELECT 1 FROM tenants.organizations WHERE id = p_tenant) THEN RETURN false; END IF;
  INSERT INTO tenants.deletion_requests (tenant_id, requested_by, execute_after)
    VALUES (p_tenant, 'operator:' || p_actor, now() + make_interval(days => least(greatest(p_grace_days, 1), 90)))
    ON CONFLICT (tenant_id) DO NOTHING;
  INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target, metadata)
    VALUES (p_tenant, 'operator:' || p_actor, 'operator.erasure_requested', 'organization:' || p_tenant,
            jsonb_build_object('reason', left(btrim(p_reason), 300), 'grace_days', least(greatest(p_grace_days, 1), 90)));
  RETURN true;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION platform.operator_cancel_erasure(p_actor text, p_tenant uuid, p_reason text) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  DELETE FROM tenants.deletion_requests WHERE tenant_id = p_tenant;
  IF NOT FOUND THEN RETURN false; END IF;
  INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target, metadata)
    VALUES (p_tenant, 'operator:' || p_actor, 'operator.erasure_cancelled', 'organization:' || p_tenant, jsonb_build_object('reason', left(btrim(coalesce(p_reason, '')), 300)));
  RETURN true;
END $$;
-- +goose StatementEnd

ALTER FUNCTION platform.operator_announce(text, text, text, text, text, text, text[], boolean) OWNER TO greenops_fanout;
ALTER FUNCTION platform.announcement_contacts(text, text[]) OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_announcements(int) OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_withdraw_announcement(text, uuid) OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_metrics(int) OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_org_overview(uuid) OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_org_timeline(uuid, int) OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_request_erasure(text, uuid, int, text) OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_cancel_erasure(text, uuid, text) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.operator_announce(text, text, text, text, text, text, text[], boolean), platform.announcement_contacts(text, text[]),
  platform.operator_announcements(int), platform.operator_withdraw_announcement(text, uuid), platform.operator_metrics(int),
  platform.operator_org_overview(uuid), platform.operator_org_timeline(uuid, int), platform.operator_request_erasure(text, uuid, int, text),
  platform.operator_cancel_erasure(text, uuid, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.operator_announce(text, text, text, text, text, text, text[], boolean), platform.announcement_contacts(text, text[]),
  platform.operator_announcements(int), platform.operator_withdraw_announcement(text, uuid), platform.operator_metrics(int),
  platform.operator_org_overview(uuid), platform.operator_org_timeline(uuid, int), platform.operator_request_erasure(text, uuid, int, text),
  platform.operator_cancel_erasure(text, uuid, text) TO greenops_api;

-- +goose Down
DROP FUNCTION platform.operator_cancel_erasure(text, uuid, text);
DROP FUNCTION platform.operator_request_erasure(text, uuid, int, text);
DROP FUNCTION platform.operator_org_timeline(uuid, int);
DROP FUNCTION platform.operator_org_overview(uuid);
DROP FUNCTION platform.operator_metrics(int);
DROP FUNCTION platform.operator_withdraw_announcement(text, uuid);
DROP FUNCTION platform.operator_announcements(int);
DROP FUNCTION platform.announcement_contacts(text, text[]);
DROP FUNCTION platform.operator_announce(text, text, text, text, text, text, text[], boolean);
DROP TABLE platform.announcements;
