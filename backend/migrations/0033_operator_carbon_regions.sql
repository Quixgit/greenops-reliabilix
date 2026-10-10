-- +goose Up
-- Operator console: the regions that have carbon calculations (for any organization), so a grid-history backfill asks
-- the provider only about regions that are in use. Region ids only; no tenant data.
-- +goose StatementBegin
CREATE FUNCTION carbon.operator_regions() RETURNS TABLE (region_id text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT DISTINCT c.region_id FROM carbon.calculations c ORDER BY 1
$$;
-- +goose StatementEnd
GRANT USAGE ON SCHEMA carbon TO greenops_fanout;
GRANT SELECT ON carbon.calculations TO greenops_fanout;
ALTER FUNCTION carbon.operator_regions() OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION carbon.operator_regions() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION carbon.operator_regions() TO greenops_api;

-- +goose Down
DROP FUNCTION carbon.operator_regions();
