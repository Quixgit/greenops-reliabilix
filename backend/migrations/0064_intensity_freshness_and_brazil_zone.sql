-- +goose Up
-- 1. A reading older than a source's accepted age no longer outranks a fresh reading of a less trusted source when the platform
--    compares regions (recommendations, the Overview map): the policy already said so for calculations, the region queries did
--    not. Readings are still used when nothing fresher exists. carbon.source_max_age(provider) is the accepted age: the per-source
--    setting, else the common one, else 72 hours.
-- 2. The zone of São Paulo and Rio regions was BR-S (South); it is BR-CS (Southeast/Centre-West). Electricity Maps readings stored
--    under those regions came from the wrong zone and are removed: they would be averaged with the right ones. They are re-fetched.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION carbon.source_max_age(p_provider text) RETURNS interval
LANGUAGE sql STABLE SET search_path = pg_catalog, pg_temp AS $$
  SELECT make_interval(hours => COALESCE(
    (SELECT split_part(e, '=', 2)::int
       FROM settings.platform_values v, jsonb_array_elements_text(v.value) e
      WHERE v.key = 'carbon.source_max_age_hours' AND split_part(e, '=', 1) = p_provider AND split_part(e, '=', 2) ~ '^[0-9]+$'
      LIMIT 1),
    (SELECT (v.value #>> '{}')::int FROM settings.platform_values v WHERE v.key = 'carbon.max_reading_age_hours' AND (v.value #>> '{}') ~ '^[0-9]+$'),
    72))
$$;
-- +goose StatementEnd

DELETE FROM carbon.grid_intensity
 WHERE provider = 'electricitymaps' AND region_id IN ('sa-east-1', 'southamerica-east1', 'brazilsouth', 'brazilsoutheast');

-- +goose Down
DROP FUNCTION IF EXISTS carbon.source_max_age(text);
