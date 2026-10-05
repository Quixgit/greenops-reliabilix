-- +goose Up
-- Global reference data (no tenant): refreshed hourly from Electricity Maps (WattTime later).
CREATE TABLE carbon.grid_intensity (
    provider       text NOT NULL,
    region_id      text NOT NULL,
    ts             timestamptz NOT NULL,
    g_co2e_per_kwh numeric(10,3) NOT NULL CHECK (g_co2e_per_kwh >= 0),
    is_forecast    boolean NOT NULL DEFAULT false,
    PRIMARY KEY (provider, region_id, ts, is_forecast)
);
CREATE TABLE carbon.calculations (
    id                  uuid NOT NULL DEFAULT gen_random_uuid(),
    tenant_id           uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id          uuid NOT NULL,
    -- dimensions copied from usage so carbon can be sliced without joining the usage schema
    provider            text NOT NULL,
    region_id           text NOT NULL,
    service_name        text NOT NULL,
    service_category    text NOT NULL CHECK (service_category IN ('compute','storage','database','networking','other')),
    method              text NOT NULL CHECK (method IN ('usage_based','cost_based')),
    period_start        timestamptz NOT NULL,
    period_end          timestamptz NOT NULL CHECK (period_end > period_start),
    energy_kwh          numeric(18,6) NOT NULL CHECK (energy_kwh >= 0),
    intensity_g_per_kwh numeric(10,3) NOT NULL,
    embodied_g          numeric(18,3) NOT NULL DEFAULT 0,   -- SCI "M"; 0 in methodology 2026.1 (operational only)
    carbon_kg_co2e      numeric(18,6) NOT NULL,
    functional_units    numeric(24,3),                      -- SCI "R" for the period, when the customer reported it
    sci_score           numeric(18,6),                      -- NULL unless R is known: never guessed
    methodology_version text NOT NULL,                      -- e.g. CCF-2026.1; history is never rewritten
    calculated_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id, period_start)
) PARTITION BY RANGE (period_start);
CREATE TABLE carbon.calculations_default PARTITION OF carbon.calculations DEFAULT;
CREATE INDEX ON carbon.calculations (tenant_id, project_id, period_start);
CREATE UNIQUE INDEX calculations_natural_key ON carbon.calculations
    (tenant_id, project_id, provider, service_name, region_id, method, methodology_version, period_start);
SELECT platform.enable_tenant_rls('carbon.calculations', 'tenant_id');
SELECT platform.ensure_month_partition('carbon.calculations', 'period_start', current_date);
SELECT platform.ensure_month_partition('carbon.calculations', 'period_start', (current_date + interval '1 month')::date);

-- +goose Down
DROP TABLE carbon.calculations; DROP TABLE carbon.grid_intensity;
