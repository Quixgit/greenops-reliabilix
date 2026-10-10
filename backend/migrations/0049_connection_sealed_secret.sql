-- +goose Up
-- A connection can carry one secret that the customer supplied and the platform must be able to use again: the client
-- secret of an Azure service principal that may read the subscription's costs. It is stored sealed (AES-256-GCM,
-- bound to the organization and the connection's token) and is never returned by the API, exported or logged. AWS and
-- GCP connections keep their promise of storing nothing: the column stays NULL for them.
ALTER TABLE cloudaccounts.connections ADD COLUMN sealed_secret text;

-- +goose Down
ALTER TABLE cloudaccounts.connections DROP COLUMN sealed_secret;
