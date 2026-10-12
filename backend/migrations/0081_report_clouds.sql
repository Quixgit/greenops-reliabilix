-- +goose Up
-- A report (and a schedule that makes one) can cover some of the project's clouds instead of all of them. An empty list is "every cloud".
ALTER TABLE reports.reports   ADD COLUMN clouds text[] NOT NULL DEFAULT '{}';
ALTER TABLE reports.schedules ADD COLUMN clouds text[] NOT NULL DEFAULT '{}';

-- The cloud scope (migration 0080) takes a list: app.cloud is "aws" or "aws,azure".
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['usage.usage_records', 'usage.tagged_cost', 'carbon.calculations', 'recommendations.recommendations', 'cloudaccounts.connections'] LOOP
    EXECUTE format('DROP POLICY IF EXISTS cloud_scope ON %s', t);
    EXECUTE format($f$CREATE POLICY cloud_scope ON %s AS RESTRICTIVE FOR SELECT
      USING ((SELECT NULLIF(current_setting('app.cloud', true), '')) IS NULL
             OR provider = ANY (string_to_array((SELECT current_setting('app.cloud', true)), ',')))$f$, t);
  END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE reports.schedules DROP COLUMN clouds;
ALTER TABLE reports.reports   DROP COLUMN clouds;
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['usage.usage_records', 'usage.tagged_cost', 'carbon.calculations', 'recommendations.recommendations', 'cloudaccounts.connections'] LOOP
    EXECUTE format('DROP POLICY IF EXISTS cloud_scope ON %s', t);
    EXECUTE format($f$CREATE POLICY cloud_scope ON %s AS RESTRICTIVE FOR SELECT
      USING ((SELECT NULLIF(current_setting('app.cloud', true), '')) IS NULL OR provider = (SELECT current_setting('app.cloud', true)))$f$, t);
  END LOOP;
END $$;
-- +goose StatementEnd
