-- +goose Up
-- Coverage of the grid data of the regions in use: how many of the last N days (ending yesterday) have an hourly reading
-- of any source or fall in a month with a published monthly figure. Used by the daily job that loads a region's past by
-- itself when a region is new or has gaps. Region ids and counts only.
-- +goose StatementBegin
CREATE FUNCTION carbon.region_coverage(p_days int) RETURNS TABLE (region_id text, covered_days int, total_days int)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  WITH n AS (SELECT least(greatest(p_days, 1), 400) AS days),
  w AS (
    SELECT d::date AS day FROM n, generate_series((now() AT TIME ZONE 'utc')::date - n.days, (now() AT TIME ZONE 'utc')::date - 1, interval '1 day') d
  ),
  r AS (SELECT DISTINCT c.region_id FROM carbon.calculations c)
  SELECT r.region_id,
    (SELECT count(*) FROM w WHERE
       EXISTS (SELECT 1 FROM carbon.grid_intensity g WHERE g.region_id = r.region_id AND g.measure = 'average' AND NOT g.is_forecast
                 AND g.resolution = 'hourly' AND g.ts >= w.day AND g.ts < w.day + 1)
    OR EXISTS (SELECT 1 FROM carbon.grid_intensity g WHERE g.region_id = r.region_id AND g.measure = 'average' AND NOT g.is_forecast
                 AND g.resolution = 'monthly' AND g.ts = date_trunc('month', w.day::timestamp)))::int,
    (SELECT days FROM n)
  FROM r ORDER BY r.region_id
$$;
-- +goose StatementEnd
ALTER FUNCTION carbon.region_coverage(int) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION carbon.region_coverage(int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION carbon.region_coverage(int) TO greenops_worker, greenops_api;

-- +goose Down
DROP FUNCTION carbon.region_coverage(int);
