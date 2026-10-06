-- +goose Up
-- A connection can read the customer's FOCUS data export from S3 instead of Cost Explorer. These are location
-- references only (bucket, prefix, export name, region); access is by the customer's read-only role.
ALTER TABLE cloudaccounts.connections
    ADD COLUMN export_bucket text,
    ADD COLUMN export_prefix text,
    ADD COLUMN export_name   text,
    ADD COLUMN export_region text,
    ADD CONSTRAINT export_all_or_nothing CHECK (
        (export_bucket IS NULL AND export_prefix IS NULL AND export_name IS NULL AND export_region IS NULL)
        OR (export_bucket IS NOT NULL AND export_prefix IS NOT NULL AND export_name IS NOT NULL AND export_region IS NOT NULL)),
    ADD CONSTRAINT export_bucket_format CHECK (export_bucket IS NULL OR export_bucket ~ '^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$'),
    ADD CONSTRAINT export_name_format CHECK (export_name IS NULL OR export_name ~ '^[A-Za-z0-9_.-]{1,128}$'),
    ADD CONSTRAINT export_region_format CHECK (export_region IS NULL OR export_region ~ '^[a-z]{2}(-[a-z]+)+-[0-9]$'),
    ADD CONSTRAINT export_prefix_format CHECK (export_prefix IS NULL OR export_prefix = ''
        OR (export_prefix ~ '^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*$' AND export_prefix !~ '(^|/)\.\.?(/|$)'));

-- +goose Down
ALTER TABLE cloudaccounts.connections
    DROP CONSTRAINT export_prefix_format, DROP CONSTRAINT export_region_format, DROP CONSTRAINT export_name_format,
    DROP CONSTRAINT export_bucket_format, DROP CONSTRAINT export_all_or_nothing,
    DROP COLUMN export_region, DROP COLUMN export_name, DROP COLUMN export_prefix, DROP COLUMN export_bucket;
