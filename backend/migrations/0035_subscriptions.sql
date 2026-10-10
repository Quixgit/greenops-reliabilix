-- +goose Up
-- Subscriptions (ADR-0022, ADR-0038): the state of an organization's plan over time. The plan's limits and modules are
-- written through the settings engine (ADR-0033); this table adds the status and the dates, and the scheduler returns an
-- organization to the Free plan when its period has ended. No payment data: a provider, when there is one, keeps it.
CREATE TABLE tenants.subscriptions (
    organization_id uuid PRIMARY KEY REFERENCES tenants.organizations(id),
    plan            text NOT NULL CHECK (plan ~ '^[a-z][a-z0-9_-]{1,31}$'),
    status          text NOT NULL CHECK (status IN ('active','trialing','past_due','canceled')),
    source          text NOT NULL DEFAULT 'manual' CHECK (source ~ '^[a-z][a-z0-9_]{1,20}$'), -- manual, or the payment provider's id
    trial_ends_at   timestamptz,
    period_end      timestamptz,
    updated_by      text NOT NULL,
    updated_at      timestamptz NOT NULL DEFAULT now()
);
SELECT platform.enable_tenant_rls('tenants.subscriptions', 'organization_id');
-- Tenant-readable; changed only through the operator function and the expiry function below.
GRANT SELECT ON tenants.subscriptions TO greenops_api, greenops_worker;
GRANT SELECT, INSERT, UPDATE, DELETE ON tenants.subscriptions TO greenops_fanout;

-- Operator: assign a plan with its status and dates. Supersedes platform.operator_set_plan for the console.
-- +goose StatementBegin
CREATE FUNCTION platform.operator_set_subscription(p_actor text, p_tenant uuid, p_plan text, p_status text,
    p_trial_end timestamptz, p_period_end timestamptz, p_reason text) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE old text;
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  IF p_plan !~ '^[a-z][a-z0-9_-]{1,31}$' THEN RAISE EXCEPTION 'invalid plan' USING ERRCODE = 'P0001'; END IF;
  IF p_status NOT IN ('active','trialing','past_due','canceled') THEN RAISE EXCEPTION 'invalid status' USING ERRCODE = 'P0001'; END IF;
  IF char_length(btrim(p_reason)) < 3 OR char_length(p_reason) > 300 THEN RAISE EXCEPTION 'a reason of 3-300 characters is required' USING ERRCODE = 'P0001'; END IF;
  IF p_status = 'trialing' AND p_trial_end IS NULL THEN RAISE EXCEPTION 'a trial needs an end date' USING ERRCODE = 'P0001'; END IF;
  SELECT plan INTO old FROM tenants.organizations WHERE id = p_tenant;
  IF NOT FOUND THEN RETURN false; END IF;
  UPDATE tenants.organizations SET plan = p_plan WHERE id = p_tenant;
  INSERT INTO tenants.subscriptions (organization_id, plan, status, trial_ends_at, period_end, updated_by)
    VALUES (p_tenant, p_plan, p_status, p_trial_end, CASE WHEN p_status = 'trialing' THEN p_trial_end ELSE p_period_end END, p_actor)
    ON CONFLICT (organization_id) DO UPDATE SET plan = EXCLUDED.plan, status = EXCLUDED.status, trial_ends_at = EXCLUDED.trial_ends_at,
      period_end = EXCLUDED.period_end, updated_by = EXCLUDED.updated_by, updated_at = now();
  INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target, metadata)
    VALUES (p_tenant, 'operator:' || p_actor, 'operator.plan_changed', 'plan:' || p_plan,
            jsonb_build_object('from', coalesce(old, ''), 'to', p_plan, 'status', p_status,
                               'period_end', CASE WHEN p_status = 'trialing' THEN p_trial_end ELSE p_period_end END,
                               'reason', left(btrim(p_reason), 300)));
  RETURN true;
END $$;
-- +goose StatementEnd

-- Writes a plan's preset as locked organization values with history, and the subscription row: the one place that does
-- it for the expiry job and the payment providers' webhooks (operators go through the settings engine instead).
-- +goose StatementBegin
CREATE FUNCTION platform.apply_plan(p_tenant uuid, p_plan text, p_status text, p_period_end timestamptz, p_source text,
    p_values jsonb, p_actor text, p_note jsonb) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE kv record; old jsonb; oldplan text;
