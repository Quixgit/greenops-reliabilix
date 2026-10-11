-- +goose Up
-- The bell: what the platform tells the people of an organization inside the product. Every published event is kept here
-- once (a repeated key is dropped), and what a person sees is decided at read time from the organization's settings, so
-- changing "what to show" applies to what is already there. The per-person read mark is one timestamp.
CREATE TABLE notifications.inbox (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid NOT NULL REFERENCES tenants.organizations(id),
    type       text NOT NULL CHECK (char_length(type) BETWEEN 3 AND 80),
    severity   text NOT NULL CHECK (severity IN ('info','warning','critical')),
    title      text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    body       text NOT NULL DEFAULT '' CHECK (char_length(body) <= 2000),
    path       text NOT NULL DEFAULT '' CHECK (char_length(path) <= 200),
    dedupe_key text CHECK (dedupe_key IS NULL OR char_length(dedupe_key) <= 200),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX inbox_dedupe ON notifications.inbox (tenant_id, dedupe_key) WHERE dedupe_key IS NOT NULL;
CREATE INDEX inbox_recent ON notifications.inbox (tenant_id, created_at DESC);
SELECT platform.enable_tenant_rls('notifications.inbox', 'tenant_id');

CREATE TABLE notifications.inbox_state (
    tenant_id  uuid NOT NULL REFERENCES tenants.organizations(id),
    user_id    text NOT NULL CHECK (char_length(user_id) BETWEEN 1 AND 200),
    read_until timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, user_id)
);
SELECT platform.enable_tenant_rls('notifications.inbox_state', 'tenant_id');

-- Producers run in the API and in the worker; old items are dropped by the one that inserts.
GRANT SELECT, INSERT, DELETE ON notifications.inbox TO greenops_api, greenops_worker;
GRANT SELECT, INSERT, UPDATE ON notifications.inbox_state TO greenops_api;
GRANT SELECT, DELETE ON notifications.inbox, notifications.inbox_state TO greenops_fanout; -- organization erasure (ADR-0027)

-- Paid subscriptions whose period ends within p_within_days days (or has just ended), with the addresses of the people
-- who should hear about the bill: owners, admins and billing users. Called by the reminder job only.
-- +goose StatementBegin
CREATE FUNCTION platform.billing_reminder_candidates(p_within_days int)
RETURNS TABLE (tenant_id uuid, plan text, status text, period_end timestamptz, emails text[])
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT s.organization_id, s.plan, s.status, coalesce(s.period_end, s.trial_ends_at),
         coalesce((SELECT array_agg(DISTINCT u.email) FROM tenants.memberships m JOIN tenants.users u ON u.id = m.user_id
                    WHERE m.organization_id = s.organization_id AND m.role IN ('owner','admin','billing') AND u.email <> ''), '{}')
  FROM tenants.subscriptions s
  WHERE s.plan <> 'free' AND s.status IN ('active','trialing','past_due')
    AND coalesce(s.period_end, s.trial_ends_at) IS NOT NULL
    AND coalesce(s.period_end, s.trial_ends_at) < now() + make_interval(days => least(greatest(p_within_days, 1), 60))
    AND coalesce(s.period_end, s.trial_ends_at) > now() - interval '2 days'
$$;
-- +goose StatementEnd
-- The people who hear about one organization's payments.
-- +goose StatementBegin
CREATE FUNCTION platform.tenant_billing_contacts(p_tenant uuid) RETURNS text[]
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT coalesce(array_agg(DISTINCT u.email), '{}') FROM tenants.memberships m JOIN tenants.users u ON u.id = m.user_id
   WHERE m.organization_id = p_tenant AND m.role IN ('owner','admin','billing') AND u.email <> ''
$$;
-- +goose StatementEnd
ALTER FUNCTION platform.billing_reminder_candidates(int) OWNER TO greenops_fanout;
ALTER FUNCTION platform.tenant_billing_contacts(uuid) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.billing_reminder_candidates(int), platform.tenant_billing_contacts(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.billing_reminder_candidates(int), platform.tenant_billing_contacts(uuid) TO greenops_worker, greenops_api;

-- +goose Down
DROP FUNCTION platform.tenant_billing_contacts(uuid);
DROP FUNCTION platform.billing_reminder_candidates(int);
DROP TABLE notifications.inbox_state, notifications.inbox;
