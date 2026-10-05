-- +goose Up
-- Grid intensity is global reference data (no tenant), refreshed from Electricity Maps.
CREATE TABLE grid_intensity (
    provider       text NOT NULL,
    region         text NOT NULL,
    ts             timestamptz NOT NULL,
    g_co2e_per_kwh numeric(10,3) NOT NULL CHECK (g_co2e_per_kwh >= 0),
    is_forecast    boolean NOT NULL DEFAULT false,
    PRIMARY KEY (provider, region, ts)
);

CREATE TABLE carbon_calculations (
    id                  uuid NOT NULL DEFAULT gen_random_uuid(),
    tenant_id           uuid NOT NULL,
    project_id          uuid NOT NULL,
    usage_record_id     uuid NOT NULL,
    period_start        timestamptz NOT NULL,
    energy_kwh          numeric(18,6) NOT NULL,
    co2e_kg             numeric(18,6) NOT NULL,
    methodology_version text NOT NULL,   -- e.g. CCF-2026.1; results stay reproducible
    calculated_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id, period_start)
) PARTITION BY RANGE (period_start);
CREATE TABLE carbon_calculations_default PARTITION OF carbon_calculations DEFAULT;
CREATE INDEX ON carbon_calculations (tenant_id, project_id, period_start);

ALTER TABLE carbon_calculations ENABLE ROW LEVEL SECURITY;
ALTER TABLE carbon_calculations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON carbon_calculations
    USING      (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose Down
DROP TABLE carbon_calculations;
DROP TABLE grid_intensity;
