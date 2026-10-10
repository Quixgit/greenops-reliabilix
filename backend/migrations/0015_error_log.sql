-- +goose Up
-- Error log of the platform's own services (ADR-0021): api, worker and the integrations they call. Errors are
-- grouped by a fingerprint (service + component + message with ids and numbers masked), so a failing job that
-- retries a thousand times is one row with a counter, not a thousand rows. Messages are sanitized before they get
-- here (tokens, URLs with credentials, e-mail addresses removed). The table holds no tenant data: at most the id of
-- the tenant the last occurrence belonged to, which lets an operator find the organization to talk to.
CREATE TABLE platform.error_groups (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    fingerprint     text NOT NULL UNIQUE CHECK (char_length(fingerprint) = 64),
    service         text NOT NULL CHECK (service ~ '^[a-z][a-z0-9_-]{1,31}$'),
    component       text NOT NULL CHECK (char_length(component) BETWEEN 1 AND 120),
    severity        text NOT NULL CHECK (severity IN ('warning','error','critical')),
    message         text NOT NULL CHECK (char_length(message) <= 500),
    status          text NOT NULL DEFAULT 'open' CHECK (status IN ('open','resolved','ignored')),
    occurrences     bigint NOT NULL DEFAULT 1,
    first_seen      timestamptz NOT NULL DEFAULT now(),
    last_seen       timestamptz NOT NULL DEFAULT now(),
    last_tenant_id  uuid REFERENCES tenants.organizations(id) ON DELETE SET NULL,
    last_request_id text CHECK (last_request_id IS NULL OR char_length(last_request_id) <= 100),
    handled_by      text,
    handled_at      timestamptz
);
CREATE INDEX ON platform.error_groups (status, last_seen DESC);

-- Called by the api and worker processes. A resolved error that happens again is reopened (an ignored one stays
-- ignored: someone decided it is noise). p_count lets a process report several identical occurrences at once.
-- +goose StatementBegin
CREATE FUNCTION platform.record_error(p_fingerprint text, p_service text, p_component text, p_severity text, p_message text,
                                      p_count int, p_tenant uuid, p_request_id text) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE tenant uuid;
BEGIN
  SELECT id INTO tenant FROM tenants.organizations WHERE id = p_tenant;
  INSERT INTO platform.error_groups (fingerprint, service, component, severity, message, occurrences, last_tenant_id, last_request_id)
  VALUES (p_fingerprint, p_service, left(p_component, 120), p_severity, left(p_message, 500), greatest(p_count, 1), tenant, left(p_request_id, 100))
  ON CONFLICT (fingerprint) DO UPDATE SET
    occurrences = platform.error_groups.occurrences + greatest(p_count, 1),
    last_seen = now(),
    severity = CASE WHEN EXCLUDED.severity = 'critical' OR platform.error_groups.severity = 'critical' THEN 'critical'
                    WHEN EXCLUDED.severity = 'error' OR platform.error_groups.severity = 'error' THEN 'error' ELSE 'warning' END,
    last_tenant_id = COALESCE(EXCLUDED.last_tenant_id, platform.error_groups.last_tenant_id),
    last_request_id = COALESCE(EXCLUDED.last_request_id, platform.error_groups.last_request_id),
    status = CASE WHEN platform.error_groups.status = 'resolved' THEN 'open' ELSE platform.error_groups.status END,
    handled_by = CASE WHEN platform.error_groups.status = 'resolved' THEN NULL ELSE platform.error_groups.handled_by END,
    handled_at = CASE WHEN platform.error_groups.status = 'resolved' THEN NULL ELSE platform.error_groups.handled_at END;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION platform.operator_errors(p_status text, p_service text, p_limit int)
RETURNS TABLE (id uuid, service text, component text, severity text, message text, status text, occurrences bigint,
               first_seen timestamptz, last_seen timestamptz, tenant_id uuid, tenant_name text, last_request_id text,
               handled_by text, handled_at timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT e.id, e.service, e.component, e.severity, e.message, e.status, e.occurrences, e.first_seen, e.last_seen,
         e.last_tenant_id, o.name, e.last_request_id, e.handled_by, e.handled_at
  FROM platform.error_groups e LEFT JOIN tenants.organizations o ON o.id = e.last_tenant_id
  WHERE (p_status IS NULL OR e.status = p_status) AND (p_service IS NULL OR e.service = p_service)
  ORDER BY (e.status = 'open') DESC, e.last_seen DESC LIMIT least(greatest(p_limit, 1), 200)
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION platform.operator_error_set_status(p_actor text, p_id uuid, p_status text) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE n int;
BEGIN
  IF platform.operator_role(p_actor) IS DISTINCT FROM 'admin' THEN RAISE EXCEPTION 'operator admin required' USING ERRCODE = 'P0001'; END IF;
  IF p_status NOT IN ('open','resolved','ignored') THEN RAISE EXCEPTION 'invalid status' USING ERRCODE = 'P0001'; END IF;
  UPDATE platform.error_groups SET status = p_status,
         handled_by = CASE WHEN p_status = 'open' THEN NULL ELSE p_actor END,
         handled_at = CASE WHEN p_status = 'open' THEN NULL ELSE now() END
   WHERE id = p_id;
  GET DIAGNOSTICS n = ROW_COUNT;
  RETURN n > 0;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION platform.purge_old_errors(p_retain interval) RETURNS bigint
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE n bigint;
BEGIN
  IF p_retain < interval '7 days' THEN RAISE EXCEPTION 'retention below 7 days is not allowed'; END IF;
  DELETE FROM platform.error_groups WHERE status <> 'open' AND last_seen < now() - p_retain;
  GET DIAGNOSTICS n = ROW_COUNT;
  RETURN n;
END $$;
-- +goose StatementEnd

GRANT SELECT, INSERT, UPDATE, DELETE ON platform.error_groups TO greenops_fanout;
ALTER FUNCTION platform.record_error(text, text, text, text, text, int, uuid, text) OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_errors(text, text, int)                            OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_error_set_status(text, uuid, text)                 OWNER TO greenops_fanout;
ALTER FUNCTION platform.purge_old_errors(interval)                                  OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.record_error(text, text, text, text, text, int, uuid, text), platform.operator_errors(text, text, int),
  platform.operator_error_set_status(text, uuid, text), platform.purge_old_errors(interval) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.record_error(text, text, text, text, text, int, uuid, text) TO greenops_api, greenops_worker;
GRANT EXECUTE ON FUNCTION platform.operator_errors(text, text, int), platform.operator_error_set_status(text, uuid, text) TO greenops_api;
GRANT EXECUTE ON FUNCTION platform.purge_old_errors(interval) TO greenops_worker;

-- +goose Down
DROP FUNCTION platform.purge_old_errors(interval), platform.operator_error_set_status(text, uuid, text),
  platform.operator_errors(text, text, int), platform.record_error(text, text, text, text, text, int, uuid, text);
DROP TABLE platform.error_groups;
