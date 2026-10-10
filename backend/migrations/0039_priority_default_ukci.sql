-- +goose Up
-- The default source order is Electricity Maps, UK Carbon Intensity (Great Britain only, free, no key), then Ember. A
-- source with no data for a region is skipped, so adding it changes nothing for regions it does not cover.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION carbon.provider_priority() RETURNS text[]
LANGUAGE sql STABLE SET search_path = pg_catalog, pg_temp AS $$
  SELECT COALESCE(
    (SELECT array(SELECT jsonb_array_elements_text(v.value)) FROM settings.platform_values v WHERE v.key = 'carbon.provider_priority'),
    ARRAY['electricitymaps','uk_carbon_intensity','ember'])
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION carbon.provider_priority() RETURNS text[]
LANGUAGE sql STABLE SET search_path = pg_catalog, pg_temp AS $$
  SELECT COALESCE((SELECT array(SELECT jsonb_array_elements_text(v.value)) FROM settings.platform_values v WHERE v.key = 'carbon.provider_priority'), ARRAY['electricitymaps','ember'])
$$;
-- +goose StatementEnd
