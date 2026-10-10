-- +goose Up
-- A source may publish monthly national averages instead of hourly readings (Ember). A reading says which it is, so a
-- monthly figure is never mistaken for an hourly one and never counted as "readings of the day".
ALTER TABLE carbon.grid_intensity ADD COLUMN resolution text NOT NULL DEFAULT 'hourly' CHECK (resolution IN ('hourly','monthly'));
ALTER TABLE carbon.calculations DROP CONSTRAINT calculations_intensity_basis_check;
ALTER TABLE carbon.calculations ADD CONSTRAINT calculations_intensity_basis_check
  CHECK (intensity_basis IN ('day_mean','month_mean','partial_day_mean','latest_before','latest_overall','live_provider'));

-- The default source order now includes Ember behind Electricity Maps: it only matters once Ember has data.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION carbon.provider_priority() RETURNS text[]
LANGUAGE sql STABLE SET search_path = pg_catalog, pg_temp AS $$
  SELECT COALESCE(
    (SELECT array(SELECT jsonb_array_elements_text(v.value)) FROM settings.platform_values v WHERE v.key = 'carbon.provider_priority'),
    ARRAY['electricitymaps','ember'])
$$;
-- +goose StatementEnd

-- Regions that have carbon calculations, for the worker's monthly refresh (region ids only).
-- +goose StatementBegin
CREATE FUNCTION carbon.regions_in_use() RETURNS TABLE (region_id text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT DISTINCT c.region_id FROM carbon.calculations c ORDER BY 1
$$;
-- +goose StatementEnd
ALTER FUNCTION carbon.regions_in_use() OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION carbon.regions_in_use() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION carbon.regions_in_use() TO greenops_worker, greenops_api;

-- Operator: per region in use, what each source has (hourly readings in the last 30 days, monthly figures, newest reading).
-- +goose StatementBegin
CREATE FUNCTION carbon.operator_region_sources()
RETURNS TABLE (region_id text, provider text, hourly_30d int, months int, newest timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT g.region_id, g.provider,
         (count(*) FILTER (WHERE g.resolution = 'hourly' AND g.ts > now() - interval '30 days'))::int,
         (count(*) FILTER (WHERE g.resolution = 'monthly'))::int,
         max(g.ts)
  FROM carbon.grid_intensity g
  WHERE g.measure = 'average' AND NOT g.is_forecast AND g.region_id IN (SELECT DISTINCT c.region_id FROM carbon.calculations c)
  GROUP BY g.region_id, g.provider ORDER BY g.region_id, g.provider
$$;
-- +goose StatementEnd
GRANT SELECT ON carbon.grid_intensity TO greenops_fanout;
ALTER FUNCTION carbon.operator_region_sources() OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION carbon.operator_region_sources() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION carbon.operator_region_sources() TO greenops_api;

-- +goose Down
DROP FUNCTION carbon.operator_region_sources(), carbon.regions_in_use();
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION carbon.provider_priority() RETURNS text[]
LANGUAGE sql STABLE SET search_path = pg_catalog, pg_temp AS $$
  SELECT COALESCE((SELECT array(SELECT jsonb_array_elements_text(v.value)) FROM settings.platform_values v WHERE v.key = 'carbon.provider_priority'), ARRAY['electricitymaps'])
$$;
-- +goose StatementEnd
ALTER TABLE carbon.calculations DROP CONSTRAINT calculations_intensity_basis_check;
ALTER TABLE carbon.calculations ADD CONSTRAINT calculations_intensity_basis_check
  CHECK (intensity_basis IN ('day_mean','partial_day_mean','latest_before','latest_overall','live_provider'));
ALTER TABLE carbon.grid_intensity DROP COLUMN resolution;