BEGIN
  SELECT plan INTO oldplan FROM tenants.organizations WHERE id = p_tenant;
  FOR kv IN SELECT key, value FROM jsonb_each(p_values) LOOP
    SELECT value INTO old FROM settings.tenant_values WHERE tenant_id = p_tenant AND key = kv.key;
    INSERT INTO settings.tenant_values (tenant_id, key, value, source, updated_by)
      VALUES (p_tenant, kv.key, kv.value, 'operator', p_actor)
      ON CONFLICT (tenant_id, key) DO UPDATE SET value = EXCLUDED.value, source = 'operator', updated_by = p_actor, updated_at = now();
    IF old IS DISTINCT FROM kv.value THEN
      INSERT INTO settings.history (tenant_id, key, old_value, new_value, actor, via) VALUES (p_tenant, kv.key, old, kv.value, p_actor, 'operator');
    END IF;
  END LOOP;
  UPDATE tenants.organizations SET plan = p_plan WHERE id = p_tenant;
  INSERT INTO tenants.subscriptions (organization_id, plan, status, source, period_end, updated_by)
    VALUES (p_tenant, p_plan, p_status, p_source, p_period_end, p_actor)
    ON CONFLICT (organization_id) DO UPDATE SET plan = EXCLUDED.plan, status = EXCLUDED.status, source = EXCLUDED.source,
      trial_ends_at = NULL, period_end = EXCLUDED.period_end, updated_by = EXCLUDED.updated_by, updated_at = now();
  INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target, metadata)
    VALUES (p_tenant, p_actor, 'subscription.changed', 'plan:' || p_plan,
            coalesce(p_note, '{}'::jsonb) || jsonb_build_object('from', coalesce(oldplan, ''), 'to', p_plan, 'status', p_status));
END $$;
-- +goose StatementEnd

-- Payment provider events already handled (a provider repeats a delivery until it is acknowledged).
CREATE TABLE platform.billing_events (
    provider        text NOT NULL,
    event_id        text NOT NULL,
    received_at     timestamptz NOT NULL DEFAULT now(),
    kind            text NOT NULL DEFAULT 'applied' CHECK (kind IN ('applied','requested')),
    organization_id uuid,
    plan            text,
    detail          text NOT NULL DEFAULT '' CHECK (char_length(detail) <= 200),
    PRIMARY KEY (provider, event_id)
);
GRANT SELECT, INSERT ON platform.billing_events TO greenops_fanout;

-- A payment provider confirmed a payment or a subscription change for an organization. Idempotent per event id.
-- Returns false when the event was already applied or the organization does not exist.
-- +goose StatementBegin
CREATE FUNCTION platform.billing_apply(p_provider text, p_event_id text, p_tenant uuid, p_plan text, p_status text,
    p_period_end timestamptz, p_values jsonb, p_ref text) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
BEGIN
  IF p_provider !~ '^[a-z][a-z0-9_]{1,20}$' OR p_plan !~ '^[a-z][a-z0-9_-]{1,31}$' OR p_status NOT IN ('active','trialing','past_due','canceled') THEN
    RAISE EXCEPTION 'invalid billing event' USING ERRCODE = 'P0001';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM tenants.organizations WHERE id = p_tenant) THEN RETURN false; END IF;
  INSERT INTO platform.billing_events (provider, event_id, organization_id, plan, detail)
    VALUES (p_provider, p_event_id, p_tenant, p_plan, left(p_status || ' ' || coalesce(p_ref, ''), 200)) ON CONFLICT DO NOTHING;
  IF NOT FOUND THEN RETURN false; END IF;
  PERFORM platform.apply_plan(p_tenant, p_plan, p_status, p_period_end, p_provider, p_values, 'billing:' || p_provider, jsonb_build_object('reference', left(p_ref, 120)));
  RETURN true;
END $$;
-- +goose StatementEnd

