-- +goose Up
-- The currency cost is reported in: the organization's choice (setting finops.reporting_currency), else the platform's,
-- else the dominant currency of the window that the caller passes. "auto" and an empty value mean "no choice".
-- Runs with the caller's rights: the organization's value is visible only inside its own tenant transaction (RLS).
-- +goose StatementBegin
CREATE FUNCTION finops.reporting_currency(p_tenant uuid, p_dominant text) RETURNS text
LANGUAGE sql STABLE SET search_path = pg_catalog, pg_temp AS $$
  SELECT COALESCE(
    NULLIF(NULLIF((SELECT value #>> '{}' FROM settings.tenant_values WHERE tenant_id = p_tenant AND key = 'finops.reporting_currency'), 'auto'), ''),
    NULLIF(NULLIF((SELECT value #>> '{}' FROM settings.platform_values WHERE key = 'finops.reporting_currency'), 'auto'), ''),
    p_dominant)
$$;
-- +goose StatementEnd
GRANT EXECUTE ON FUNCTION finops.reporting_currency(uuid, text) TO greenops_api, greenops_worker;
GRANT SELECT ON settings.tenant_values, settings.platform_values TO greenops_api, greenops_worker;

-- +goose Down
DROP FUNCTION finops.reporting_currency(uuid, text);
