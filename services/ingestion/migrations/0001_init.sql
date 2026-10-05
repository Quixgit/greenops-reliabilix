-- +goose Up
CREATE TABLE usage_records (
    id            uuid NOT NULL DEFAULT gen_random_uuid(),
    tenant_id     uuid NOT NULL,
    project_id    uuid NOT NULL,
    provider      text NOT NULL,
    source        text NOT NULL,
    resource_id   text NOT NULL,
    resource_type text NOT NULL,
    region        text NOT NULL,
    usage_amount  numeric(24,6) NOT NULL CHECK (usage_amount >= 0),
    usage_unit    text NOT NULL,
    cost          numeric(18,6) NOT NULL CHECK (cost >= 0),
    currency      char(3) NOT NULL,
    period_start  timestamptz NOT NULL,
    period_end    timestamptz NOT NULL CHECK (period_end > period_start),
    PRIMARY KEY (id, period_start)
) PARTITION BY RANGE (period_start);
-- Monthly partitions are created by a scheduled job (pg_partman or the service itself).
CREATE TABLE usage_records_default PARTITION OF usage_records DEFAULT;
CREATE INDEX ON usage_records (tenant_id, project_id, period_start);
-- Idempotent re-sync: the same resource+period is upserted, never duplicated.
CREATE UNIQUE INDEX ON usage_records (tenant_id, provider, resource_id, usage_unit, period_start);

ALTER TABLE usage_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE usage_records FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON usage_records
    USING      (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose Down
DROP TABLE usage_records;
