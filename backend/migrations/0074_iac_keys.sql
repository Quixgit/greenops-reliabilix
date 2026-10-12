-- +goose Up
-- API keys of role 'iac' for infrastructure as code: the Terraform provider (ADR-0057).
ALTER TABLE tenants.api_keys DROP CONSTRAINT api_keys_role_check;
ALTER TABLE tenants.api_keys ADD CONSTRAINT api_keys_role_check CHECK (role IN ('ci','viewer','agent','scim','iac'));

-- +goose Down
DELETE FROM tenants.api_keys WHERE role = 'iac';
ALTER TABLE tenants.api_keys DROP CONSTRAINT api_keys_role_check;
ALTER TABLE tenants.api_keys ADD CONSTRAINT api_keys_role_check CHECK (role IN ('ci','viewer','agent','scim'));
