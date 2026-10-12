-- +goose Up
-- One cloud at a time (ADR-0065). A request that carries ?cloud=aws binds the transaction setting app.cloud; these restrictive read
-- policies then hide the rows of every other cloud in every query of that request, so the cost, carbon, recommendations and connection
-- figures of a cloud are computed from that cloud alone, with no query to remember. Without the setting nothing changes. The setting is
-- evaluated once per statement (an uncorrelated sub-select), not per row.
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['usage.usage_records', 'usage.tagged_cost', 'carbon.calculations', 'recommendations.recommendations', 'cloudaccounts.connections'] LOOP
    EXECUTE format($f$CREATE POLICY cloud_scope ON %s AS RESTRICTIVE FOR SELECT
      USING ((SELECT NULLIF(current_setting('app.cloud', true), '')) IS NULL OR provider = (SELECT current_setting('app.cloud', true)))$f$, t);
  END LOOP;
END $$;
-- +goose StatementEnd

-- A cloud's own figures are in its own currency: in a scoped request the organization's reporting currency is not applied, the currency
-- is the one carrying most of that cloud's cost (the scoped rows only).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION finops.reporting_currency(p_tenant uuid, p_dominant text) RETURNS text
LANGUAGE sql STABLE SET search_path = pg_catalog, pg_temp AS $$
  SELECT COALESCE(
    CASE WHEN NULLIF(current_setting('app.cloud', true), '') IS NULL THEN
      COALESCE(
        NULLIF(NULLIF((SELECT value #>> '{}' FROM settings.tenant_values WHERE tenant_id = p_tenant AND key = 'finops.reporting_currency'), 'auto'), ''),
        NULLIF(NULLIF((SELECT value #>> '{}' FROM settings.platform_values WHERE key = 'finops.reporting_currency'), 'auto'), ''))
    END,
    (SELECT g.currency FROM (
        SELECT u.currency::text AS currency, sum(abs(u.billed_cost)) AS total
        FROM usage.usage_records u
        WHERE u.tenant_id = p_tenant AND u.recorded_at >= now() - interval '90 days'
        GROUP BY 1) g
      LEFT JOIN LATERAL (SELECT r.per_eur FROM finops.fx_rates r WHERE r.currency = upper(g.currency) ORDER BY r.day DESC LIMIT 1) fx ON true
      ORDER BY g.total / COALESCE(CASE WHEN upper(g.currency) = 'EUR' THEN 1 END, fx.per_eur, 1) DESC, g.currency
      LIMIT 1),
    p_dominant)
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['usage.usage_records', 'usage.tagged_cost', 'carbon.calculations', 'recommendations.recommendations', 'cloudaccounts.connections'] LOOP
    EXECUTE format('DROP POLICY IF EXISTS cloud_scope ON %s', t);
  END LOOP;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION finops.reporting_currency(p_tenant uuid, p_dominant text) RETURNS text
LANGUAGE sql STABLE SET search_path = pg_catalog, pg_temp AS $$
  SELECT COALESCE(
    NULLIF(NULLIF((SELECT value #>> '{}' FROM settings.tenant_values WHERE tenant_id = p_tenant AND key = 'finops.reporting_currency'), 'auto'), ''),
    NULLIF(NULLIF((SELECT value #>> '{}' FROM settings.platform_values WHERE key = 'finops.reporting_currency'), 'auto'), ''),
    (SELECT g.currency FROM (
        SELECT u.currency::text AS currency, sum(abs(u.billed_cost)) AS total
        FROM usage.usage_records u
        WHERE u.tenant_id = p_tenant AND u.recorded_at >= now() - interval '90 days'
        GROUP BY 1) g
      LEFT JOIN LATERAL (SELECT r.per_eur FROM finops.fx_rates r WHERE r.currency = upper(g.currency) ORDER BY r.day DESC LIMIT 1) fx ON true
      ORDER BY g.total / COALESCE(CASE WHEN upper(g.currency) = 'EUR' THEN 1 END, fx.per_eur, 1) DESC, g.currency
      LIMIT 1),
    p_dominant)
$$;
-- +goose StatementEnd
