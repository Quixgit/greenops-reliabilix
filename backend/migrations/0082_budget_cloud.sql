-- +goose Up
-- A budget can belong to one cloud: it is then measured on that cloud's own spend in its own currency (a budget of USD 1,500 for AWS
-- and one of CAD 2,000 for Azure), instead of a sum over clouds billed in different currencies. NULL keeps the old meaning, all clouds.
ALTER TABLE finops.budgets ADD COLUMN cloud text CHECK (cloud IS NULL OR cloud ~ '^[a-z0-9][a-z0-9_-]{0,31}$');

-- +goose Down
ALTER TABLE finops.budgets DROP COLUMN cloud;
