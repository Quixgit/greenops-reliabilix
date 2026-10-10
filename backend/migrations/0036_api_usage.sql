-- +goose Up
-- Metering of calls to paid external APIs (cost controls): requests and failures per provider and day. Counts only.
CREATE TABLE platform.api_usage (
    provider text NOT NULL CHECK (provider ~ '^[a-z][a-z0-9_]{1,30}$'),
    day      date NOT NULL,
    requests int  NOT NULL DEFAULT 0 CHECK (requests >= 0),
    errors   int  NOT NULL DEFAULT 0 CHECK (errors >= 0),
    PRIMARY KEY (provider, day)
);
GRANT SELECT ON platform.api_usage TO greenops_api, greenops_worker;

-- +goose StatementBegin
CREATE FUNCTION platform.record_api_usage(p_provider text, p_requests int, p_errors int) RETURNS void
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  INSERT INTO platform.api_usage (provider, day, requests, errors)
  VALUES (p_provider, (now() AT TIME ZONE 'utc')::date, greatest(p_requests, 0), greatest(p_errors, 0))
  ON CONFLICT (provider, day) DO UPDATE SET requests = platform.api_usage.requests + EXCLUDED.requests, errors = platform.api_usage.errors + EXCLUDED.errors
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION platform.operator_api_usage(p_days int) RETURNS TABLE (provider text, day date, requests int, errors int)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT u.provider, u.day, u.requests, u.errors FROM platform.api_usage u
  WHERE u.day > (now() AT TIME ZONE 'utc')::date - least(greatest(p_days, 1), 366) ORDER BY u.day DESC, u.provider
$$;
-- +goose StatementEnd
GRANT INSERT, UPDATE, SELECT ON platform.api_usage TO greenops_fanout;
ALTER FUNCTION platform.record_api_usage(text, int, int) OWNER TO greenops_fanout;
ALTER FUNCTION platform.operator_api_usage(int) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.record_api_usage(text, int, int), platform.operator_api_usage(int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.record_api_usage(text, int, int) TO greenops_api, greenops_worker;
GRANT EXECUTE ON FUNCTION platform.operator_api_usage(int) TO greenops_api;

-- +goose Down
DROP FUNCTION platform.operator_api_usage(int), platform.record_api_usage(text, int, int);
DROP TABLE platform.api_usage;