-- A customer asked for a plan by a provider that has no hosted page (invoice): recorded for the operator.
-- +goose StatementBegin
CREATE FUNCTION platform.billing_request(p_tenant uuid, p_provider text, p_plan text, p_email text) RETURNS void
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  INSERT INTO platform.billing_events (provider, event_id, kind, organization_id, plan, detail)
  VALUES (left(p_provider, 20), 'req:' || gen_random_uuid(), 'requested', p_tenant, left(p_plan, 32), left(p_email, 200))
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION platform.operator_billing_events(p_limit int)
RETURNS TABLE (at timestamptz, provider text, kind text, organization_id uuid, organization_name text, plan text, detail text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT e.received_at, e.provider, e.kind, e.organization_id, coalesce(o.name, ''), coalesce(e.plan, ''), e.detail
  FROM platform.billing_events e LEFT JOIN tenants.organizations o ON o.id = e.organization_id
  ORDER BY e.received_at DESC LIMIT least(greatest(p_limit, 1), 200)
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION platform.operator_subscription(p_tenant uuid)
RETURNS TABLE (plan text, status text, source text, trial_ends_at timestamptz, period_end timestamptz, updated_by text, updated_at timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT s.plan, s.status, s.source, s.trial_ends_at, s.period_end, s.updated_by, s.updated_at FROM tenants.subscriptions s WHERE s.organization_id = p_tenant
$$;
-- +goose StatementEnd

-- Scheduler: subscriptions whose period has ended return to the Free plan. The free preset (limits and modules) comes
-- from the application's plan catalog as jsonb, so the numbers have one definition. Each change is in the settings
-- history (actor system:subscription) and in the organization's audit log. Returns how many organizations changed.
-- +goose StatementBegin
CREATE FUNCTION platform.expire_subscriptions(p_free_values jsonb) RETURNS int
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE r record; n int := 0;
BEGIN
  FOR r IN SELECT organization_id, plan, status FROM tenants.subscriptions
           WHERE status IN ('active','trialing') AND period_end IS NOT NULL AND period_end < now() AND plan <> 'free' FOR UPDATE
  LOOP
    PERFORM platform.apply_plan(r.organization_id, 'free', 'canceled', NULL, 'manual', p_free_values, 'system:subscription',
                                jsonb_build_object('expired', r.plan, 'was', r.status));
    n := n + 1;
  END LOOP;
  RETURN n;
END $$;
-- +goose StatementEnd

GRANT USAGE ON SCHEMA settings TO greenops_fanout;
ALTER FUNCTION platform.operator_set_subscription(text, uuid, text, text, timestamptz, timestamptz, text) OWNER TO greenops_fanout;
ALTER FUNCTION platform.expire_subscriptions(jsonb) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.operator_set_subscription(text, uuid, text, text, timestamptz, timestamptz, text), platform.expire_subscriptions(jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.operator_set_subscription(text, uuid, text, text, timestamptz, timestamptz, text) TO greenops_api;
GRANT EXECUTE ON FUNCTION platform.expire_subscriptions(jsonb) TO greenops_worker;
ALTER FUNCTION platform.apply_plan(uuid, text, text, timestamptz, text, jsonb, text, jsonb) OWNER TO greenops_fanout;
ALTER FUNCTION platform.billing_apply(text, text, uuid, text, text, timestamptz, jsonb, text) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.apply_plan(uuid, text, text, timestamptz, text, jsonb, text, jsonb), platform.billing_apply(text, text, uuid, text, text, timestamptz, jsonb, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.billing_apply(text, text, uuid, text, text, timestamptz, jsonb, text) TO greenops_api;
ALTER FUNCTION platform.billing_request(uuid, text, text, text) OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_billing_events(int) OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_subscription(uuid) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.billing_request(uuid, text, text, text), platform.operator_billing_events(int), platform.operator_subscription(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.billing_request(uuid, text, text, text), platform.operator_billing_events(int), platform.operator_subscription(uuid) TO greenops_api;

-- +goose Down
DROP FUNCTION platform.operator_subscription(uuid), platform.operator_billing_events(int), platform.billing_request(uuid, text, text, text);
DROP FUNCTION platform.billing_apply(text, text, uuid, text, text, timestamptz, jsonb, text);
DROP TABLE platform.billing_events;
DROP FUNCTION platform.expire_subscriptions(jsonb), platform.apply_plan(uuid, text, text, timestamptz, text, jsonb, text, jsonb), platform.operator_set_subscription(text, uuid, text, text, timestamptz, timestamptz, text);
DROP TABLE tenants.subscriptions;
