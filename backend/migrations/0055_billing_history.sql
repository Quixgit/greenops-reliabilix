-- +goose Up
-- What a customer may see of its own payments: the provider events that changed its plan or asked for one, and the
-- checkouts it started. Read through a function that takes the organization from the transaction's own context, so one
-- organization can never ask for another's history.
-- +goose StatementBegin
CREATE FUNCTION platform.tenant_billing_history(p_limit int)
RETURNS TABLE (at timestamptz, provider text, kind text, plan text, detail text)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE oid uuid := NULLIF(current_setting('app.tenant_id', true), '')::uuid;
BEGIN
  IF oid IS NULL THEN RAISE EXCEPTION 'no tenant in context' USING ERRCODE = 'P0001'; END IF;
  RETURN QUERY
    SELECT x.at, x.provider, x.kind, x.plan, x.detail FROM (
      SELECT e.received_at AS at, e.provider, e.kind, coalesce(e.plan, '') AS plan, e.detail
        FROM platform.billing_events e WHERE e.organization_id = oid
      UNION ALL
      SELECT a.created_at, coalesce(a.metadata->>'provider', ''), 'started', coalesce(a.metadata->>'plan', ''), coalesce(a.metadata->>'intent', '')
        FROM audit.audit_logs a WHERE a.tenant_id = oid AND a.action = 'billing.checkout_started'
    ) x ORDER BY x.at DESC LIMIT least(greatest(p_limit, 1), 100);
END $$;
-- +goose StatementEnd
ALTER FUNCTION platform.tenant_billing_history(int) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.tenant_billing_history(int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.tenant_billing_history(int) TO greenops_api;

-- +goose Down
DROP FUNCTION platform.tenant_billing_history(int);
