-- +goose Up
-- Operator console: the operators' own access log and a per-provider connection summary for the data source registry.
-- Same rules as 0014: narrow SECURITY DEFINER functions owned by greenops_fanout, metadata only.
-- +goose StatementBegin
CREATE FUNCTION platform.operator_audit(p_limit int)
RETURNS TABLE (id bigint, at timestamptz, operator_sub text, operator_email text, role text, action text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT l.id, l.at, l.operator_sub, coalesce(o.email, ''), coalesce(o.role, ''), l.action
  FROM platform.operator_access_log l LEFT JOIN platform.operators o ON o.auth_sub = l.operator_sub
  ORDER BY l.id DESC LIMIT least(greatest(p_limit, 1), 500)
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION cloudaccounts.operator_provider_usage()
RETURNS TABLE (provider text, connections int, organizations int, in_error int, last_sync_at timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT c.provider, count(*)::int, count(DISTINCT c.tenant_id)::int, (count(*) FILTER (WHERE c.sync_status = 'error'))::int, max(c.last_sync_at)
  FROM cloudaccounts.connections c GROUP BY c.provider ORDER BY c.provider
$$;
-- +goose StatementEnd

GRANT SELECT ON platform.operator_access_log TO greenops_fanout;
ALTER FUNCTION platform.operator_audit(int)                  OWNER TO greenops_fanout;
ALTER FUNCTION cloudaccounts.operator_provider_usage()       OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.operator_audit(int), cloudaccounts.operator_provider_usage() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.operator_audit(int), cloudaccounts.operator_provider_usage() TO greenops_api;

-- +goose Down
DROP FUNCTION cloudaccounts.operator_provider_usage(), platform.operator_audit(int);
REVOKE SELECT ON platform.operator_access_log FROM greenops_fanout;
