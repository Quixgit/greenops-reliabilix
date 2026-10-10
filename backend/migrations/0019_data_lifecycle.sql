-- +goose Up
-- Data lifecycle (ADR-0027): an organization owner can have the organization erased after a grace period, and the
-- platform drops usage and carbon partitions past the retention window.

-- A request to erase an organization. One row per organization; cancelling deletes the row. The row is erased with
-- the organization, so the proof that it happened is the minimal, personal-data-free platform.erasure_log.
CREATE TABLE tenants.deletion_requests (
    tenant_id     uuid PRIMARY KEY REFERENCES tenants.organizations(id),
    requested_by  text NOT NULL,
    requested_at  timestamptz NOT NULL DEFAULT now(),
    execute_after timestamptz NOT NULL CHECK (execute_after > requested_at)
);
SELECT platform.enable_tenant_rls('tenants.deletion_requests', 'tenant_id');
GRANT SELECT, INSERT, DELETE ON tenants.deletion_requests TO greenops_api;

-- What remains after an erasure: that it happened, when, and how many rows it removed. No names, no e-mail addresses.
CREATE TABLE platform.erasure_log (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id   uuid NOT NULL,
    erased_at   timestamptz NOT NULL DEFAULT now(),
    row_counts  jsonb NOT NULL
);

