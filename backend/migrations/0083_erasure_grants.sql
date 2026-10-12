-- +goose Up
-- Organization erasure (ADR-0027) runs as greenops_fanout and deletes from every table that has a tenant column. Tables added after
-- the function were never granted to it, so erasing an organization that had a report schedule, a commitment snapshot or a support
-- view failed with "permission denied" and was rolled back (the integration test found it). A test now lists every tenant table the role cannot empty.
GRANT SELECT, DELETE ON reports.schedules, reports.schedule_runs, cloudaccounts.commitment_snapshots TO greenops_fanout;
-- and the support views were granted DELETE only: the erasure reads the column it deletes by
GRANT SELECT ON tenants.support_views TO greenops_fanout;

-- +goose Down
REVOKE SELECT, DELETE ON reports.schedules, reports.schedule_runs, cloudaccounts.commitment_snapshots FROM greenops_fanout;
REVOKE SELECT ON tenants.support_views FROM greenops_fanout;
