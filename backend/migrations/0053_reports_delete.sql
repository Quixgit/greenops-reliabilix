-- +goose Up
-- A report can be removed by the people who may create reports: the metadata row goes through the API role (row-level
-- security still limits it to the organization's own rows); the stored file is removed by the application.
GRANT DELETE ON reports.reports TO greenops_api;

-- +goose Down
REVOKE DELETE ON reports.reports FROM greenops_api;
