-- +goose Up
-- The weekly job that refreshes the relative region price index (EC2 on-demand basket) writes it as the worker.
GRANT INSERT, UPDATE ON finops.region_price_index TO greenops_worker;

-- +goose Down
REVOKE INSERT, UPDATE ON finops.region_price_index FROM greenops_worker;
