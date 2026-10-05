-- +goose Up
CREATE TABLE usage.usage_records (
    id            uuid NOT NULL DEFAULT gen_random_uuid(),
    tenant_id     uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id    uuid NOT NULL,
    source        text NOT NULL,
    provider      text NOT NULL,
    resource_id   text NOT NULL,
    resource_type text NOT NULL,
    service_name     text NOT NULL,   -- FOCUS ServiceName, e.g. "Amazon EC2"
    service_category text NOT NULL CHECK (service_category IN ('compute','storage','database','networking','other')),  -- FOCUS ServiceCategory
    region        text NOT NULL,
    usage_amount  numeric(24,6) NOT NULL CHECK (usage_amount >= 0),
    usage_unit    text NOT NULL,
    cost          numeric(18,6) NOT NULL CHECK (cost >= 0),
    currency      char(3) NOT NULL,
    recorded_at   timestamptz NOT NULL,
    PRIMARY KEY (id, recorded_at)
) PARTITION BY RANGE (recorded_at);
CREATE TABLE usage.usage_records_default PARTITION OF usage.usage_records DEFAULT;
CREATE INDEX ON usage.usage_records (tenant_id, project_id, recorded_at);
-- Idempotent re-sync: same resource/unit/instant is upserted, never duplicated.
CREATE UNIQUE INDEX ON usage.usage_records (tenant_id, provider, resource_id, usage_unit, recorded_at);
SELECT platform.enable_tenant_rls('usage.usage_records', 'tenant_id');
SELECT platform.ensure_month_partition('usage.usage_records', 'recorded_at', current_date);
SELECT platform.ensure_month_partition('usage.usage_records', 'recorded_at', (current_date + interval '1 month')::date);

-- +goose Down
DROP TABLE usage.usage_records;
