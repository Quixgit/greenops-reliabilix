-- +goose Up
-- Credentials are NEVER stored here: only a role reference and a secret-manager path.
CREATE TABLE cloud_accounts (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           uuid NOT NULL,
    project_id          uuid NOT NULL,
    provider            text NOT NULL CHECK (provider IN ('aws','azure','gcp')),
    external_id         text NOT NULL,   -- AWS account id / subscription id / GCP project id
    role_ref            text NOT NULL,   -- e.g. IAM role ARN
    sts_external_id_ref text NOT NULL,   -- path in secret manager, not the value
    status              text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','healthy','error','disabled')),
    last_sync_at        timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, provider, external_id)
);
ALTER TABLE cloud_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE cloud_accounts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON cloud_accounts
    USING      (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose Down
DROP TABLE cloud_accounts;