-- Organizations whose grace period is over (the worker's input).
-- +goose StatementBegin
CREATE FUNCTION platform.due_deletions() RETURNS TABLE (tenant_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT d.tenant_id FROM tenants.deletion_requests d WHERE d.execute_after <= now() ORDER BY d.execute_after
$$;
-- +goose StatementEnd

-- Erases one organization and everything that belongs to it, in one transaction. Tables are discovered, not listed:
-- every row-level-secured table that has a tenant_id or organization_id column is emptied for this organization,
-- retrying until foreign keys allow it. If anything is left the whole erasure is rolled back and the function raises,
-- so a half-erased organization cannot exist. Users that belonged only to this organization are removed too.
-- +goose StatementBegin
CREATE FUNCTION platform.erase_tenant(p_tenant uuid) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE
  r record; pass int; n bigint; remaining int := 0;
  counts jsonb := '{}'::jsonb; users uuid[];
BEGIN
  IF NOT EXISTS (SELECT 1 FROM tenants.organizations WHERE id = p_tenant) THEN
    RETURN NULL;
  END IF;
  SELECT array_agg(user_id) INTO users FROM tenants.memberships WHERE organization_id = p_tenant;

  FOR pass IN 1..8 LOOP
    remaining := 0;
    FOR r IN
      SELECT ns.nspname AS sch, c.relname AS tbl,
             (SELECT a.attname FROM pg_attribute a WHERE a.attrelid = c.oid AND a.attname IN ('tenant_id', 'organization_id')
               AND NOT a.attisdropped ORDER BY a.attname DESC LIMIT 1) AS col
      FROM pg_class c JOIN pg_namespace ns ON ns.oid = c.relnamespace
      WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition AND c.relrowsecurity
        AND ns.nspname NOT IN ('pg_catalog', 'information_schema', 'platform')
        AND (ns.nspname, c.relname) <> ('tenants', 'users')
        AND EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = c.oid AND a.attname IN ('tenant_id', 'organization_id') AND NOT a.attisdropped)
    LOOP
      BEGIN
        EXECUTE format('DELETE FROM %I.%I WHERE %I = $1', r.sch, r.tbl, r.col) USING p_tenant;
        GET DIAGNOSTICS n = ROW_COUNT;
        IF n > 0 THEN
          counts := jsonb_set(counts, ARRAY[r.sch || '.' || r.tbl], to_jsonb(coalesce((counts ->> (r.sch || '.' || r.tbl))::bigint, 0) + n));
        END IF;
      EXCEPTION WHEN foreign_key_violation THEN
        remaining := remaining + 1;
      END;
    END LOOP;
    EXIT WHEN remaining = 0;
  END LOOP;
  IF remaining > 0 THEN
    RAISE EXCEPTION 'erase_tenant: % tables could not be emptied (a foreign key still points at the organization)', remaining;
  END IF;

  DELETE FROM tenants.organizations WHERE id = p_tenant;
  IF users IS NOT NULL THEN
    DELETE FROM tenants.users u WHERE u.id = ANY (users)
      AND NOT EXISTS (SELECT 1 FROM tenants.memberships m WHERE m.user_id = u.id);
    GET DIAGNOSTICS n = ROW_COUNT;
    counts := jsonb_set(counts, ARRAY['tenants.users'], to_jsonb(n));
  END IF;
  INSERT INTO platform.erasure_log (tenant_id, row_counts) VALUES (p_tenant, counts);
  RETURN counts;
END $$;
-- +goose StatementEnd

-- Drops monthly partitions of the two managed tables that end on or before p_older_than. The default partition and
-- the current month are never touched. Returns the number of partitions dropped.
-- +goose StatementBegin
CREATE FUNCTION platform.drop_old_partitions(p_parent regclass, p_older_than date) RETURNS int
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE r record; dropped int := 0; part_end date;
BEGIN
  IF p_parent::text NOT IN ('usage.usage_records', 'carbon.calculations') THEN
    RAISE EXCEPTION 'drop_old_partitions: % is not a managed partitioned table', p_parent;
  END IF;
  FOR r IN
    SELECT c.oid::regclass AS part, c.relname,
           pg_get_expr(c.relpartbound, c.oid) AS bound
    FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
    WHERE i.inhparent = p_parent AND c.relname ~ '_[0-9]{4}_[0-9]{2}$'
  LOOP
    part_end := (regexp_match(r.bound, 'TO \(''([0-9]{4}-[0-9]{2}-[0-9]{2})'))[1]::date;
    IF part_end IS NOT NULL AND part_end <= p_older_than THEN
      EXECUTE format('DROP TABLE %s', r.part);
      dropped := dropped + 1;
    END IF;
  END LOOP;
  RETURN dropped;
END $$;
-- +goose StatementEnd

-- The erase function runs as greenops_fanout (BYPASSRLS), like the other cross-tenant functions; it names its
-- organization in every statement. Grants are explicit so that a new tenant table is a visible decision: the
-- integration test fails until it is covered here.
ALTER FUNCTION platform.due_deletions()   OWNER TO greenops_fanout;
ALTER FUNCTION platform.erase_tenant(uuid) OWNER TO greenops_fanout;
GRANT USAGE ON SCHEMA tenants, projects, cloudaccounts, usage, carbon, finops, recommendations, reports, automation,
  notifications, settings, audit, platform TO greenops_fanout;
GRANT SELECT, DELETE ON
  tenants.organizations, tenants.users, tenants.memberships, tenants.invitations, tenants.api_keys, tenants.deletion_requests,
  projects.projects, projects.functional_unit_counts, projects.policies,
  cloudaccounts.connections, cloudaccounts.sync_runs,
  usage.usage_records, carbon.calculations, finops.budgets,
  recommendations.recommendations, recommendations.impact_verifications,
  reports.reports, automation.jobs,
  notifications.destinations, notifications.deliveries,
  settings.tenant_values, settings.history, audit.audit_logs
  TO greenops_fanout;
GRANT INSERT ON platform.erasure_log TO greenops_fanout;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA platform TO greenops_fanout;
GRANT UPDATE ON platform.error_groups TO greenops_fanout; -- last_tenant_id is set to NULL by the foreign key
REVOKE ALL ON FUNCTION platform.due_deletions(), platform.erase_tenant(uuid), platform.drop_old_partitions(regclass, date) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.due_deletions(), platform.erase_tenant(uuid), platform.drop_old_partitions(regclass, date) TO greenops_worker;

-- +goose Down
DROP FUNCTION platform.drop_old_partitions(regclass, date);
DROP FUNCTION platform.erase_tenant(uuid);
DROP FUNCTION platform.due_deletions();
DROP TABLE platform.erasure_log;
DROP TABLE tenants.deletion_requests;
