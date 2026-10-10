-- +goose Up
-- Operator console: what each organization uses (counts of records and runs in a window). Counts only: no content, no
-- amounts. The console turns them into an estimated cost of serving the organization with unit costs from settings.
-- +goose StatementBegin
CREATE FUNCTION platform.operator_org_usage(p_days int)
RETURNS TABLE (tenant_id uuid, name text, plan text, usage_rows bigint, calculation_rows bigint, sync_runs bigint,
               deliveries bigint, reports bigint, k8s_samples bigint, members int, connections int)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  WITH w AS (SELECT now() - make_interval(days => least(greatest(p_days, 1), 366)) AS since)
  SELECT o.id, o.name, o.plan,
    (SELECT count(*) FROM usage.usage_records x, w WHERE x.tenant_id = o.id AND x.recorded_at >= w.since),
    (SELECT count(*) FROM carbon.calculations x, w WHERE x.tenant_id = o.id AND x.period_start >= w.since),
    (SELECT count(*) FROM cloudaccounts.sync_runs x, w WHERE x.tenant_id = o.id AND x.started_at >= w.since),
    (SELECT count(*) FROM notifications.deliveries x, w WHERE x.tenant_id = o.id AND x.created_at >= w.since),
    (SELECT count(*) FROM reports.reports x, w WHERE x.tenant_id = o.id AND x.created_at >= w.since),
    (SELECT count(*) FROM kubernetes.workload_energy x, w WHERE x.tenant_id = o.id AND x.period_start >= w.since),
    (SELECT count(*)::int FROM tenants.memberships m WHERE m.organization_id = o.id),
    (SELECT count(*)::int FROM cloudaccounts.connections c WHERE c.tenant_id = o.id)
  FROM tenants.organizations o ORDER BY o.created_at DESC LIMIT 500
$$;
-- +goose StatementEnd
GRANT USAGE ON SCHEMA kubernetes TO greenops_fanout;
GRANT SELECT ON kubernetes.workload_energy TO greenops_fanout;
ALTER FUNCTION platform.operator_org_usage(int) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.operator_org_usage(int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.operator_org_usage(int) TO greenops_api;

-- +goose Down
DROP FUNCTION platform.operator_org_usage(int);
