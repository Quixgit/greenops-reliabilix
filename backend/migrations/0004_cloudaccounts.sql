-- +goose Up
-- credential_ref is a REFERENCE (IAM role ARN / secret-manager path). Never a secret.
-- tenant_id is added to the spec'd shape so the table can carry its own RLS policy.
CREATE TABLE cloudaccounts.connections (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id     uuid NOT NULL REFERENCES projects.projects(id),
    provider       text NOT NULL CHECK (provider IN ('aws','azure','gcp')),
    account_ref    text NOT NULL,
    credential_ref text NOT NULL,
    last_sync_at   timestamptz,
    sync_status    text NOT NULL DEFAULT 'pending' CHECK (sync_status IN ('healthy','error','pending')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, provider, account_ref)
);
SELECT platform.enable_tenant_rls('cloudaccounts.connections', 'tenant_id');

-- Worker fan-out across tenants WITHOUT giving the worker role BYPASSRLS:
-- a narrow SECURITY DEFINER function owned by the migration (owner) role.
-- +goose StatementBegin
CREATE FUNCTION cloudaccounts.list_connections_for_sync()
RETURNS TABLE (tenant_id uuid, project_id uuid, connection_id uuid, provider text)
LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT c.tenant_id, c.project_id, c.id, c.provider FROM cloudaccounts.connections c
  WHERE c.sync_status <> 'error'
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION cloudaccounts.list_connections_for_sync() FROM PUBLIC;
-- FORCE RLS also binds table owners, so the function owner is a dedicated
-- NOLOGIN role with BYPASSRLS and SELECT on this one table (creating such a
-- role requires the migration role to be a superuser or have CREATEROLE+BYPASSRLS).
-- +goose StatementBegin
DO $$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'greenops_fanout') THEN
    CREATE ROLE greenops_fanout NOLOGIN BYPASSRLS;
  END IF;
END $$;
-- +goose StatementEnd
GRANT USAGE ON SCHEMA cloudaccounts TO greenops_fanout;
GRANT SELECT ON cloudaccounts.connections TO greenops_fanout;
ALTER FUNCTION cloudaccounts.list_connections_for_sync() OWNER TO greenops_fanout;

-- +goose Down
DROP FUNCTION cloudaccounts.list_connections_for_sync();
DROP TABLE cloudaccounts.connections;
