-- +goose Up
-- Columns follow FOCUS v1.4 naming (FinOps Open Cost and Usage Specification):
--   service_name=ServiceName  service_category=ServiceCategory  region_id=RegionId  resource_id=ResourceId
--   resource_type=ResourceType  consumed_quantity/consumed_unit=ConsumedQuantity/ConsumedUnit
--   billed_cost=BilledCost  effective_cost=EffectiveCost  currency=BillingCurrency
--   recorded_at=ChargePeriodStart  charge_period_end=ChargePeriodEnd
-- consumed_* are NULL for sources that expose cost only (e.g. AWS Cost Explorer).
CREATE TABLE usage.usage_records (
    id                uuid NOT NULL DEFAULT gen_random_uuid(),
    tenant_id         uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id        uuid NOT NULL,
    connection_id     uuid,
    source            text NOT NULL,           -- cost_explorer | cur | k8s | ...
    provider          text NOT NULL,           -- aws | azure | gcp | k8s
    resource_id       text NOT NULL DEFAULT '',
    resource_type     text NOT NULL DEFAULT '',
    service_name      text NOT NULL,
    service_category  text NOT NULL CHECK (service_category IN ('compute','storage','database','networking','other')),
    region_id         text NOT NULL,
    consumed_quantity numeric(24,6) CHECK (consumed_quantity >= 0),
    consumed_unit     text,
    billed_cost       numeric(18,6) NOT NULL,   -- may be negative (credits/refunds)
    effective_cost    numeric(18,6) NOT NULL,
    currency          char(3) NOT NULL,
    recorded_at       timestamptz NOT NULL,
    charge_period_end timestamptz NOT NULL,
    PRIMARY KEY (id, recorded_at),
    CHECK (charge_period_end > recorded_at)
) PARTITION BY RANGE (recorded_at);
CREATE TABLE usage.usage_records_default PARTITION OF usage.usage_records DEFAULT;
CREATE INDEX ON usage.usage_records (tenant_id, project_id, recorded_at);
-- Idempotent re-sync: the same charge line is upserted, never duplicated.
CREATE UNIQUE INDEX usage_records_natural_key ON usage.usage_records
    (tenant_id, project_id, provider, service_name, region_id, resource_id, consumed_unit, recorded_at) NULLS NOT DISTINCT;
SELECT platform.enable_tenant_rls('usage.usage_records', 'tenant_id');
SELECT platform.ensure_month_partition('usage.usage_records', 'recorded_at', current_date);
SELECT platform.ensure_month_partition('usage.usage_records', 'recorded_at', (current_date + interval '1 month')::date);

-- +goose Down
DROP TABLE usage.usage_records;
