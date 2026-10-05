-- +goose Up
-- Global reference data (no tenant): refreshed from Electricity Maps.
CREATE TABLE carbon.grid_intensity (
    provider       text NOT NULL,
    region         text NOT NULL,
    ts             timestamptz NOT NULL,
    g_co2e_per_kwh numeric(10,3) NOT NULL CHECK (g_co2e_per_kwh >= 0),
    is_forecast    boolean NOT NULL DEFAULT false,
    PRIMARY KEY (provider, region, ts)
);
CREATE TABLE carbon.calculations (
    id                  uuid NOT NULL DEFAULT gen_random_uuid(),
    tenant_id           uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id          uuid NOT NULL,
    -- Denormalized dimensions copied from the usage record, so carbon can be
    -- sliced without joining the usage schema.
    provider            text NOT NULL,
    region              text NOT NULL,
    service_name        text NOT NULL,
    service_category    text NOT NULL CHECK (service_category IN ('compute','storage','database','networking','other')),
    period_start        timestamptz NOT NULL,
    period_end          timestamptz NOT NULL CHECK (period_end > period_start),
    energy_kwh          numeric(18,6) NOT NULL,
    carbon_kg_co2e      numeric(18,6) NOT NULL,
    sci_score           numeric(18,6),
    methodology_version text NOT NULL,   -- e.g. CCF-2026.1: results stay reproducible
    calculated_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id, period_start)
) PARTITION BY RANGE (period_start);
CREATE TABLE carbon.calculations_default PARTITION OF carbon.calculations DEFAULT;
CREATE INDEX ON carbon.calculations (tenant_id, project_id, period_start);
SELECT platform.enable_tenant_rls('carbon.calculations', 'tenant_id');
SELECT platform.ensure_month_partition('carbon.calculations', 'period_start', current_date);
SELECT platform.ensure_month_partition('carbon.calculations', 'period_start', (current_date + interval '1 month')::date);

-- +goose Down
DROP TABLE carbon.calculations; DROP TABLE carbon.grid_intensity;
