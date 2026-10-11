-- +goose Up
-- Default source order gains ENTSO-E (Europe, needs a free token; skipped without one) and ONS (Brazil, open). Both are computed from
-- the generation mix, so they sit behind the sources that publish a measured intensity. A source with no data for a region is skipped.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION carbon.provider_priority() RETURNS text[]
LANGUAGE sql STABLE SET search_path = pg_catalog, pg_temp AS $$
  SELECT COALESCE(
    (SELECT array(SELECT jsonb_array_elements_text(v.value)) FROM settings.platform_values v WHERE v.key = 'carbon.provider_priority'),
    ARRAY['electricitymaps','uk_carbon_intensity','eia','energy_charts','entsoe','ieso','hydroquebec','ons','ember'])
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION carbon.provider_priority() RETURNS text[]
LANGUAGE sql STABLE SET search_path = pg_catalog, pg_temp AS $$
  SELECT COALESCE((SELECT array(SELECT jsonb_array_elements_text(v.value)) FROM settings.platform_values v WHERE v.key = 'carbon.provider_priority'), ARRAY['electricitymaps','uk_carbon_intensity','eia','energy_charts','ieso','hydroquebec','ember'])
$$;
-- +goose StatementEnd
