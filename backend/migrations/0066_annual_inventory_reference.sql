-- +goose Up
-- An annual reference intensity per region: the national power-sector emissions of the Climate TRACE inventory divided by the
-- generation of the same year (Ember). It is stored under its own provider id, which is not in the source policy, so no calculation
-- takes it; the operator's divergence table holds the hourly sources against it. Resolution gains "annual".
ALTER TABLE carbon.grid_intensity DROP CONSTRAINT IF EXISTS grid_intensity_resolution_check;
ALTER TABLE carbon.grid_intensity ADD CONSTRAINT grid_intensity_resolution_check CHECK (resolution IN ('hourly','monthly','annual'));

DROP FUNCTION carbon.operator_source_divergence(int);
-- +goose StatementBegin
CREATE FUNCTION carbon.operator_source_divergence(p_days int)
RETURNS TABLE (region_id text, provider text, g float8, n int, newest timestamptz, kind text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT x.region_id, x.provider, x.g, x.n, x.newest, 'hourly'::text FROM (
    SELECT i.region_id, i.provider,
           (percentile_cont(0.5) WITHIN GROUP (ORDER BY i.g_co2e_per_kwh))::float8 AS g, count(*)::int AS n, max(i.ts) AS newest
    FROM carbon.grid_intensity i
    WHERE i.measure = 'average' AND NOT i.is_forecast AND i.resolution = 'hourly' AND i.ts > now() - make_interval(days => p_days)
    GROUP BY i.region_id, i.provider HAVING count(*) >= 24) x
  UNION ALL
  SELECT y.region_id, y.provider, y.g, 1, y.ts, y.resolution FROM (
    SELECT DISTINCT ON (m.region_id, m.provider, m.resolution) m.region_id, m.provider, m.g_co2e_per_kwh::float8 AS g, m.ts, m.resolution
    FROM carbon.grid_intensity m
    WHERE m.measure = 'average' AND NOT m.is_forecast AND m.resolution IN ('monthly', 'annual')
    ORDER BY m.region_id, m.provider, m.resolution, m.ts DESC) y
$$;
-- +goose StatementEnd
ALTER FUNCTION carbon.operator_source_divergence(int) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION carbon.operator_source_divergence(int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION carbon.operator_source_divergence(int) TO greenops_api;

-- +goose Down
DROP FUNCTION carbon.operator_source_divergence(int);
-- +goose StatementBegin
CREATE FUNCTION carbon.operator_source_divergence(p_days int)
RETURNS TABLE (region_id text, provider text, g float8, n int, newest timestamptz, monthly boolean)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT x.region_id, x.provider, x.g, x.n, x.newest, false FROM (
    SELECT i.region_id, i.provider,
           (percentile_cont(0.5) WITHIN GROUP (ORDER BY i.g_co2e_per_kwh))::float8 AS g, count(*)::int AS n, max(i.ts) AS newest
    FROM carbon.grid_intensity i
    WHERE i.measure = 'average' AND NOT i.is_forecast AND i.resolution = 'hourly' AND i.ts > now() - make_interval(days => p_days)
    GROUP BY i.region_id, i.provider HAVING count(*) >= 24) x
  UNION ALL
  SELECT y.region_id, y.provider, y.g, 1, y.ts, true FROM (
    SELECT DISTINCT ON (m.region_id, m.provider) m.region_id, m.provider, m.g_co2e_per_kwh::float8 AS g, m.ts
    FROM carbon.grid_intensity m
    WHERE m.measure = 'average' AND NOT m.is_forecast AND m.resolution = 'monthly'
    ORDER BY m.region_id, m.provider, m.ts DESC) y
$$;
-- +goose StatementEnd
ALTER FUNCTION carbon.operator_source_divergence(int) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION carbon.operator_source_divergence(int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION carbon.operator_source_divergence(int) TO greenops_api;
ALTER TABLE carbon.grid_intensity DROP CONSTRAINT IF EXISTS grid_intensity_resolution_check;
DELETE FROM carbon.grid_intensity WHERE resolution = 'annual';
ALTER TABLE carbon.grid_intensity ADD CONSTRAINT grid_intensity_resolution_check CHECK (resolution IN ('hourly','monthly'));
