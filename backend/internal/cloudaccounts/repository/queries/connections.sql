-- name: ListConnections :many
SELECT id, tenant_id, project_id, provider, account_ref, credential_ref, external_id, sync_status, last_error, last_sync_at, synced_through, created_at
FROM cloudaccounts.connections WHERE tenant_id = sqlc.arg(tenant_id) ORDER BY created_at DESC LIMIT 500;

-- name: GetConnection :one
SELECT id, tenant_id, project_id, provider, account_ref, credential_ref, external_id, sync_status, last_error, last_sync_at, synced_through, created_at
FROM cloudaccounts.connections WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: CreateConnection :one
INSERT INTO cloudaccounts.connections (tenant_id, project_id, provider, account_ref, credential_ref, external_id)
VALUES (sqlc.arg(tenant_id), sqlc.arg(project_id), sqlc.arg(provider), sqlc.arg(account_ref), sqlc.arg(credential_ref), sqlc.arg(external_id))
RETURNING id, tenant_id, project_id, provider, account_ref, credential_ref, external_id, sync_status, last_error, last_sync_at, synced_through, created_at;

-- name: DeleteConnection :execrows
DELETE FROM cloudaccounts.connections WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: SetConnectionStatus :exec
UPDATE cloudaccounts.connections SET sync_status = sqlc.arg(status), last_error = sqlc.narg(last_error)
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: MarkConnectionSynced :exec
UPDATE cloudaccounts.connections
SET sync_status = 'healthy', last_error = NULL, last_sync_at = now(), synced_through = sqlc.arg(synced_through)
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: StartSyncRun :one
INSERT INTO cloudaccounts.sync_runs (tenant_id, connection_id) VALUES (sqlc.arg(tenant_id), sqlc.arg(connection_id)) RETURNING id;

-- name: FinishSyncRun :exec
UPDATE cloudaccounts.sync_runs SET finished_at = now(), status = sqlc.arg(status), records = sqlc.arg(records), error = sqlc.narg(error)
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: ListSyncRuns :many
SELECT id, connection_id, started_at, finished_at, status, records, error
FROM cloudaccounts.sync_runs WHERE tenant_id = sqlc.arg(tenant_id) AND connection_id = sqlc.arg(connection_id)
ORDER BY started_at DESC LIMIT 20;

