-- +goose Up
-- Operator console: the list of operators with their recent activity (read-only; who is an operator stays a
-- database fact only the migration owner can change, ADR-0020).
-- +goose StatementBegin
CREATE FUNCTION platform.operator_list()
RETURNS TABLE (auth_sub text, email text, role text, created_at timestamptz, last_seen_at timestamptz, actions_30d int)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT o.auth_sub, o.email, o.role, o.created_at,
         (SELECT max(l.at) FROM platform.operator_access_log l WHERE l.operator_sub = o.auth_sub),
         (SELECT count(*)::int FROM platform.operator_access_log l WHERE l.operator_sub = o.auth_sub AND l.at > now() - interval '30 days')
  FROM platform.operators o ORDER BY o.created_at
$$;
-- +goose StatementEnd
ALTER FUNCTION platform.operator_list() OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.operator_list() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.operator_list() TO greenops_api;

-- +goose Down
DROP FUNCTION platform.operator_list();
