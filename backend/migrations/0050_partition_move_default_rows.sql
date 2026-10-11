-- +goose Up
-- A monthly partition cannot be created while the DEFAULT partition holds rows of that month: PostgreSQL refuses with
-- "updated partition constraint for default partition would be violated by some row". Rows end up there when data of a
-- month arrives before its partition exists (a seed, a manual load, a backfill that reached further back than the
-- partitions that were created). Every later sync of that month then failed for good. The function now moves such rows
-- into the new partition: they are read and written through the partition tables directly (row-level security is
-- defined on the parent), in the same transaction as the creation.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION platform.ensure_month_partition(parent regclass, col text, d date) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE
  from_d date := date_trunc('month', d)::date;
  to_d   date := (date_trunc('month', d) + interval '1 month')::date;
  part   text := format('%s_%s', parent::text, to_char(from_d, 'YYYY_MM'));
  def    text := format('%s_default', parent::text);
  held   bigint := 0;
BEGIN
  IF parent::text NOT IN ('usage.usage_records', 'carbon.calculations') THEN
    RAISE EXCEPTION 'ensure_month_partition: % is not a managed partitioned table', parent;
  END IF;
  IF to_regclass(part) IS NOT NULL THEN
    RETURN;
  END IF;
  IF to_regclass(def) IS NOT NULL THEN
    EXECUTE format('SELECT count(*) FROM %s WHERE %I >= %L AND %I < %L', def, col, from_d, col, to_d) INTO held;
  END IF;
  IF held = 0 THEN
    EXECUTE format('CREATE TABLE %s PARTITION OF %s FOR VALUES FROM (%L) TO (%L)', part, parent, from_d, to_d);
    RETURN;
  END IF;
  -- move: set the month's rows aside, create the partition, put them back
  EXECUTE format('CREATE TEMP TABLE partition_move ON COMMIT DROP AS SELECT * FROM %s WHERE %I >= %L AND %I < %L', def, col, from_d, col, to_d);
  EXECUTE format('DELETE FROM %s WHERE %I >= %L AND %I < %L', def, col, from_d, col, to_d);
  EXECUTE format('CREATE TABLE %s PARTITION OF %s FOR VALUES FROM (%L) TO (%L)', part, parent, from_d, to_d);
  EXECUTE format('INSERT INTO %s SELECT * FROM pg_temp.partition_move', part);
  DROP TABLE pg_temp.partition_move;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION platform.ensure_month_partition(parent regclass, col text, d date) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE
  from_d date := date_trunc('month', d)::date;
  to_d   date := (date_trunc('month', d) + interval '1 month')::date;
  part   text := format('%s_%s', parent::text, to_char(from_d, 'YYYY_MM'));
BEGIN
  IF parent::text NOT IN ('usage.usage_records', 'carbon.calculations') THEN
    RAISE EXCEPTION 'ensure_month_partition: % is not a managed partitioned table', parent;
  END IF;
  IF to_regclass(part) IS NULL THEN
    EXECUTE format('CREATE TABLE %s PARTITION OF %s FOR VALUES FROM (%L) TO (%L)', part, parent, from_d, to_d);
  END IF;
END $$;
-- +goose StatementEnd
