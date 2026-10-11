-- +goose Up
-- Two accounts of one provider in one project (two AWS accounts, two Azure subscriptions) produce lines with the same
-- service, region and day. The natural key did not know the account, so the second sync overwrote the first one's
-- lines: the costs of one account silently replaced the other's. The account is part of the key now. It is the
-- account (not the connection) on purpose: deleting a connection keeps its history, and connecting the same account
-- again must update those lines, not double them.
ALTER TABLE usage.usage_records ADD COLUMN account_ref text NOT NULL DEFAULT '';
UPDATE usage.usage_records u SET account_ref = c.account_ref
FROM cloudaccounts.connections c WHERE u.connection_id = c.id AND u.tenant_id = c.tenant_id;
CREATE UNIQUE INDEX usage_records_natural_key_v2 ON usage.usage_records
    (tenant_id, project_id, provider, account_ref, service_name, region_id, resource_id, consumed_unit, recorded_at) NULLS NOT DISTINCT;
DROP INDEX usage.usage_records_natural_key;
ALTER INDEX usage.usage_records_natural_key_v2 RENAME TO usage_records_natural_key;

-- +goose Down
CREATE UNIQUE INDEX usage_records_natural_key_v1 ON usage.usage_records
    (tenant_id, project_id, provider, service_name, region_id, resource_id, consumed_unit, recorded_at) NULLS NOT DISTINCT;
DROP INDEX usage.usage_records_natural_key;
ALTER INDEX usage.usage_records_natural_key_v1 RENAME TO usage_records_natural_key;
ALTER TABLE usage.usage_records DROP COLUMN account_ref;
