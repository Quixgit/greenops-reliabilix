-- +goose Up
-- credential_ref is a REFERENCE (IAM role ARN / secret-manager path), never a secret.
-- external_id is the per-connection STS ExternalId (confused-deputy protection): unguessable,
-- generated server-side and shown to the customer for their role trust policy. It is not a credential.
CREATE TABLE cloudaccounts.connections (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id     uuid NOT NULL REFERENCES projects.projects(id),
    provider       text NOT NULL CHECK (provider IN ('aws','azure','gcp')),
    account_ref    text NOT NULL,
    credential_ref text NOT NULL,
    external_id    text NOT NULL,
    sync_status    text NOT NULL DEFAULT 'pending' CHECK (sync_status IN ('healthy','error','pending')),
    last_error     text,
    last_sync_at   timestamptz,
    synced_through date,          -- last fully ingested day (inclusive); next sync resumes after it
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, provider, account_ref)
);
CREATE TABLE cloudaccounts.sync_runs (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid NOT NULL REFERENCES tenants.organizations(id),
    connection_id uuid NOT NULL REFERENCES cloudaccounts.connections(id) ON DELETE CASCADE,
    started_at    timestamptz NOT NULL DEFAULT now(),
    finished_at   timestamptz,
    status        text NOT NULL DEFAULT 'running' CHECK (status IN ('running','succeeded','failed')),
    records       integer NOT NULL DEFAULT 0,
    error         text
);
CREATE INDEX ON cloudaccounts.sync_runs (connection_id, started_at DESC);
SELECT platform.enable_tenant_rls('cloudaccounts.connections', 'tenant_id');
SELECT platform.enable_tenant_rls('cloudaccounts.sync_runs', 'tenant_id');

-- +goose Down
DROP TABLE cloudaccounts.sync_runs, cloudaccounts.connections;
