-- +goose Up
-- "auto" reporting currency no longer depends on the window a query happens to cover. With two clouds in two currencies the dominant
-- currency of a 30-day window and of the 56-day window the forecast reads could differ (USD in one, CAD in the other), and a forecast
-- computed in dollars was printed as Canadian dollars. Without a choice, the currency is now the one carrying most of the organization's
-- cost over the last 90 days, measured in euros at the latest reference rate (so 100,000 JPY does not outweigh 900 USD); the window's own
-- dominant currency (p_dominant) is only the fallback when there is no cost in those 90 days.
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

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION finops.reporting_currency(p_tenant uuid, p_dominant text) RETURNS text
LANGUAGE sql STABLE SET search_path = pg_catalog, pg_temp AS $$
  SELECT COALESCE(
    NULLIF(NULLIF((SELECT value #>> '{}' FROM settings.tenant_values WHERE tenant_id = p_tenant AND key = 'finops.reporting_currency'), 'auto'), ''),
    NULLIF(NULLIF((SELECT value #>> '{}' FROM settings.platform_values WHERE key = 'finops.reporting_currency'), 'auto'), ''),
    p_dominant)
$$;
-- +goose StatementEnd
