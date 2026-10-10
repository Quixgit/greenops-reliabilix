-- +goose Up
-- Grid intensity from several sources (ADR-0035). A reading says which measure it is (average or marginal intensity are
-- different quantities and are never mixed), and every calculation row says which source and which basis its
-- intensity came from, so a figure can be explained.
ALTER TABLE carbon.grid_intensity ADD COLUMN measure text NOT NULL DEFAULT 'average' CHECK (measure IN ('average','marginal'));
ALTER TABLE carbon.grid_intensity DROP CONSTRAINT grid_intensity_pkey;
ALTER TABLE carbon.grid_intensity ADD PRIMARY KEY (provider, region_id, ts, is_forecast, measure);
CREATE INDEX grid_intensity_region_ts ON carbon.grid_intensity (region_id, ts DESC);

-- NULL on rows calculated before this change: the source of those intensities was not recorded and is not guessed.
ALTER TABLE carbon.calculations ADD COLUMN intensity_source text;
ALTER TABLE carbon.calculations ADD COLUMN intensity_basis text
  CHECK (intensity_basis IN ('day_mean','partial_day_mean','latest_before','latest_overall','live_provider'));

-- The ordered list of sources to trust (platform setting carbon.provider_priority). One definition for every query that
-- has to pick a source; the default must equal the catalog default of that setting (a test pins it).
-- +goose StatementBegin
CREATE FUNCTION carbon.provider_priority() RETURNS text[]
LANGUAGE sql STABLE SET search_path = pg_catalog, pg_temp AS $$
  SELECT COALESCE(
    (SELECT array(SELECT jsonb_array_elements_text(v.value)) FROM settings.platform_values v WHERE v.key = 'carbon.provider_priority'),
    ARRAY['electricitymaps'])
$$;
-- +goose StatementEnd
GRANT EXECUTE ON FUNCTION carbon.provider_priority() TO greenops_api, greenops_worker;

-- +goose Down
DROP FUNCTION carbon.provider_priority();
ALTER TABLE carbon.calculations DROP COLUMN intensity_basis, DROP COLUMN intensity_source;
DROP INDEX carbon.grid_intensity_region_ts;
ALTER TABLE carbon.grid_intensity DROP CONSTRAINT grid_intensity_pkey;
ALTER TABLE carbon.grid_intensity ADD PRIMARY KEY (provider, region_id, ts, is_forecast);
ALTER TABLE carbon.grid_intensity DROP COLUMN measure;
