-- +goose Up
-- Notifications: channel-neutral events are delivered to tenant-configured destinations (e-mail, webhook, Slack,
-- more later). notifications.deliveries is a transactional outbox: Publish writes the rows, the queue is only a
-- fast path, and a periodic sweep re-queues anything due, so a Redis outage or a crash loses nothing (ADR-0019).
CREATE SCHEMA notifications;

CREATE TABLE notifications.destinations (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid NOT NULL REFERENCES tenants.organizations(id),
    -- 'email' | 'webhook' | 'slack' today. Deliberately not a CHECK: a new channel is code, not a migration.
    kind         text NOT NULL CHECK (kind ~ '^[a-z][a-z0-9_]{1,31}$'),
    name         text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    -- The address (mailbox, endpoint URL) sealed with AES-256-GCM (platform/secretbox): never stored in clear.
    target_sealed text NOT NULL,
    -- Non-secret description for display and audit: a masked mailbox or just the host of a URL.
    target_hint  text NOT NULL CHECK (char_length(target_hint) <= 200),
    event_types  text[] NOT NULL CHECK (cardinality(event_types) BETWEEN 1 AND 50),
    min_severity text NOT NULL DEFAULT 'info' CHECK (min_severity IN ('info','warning','critical')),
    enabled      boolean NOT NULL DEFAULT true,
    created_by   text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON notifications.destinations (tenant_id, created_at);
SELECT platform.enable_tenant_rls('notifications.destinations', 'tenant_id');

CREATE TABLE notifications.deliveries (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid NOT NULL REFERENCES tenants.organizations(id),
    -- Kept (NULL) when a destination is deleted, so the history survives.
    destination_id  uuid REFERENCES notifications.destinations(id) ON DELETE SET NULL,
    kind            text NOT NULL,
    destination_name text NOT NULL,
    event_type      text NOT NULL,
    severity        text NOT NULL CHECK (severity IN ('info','warning','critical')),
    payload         jsonb NOT NULL,
    dedupe_key      text CHECK (dedupe_key IS NULL OR char_length(dedupe_key) <= 200),
    status          text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','failed','skipped')),
    attempts        int  NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    -- A worker holds a short lease while it sends, so a redelivered job or the sweep cannot send twice at once.
    lease_until     timestamptz,
    last_error      text CHECK (last_error IS NULL OR char_length(last_error) <= 500),
    created_at      timestamptz NOT NULL DEFAULT now(),
    sent_at         timestamptz
);
-- The same event (same key) reaches a destination at most once.
CREATE UNIQUE INDEX deliveries_dedupe ON notifications.deliveries (destination_id, dedupe_key)
    WHERE dedupe_key IS NOT NULL AND destination_id IS NOT NULL;
CREATE INDEX deliveries_due ON notifications.deliveries (next_attempt_at) WHERE status = 'pending';
CREATE INDEX ON notifications.deliveries (tenant_id, created_at DESC);
SELECT platform.enable_tenant_rls('notifications.deliveries', 'tenant_id');

-- Cross-tenant helpers for the scheduler (owner greenops_fanout, BYPASSRLS): they return ids only.
-- A fresh delivery gets a 30 s head start for the queue's fast path before the sweep considers it.
-- +goose StatementBegin
CREATE FUNCTION notifications.list_due_deliveries(p_limit int) RETURNS TABLE (tenant_id uuid, id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT d.tenant_id, d.id FROM notifications.deliveries d
  WHERE d.status = 'pending' AND d.next_attempt_at <= now() - interval '30 seconds'
    AND (d.lease_until IS NULL OR d.lease_until < now())
  ORDER BY d.next_attempt_at LIMIT least(greatest(p_limit, 1), 1000)
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION notifications.purge_old_deliveries(p_retain interval) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE n bigint;
BEGIN
  IF p_retain < interval '7 days' THEN RAISE EXCEPTION 'retention below 7 days is not allowed'; END IF;
  DELETE FROM notifications.deliveries WHERE status <> 'pending' AND created_at < now() - p_retain;
  GET DIAGNOSTICS n = ROW_COUNT;
  RETURN n;
END $$;
-- +goose StatementEnd

GRANT USAGE ON SCHEMA notifications TO greenops_fanout, greenops_api, greenops_worker;
GRANT SELECT ON notifications.deliveries TO greenops_fanout;
GRANT DELETE ON notifications.deliveries TO greenops_fanout;
ALTER FUNCTION notifications.list_due_deliveries(int)         OWNER TO greenops_fanout;
ALTER FUNCTION notifications.purge_old_deliveries(interval)   OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION notifications.list_due_deliveries(int), notifications.purge_old_deliveries(interval) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION notifications.list_due_deliveries(int), notifications.purge_old_deliveries(interval) TO greenops_worker;

-- api: people manage destinations, publish events and send tests. Delivery rows are never edited by the API.
GRANT SELECT, INSERT, UPDATE, DELETE ON notifications.destinations TO greenops_api;
GRANT SELECT, INSERT ON notifications.deliveries TO greenops_api;
-- worker: publishes events from jobs, sends them and records the outcome. It cannot change destinations.
GRANT SELECT ON notifications.destinations TO greenops_worker;
GRANT SELECT, INSERT, UPDATE ON notifications.deliveries TO greenops_worker;

-- +goose Down
DROP FUNCTION notifications.purge_old_deliveries(interval), notifications.list_due_deliveries(int);
DROP TABLE notifications.deliveries;
DROP TABLE notifications.destinations;
DROP SCHEMA notifications;
