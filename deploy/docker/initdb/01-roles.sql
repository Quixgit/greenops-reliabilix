-- Local development only. Production passwords come from the secret manager.
CREATE ROLE greenops_api    LOGIN PASSWORD 'dev-only';
CREATE ROLE greenops_worker LOGIN PASSWORD 'dev-only';
