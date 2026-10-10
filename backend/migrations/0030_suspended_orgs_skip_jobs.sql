-- +goose Up
-- A suspended organization (setting organization.suspended, ADR-0033 / alpha.22) is skipped by the scheduler fan-out:
-- no cloud synchronization and no calculation jobs are started for it, so it costs nothing while suspended.
-- Reading its data and exporting it stay available through the API. Restoring access resumes the jobs at the next run.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION cloudaccounts.list_connections_for_sync()
RETURNS TABLE (tenant_id uuid, project_id uuid, connection_id uuid, provider text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT c.tenant_id, c.project_id, c.id, c.provider FROM cloudaccounts.connections c
  WHERE c.sync_status <> 'error'
    AND NOT EXISTS (SELECT 1 FROM settings.tenant_values v WHERE v.tenant_id = c.tenant_id AND v.key = 'organization.suspended' AND v.value = 'true'::jsonb)
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION projects.list_projects_for_jobs() RETURNS TABLE (tenant_id uuid, project_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT p.tenant_id, p.id FROM projects.projects p
  WHERE NOT EXISTS (SELECT 1 FROM settings.tenant_values v WHERE v.tenant_id = p.tenant_id AND v.key = 'organization.suspended' AND v.value = 'true'::jsonb)
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION cloudaccounts.list_connections_for_sync()
RETURNS TABLE (tenant_id uuid, project_id uuid, connection_id uuid, provider text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT c.tenant_id, c.project_id, c.id, c.provider FROM cloudaccounts.connections c WHERE c.sync_status <> 'error'
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION projects.list_projects_for_jobs() RETURNS TABLE (tenant_id uuid, project_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT p.tenant_id, p.id FROM projects.projects p
$$;
-- +goose StatementEnd
