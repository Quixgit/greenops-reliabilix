-- +goose Up
-- api: CRUD on business tables, read-only on audit trail.
-- worker: same data access (RLS-bound); no access to users/memberships administration.
GRANT USAGE ON SCHEMA tenants, projects, cloudaccounts, usage, carbon, finops, recommendations, reports, audit, platform
  TO greenops_api, greenops_worker;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA projects, cloudaccounts, usage, carbon, finops, recommendations, reports
  TO greenops_api, greenops_worker;
GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA tenants TO greenops_api;
GRANT SELECT ON tenants.organizations, tenants.memberships TO greenops_worker;
-- Audit is append-only for everyone at runtime: INSERT + SELECT, never UPDATE/DELETE.
GRANT SELECT, INSERT ON audit.audit_logs TO greenops_api, greenops_worker;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA audit TO greenops_api, greenops_worker;
GRANT EXECUTE ON FUNCTION platform.ensure_month_partition(regclass, text, date) TO greenops_worker;
GRANT EXECUTE ON FUNCTION cloudaccounts.list_connections_for_sync() TO greenops_worker;
-- carbon.grid_intensity is global reference data (no tenant column).
GRANT SELECT ON carbon.grid_intensity TO greenops_api;
GRANT SELECT, INSERT, UPDATE ON carbon.grid_intensity TO greenops_worker;

-- +goose Down
REVOKE ALL ON ALL TABLES IN SCHEMA tenants, projects, cloudaccounts, usage, carbon, finops, recommendations, reports, audit FROM greenops_api, greenops_worker;
