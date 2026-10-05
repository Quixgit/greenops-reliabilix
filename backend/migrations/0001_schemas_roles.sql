-- +goose Up
CREATE SCHEMA tenants;
CREATE SCHEMA projects;
CREATE SCHEMA cloudaccounts;
CREATE SCHEMA usage;
CREATE SCHEMA carbon;
CREATE SCHEMA finops;
CREATE SCHEMA recommendations;
CREATE SCHEMA reports;
CREATE SCHEMA audit;
CREATE SCHEMA platform;

-- Least-privilege runtime roles. Passwords/secrets are set by the deployment
-- (secret manager), never in migrations. Neither role owns tables or has
-- BYPASSRLS, so Row Level Security always applies to them.
-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'greenops_api')    THEN CREATE ROLE greenops_api    LOGIN; END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'greenops_worker') THEN CREATE ROLE greenops_worker LOGIN; END IF;
END $$;
-- +goose StatementEnd

-- Enables and FORCES RLS on a table, scoped by the transaction-local setting
-- app.tenant_id (set by backend/internal/platform/database.WithTenantTx).
-- Missing setting => NULL => no rows (fail closed).
-- +goose StatementBegin
CREATE FUNCTION platform.enable_tenant_rls(tbl regclass, col text) RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
  EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY', tbl);
  EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY', tbl);
  EXECUTE format($p$CREATE POLICY tenant_isolation ON %s
      USING (%I = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
      WITH CHECK (%I = NULLIF(current_setting('app.tenant_id', true), '')::uuid)$p$, tbl, col, col);
END $$;
-- +goose StatementEnd

-- Creates the monthly partition of a range-partitioned table for the month of d.
-- +goose StatementBegin
CREATE FUNCTION platform.ensure_month_partition(parent regclass, col text, d date) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE
  from_d date := date_trunc('month', d)::date;
  to_d   date := (date_trunc('month', d) + interval '1 month')::date;
  part   text := format('%s_%s', parent::text, to_char(from_d, 'YYYY_MM'));
BEGIN
  -- SECURITY DEFINER: only the known partitioned tables may be touched.
  IF parent::text NOT IN ('usage.usage_records', 'carbon.calculations') THEN
    RAISE EXCEPTION 'ensure_month_partition: % is not a managed partitioned table', parent;
  END IF;
  IF to_regclass(part) IS NULL THEN
    EXECUTE format('CREATE TABLE %s PARTITION OF %s FOR VALUES FROM (%L) TO (%L)', part, parent, from_d, to_d);
  END IF;
END $$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION platform.enable_tenant_rls(regclass, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION platform.ensure_month_partition(regclass, text, date) FROM PUBLIC;

-- +goose Down
DROP FUNCTION platform.ensure_month_partition(regclass, text, date);
DROP FUNCTION platform.enable_tenant_rls(regclass, text);
DROP SCHEMA platform, audit, reports, recommendations, finops, carbon, usage, cloudaccounts, projects, tenants;
