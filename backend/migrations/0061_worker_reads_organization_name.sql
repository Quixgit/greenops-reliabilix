-- +goose Up
-- A report is made by the worker and now says for whom it was made (the title block and the footer of the PDF). That needs the
-- organization's name and nothing else of the organization row: a column-level grant, and the row policy still limits the
-- worker to the organization of the job it is running.
GRANT SELECT (id, name) ON tenants.organizations TO greenops_worker;

-- +goose Down
REVOKE SELECT (id, name) ON tenants.organizations FROM greenops_worker;
