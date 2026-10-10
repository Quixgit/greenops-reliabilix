-- +goose Up
-- Cost data quality (ADR-0024). After every sync the platform compares what it stored with the totals the cloud
-- itself reports for the same days. The outcome is kept per run and, for the latest sync, on the connection, so
-- people can see whether the numbers can be trusted instead of discovering a hole in a report months later.
ALTER TABLE cloudaccounts.sync_runs   ADD COLUMN quality jsonb;
ALTER TABLE cloudaccounts.connections ADD COLUMN last_quality jsonb;

-- +goose Down
ALTER TABLE cloudaccounts.connections DROP COLUMN last_quality;
ALTER TABLE cloudaccounts.sync_runs   DROP COLUMN quality;
