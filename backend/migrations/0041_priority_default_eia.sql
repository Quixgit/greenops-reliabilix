-- +goose Up
-- Default source order: Electricity Maps, UK Carbon Intensity (GB), EIA (US, computed), Ember (monthly). A source with
-- no data for a region is skipped, so adding one changes nothing for regions it does not cover.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION carbon.provider_priority() RETURNS text[]
LANGUAGE sql STABLE SET search_path = pg_catalog, pg_temp AS $$
  SELECT COALESCE(
    (SELECT array(SELECT jsonb_array_elements_text(v.value)) FROM settings.platform_values v WHERE v.key = 'carbon.provider_priority'),
    ARRAY['electricitymaps','uk_carbon_intensity','eia','ember'])
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION carbon.provider_priority() RETURNS text[]
LANGUAGE sql STABLE SET search_path = pg_catalog, pg_temp AS $$
  SELECT COALESCE((SELECT array(SELECT jsonb_array_elements_text(v.value)) FROM settings.platform_values v WHERE v.key = 'carbon.provider_priority'), ARRAY['electricitymaps','uk_carbon_intensity','ember'])
$$;
-- +goose StatementEnd
