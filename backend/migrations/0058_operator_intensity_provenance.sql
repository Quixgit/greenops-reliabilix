-- +goose Up
-- Aggregation & policies in the operator console: where the intensity of the platform's carbon figures came from. The
-- function counts calculation rows by the source and basis that were recorded with them (ADR-0035). It returns counts
-- only, never an organization, a project or a value, and rows calculated before the origin was recorded are reported
-- with an empty source: their origin is not guessed.

-- +goose StatementBegin
CREATE FUNCTION carbon.operator_intensity_provenance(p_days int)
RETURNS TABLE (source text, basis text, n bigint)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT coalesce(c.intensity_source, ''), coalesce(c.intensity_basis, ''), count(*)
  FROM carbon.calculations c
  WHERE c.period_start >= now() - make_interval(days => greatest(1, least(p_days, 400)))
  GROUP BY 1, 2 ORDER BY 3 DESC
$$;
-- +goose StatementEnd
GRANT SELECT ON carbon.calculations TO greenops_fanout;
ALTER FUNCTION carbon.operator_intensity_provenance(int) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION carbon.operator_intensity_provenance(int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION carbon.operator_intensity_provenance(int) TO greenops_api;

-- +goose Down
DROP FUNCTION carbon.operator_intensity_provenance(int);
