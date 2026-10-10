-- name: CountDestinations :one
SELECT count(*) FROM notifications.destinations WHERE tenant_id = sqlc.arg(tenant_id);

-- name: InsertDestination :one
INSERT INTO notifications.destinations (id, tenant_id, kind, name, target_sealed, target_hint, event_types, min_severity, enabled, created_by)
VALUES (sqlc.arg(id), sqlc.arg(tenant_id), sqlc.arg(kind), sqlc.arg(name), sqlc.arg(target_sealed), sqlc.arg(target_hint),
        sqlc.arg(event_types), sqlc.arg(min_severity), sqlc.arg(enabled), sqlc.arg(created_by))
RETURNING id, tenant_id, kind, name, target_sealed, target_hint, event_types, min_severity, enabled, created_by, created_at;

-- name: ListDestinations :many
SELECT id, tenant_id, kind, name, target_sealed, target_hint, event_types, min_severity, enabled, created_by, created_at
FROM notifications.destinations WHERE tenant_id = sqlc.arg(tenant_id) ORDER BY created_at, id;

-- name: GetDestination :one
SELECT id, tenant_id, kind, name, target_sealed, target_hint, event_types, min_severity, enabled, created_by, created_at
FROM notifications.destinations WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: SetDestinationEnabled :one
UPDATE notifications.destinations SET enabled = sqlc.arg(enabled)
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id)
RETURNING id, tenant_id, kind, name, target_sealed, target_hint, event_types, min_severity, enabled, created_by, created_at;

-- name: DeleteDestination :execrows
DELETE FROM notifications.destinations WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: InsertDelivery :one
-- ON CONFLICT: the same event (dedupe key) already reached this destination, so no row is returned.
INSERT INTO notifications.deliveries (tenant_id, destination_id, kind, destination_name, event_type, severity, payload, dedupe_key)
VALUES (sqlc.arg(tenant_id), sqlc.arg(destination_id), sqlc.arg(kind), sqlc.arg(destination_name), sqlc.arg(event_type),
        sqlc.arg(severity), sqlc.arg(payload), sqlc.narg(dedupe_key))
ON CONFLICT (destination_id, dedupe_key) WHERE dedupe_key IS NOT NULL AND destination_id IS NOT NULL DO NOTHING
RETURNING id;

-- name: ListDeliveries :many
SELECT id, destination_id, kind, destination_name, event_type, severity, status, attempts, next_attempt_at, last_error, created_at, sent_at
FROM notifications.deliveries WHERE tenant_id = sqlc.arg(tenant_id) ORDER BY created_at DESC, id LIMIT sqlc.arg(row_limit);

-- name: ClaimDelivery :one
-- Takes the lease on a due delivery. No row = already sent, not due yet, or another worker is sending it.
UPDATE notifications.deliveries SET attempts = attempts + 1, lease_until = now() + make_interval(secs => sqlc.arg(lease_secs)::float8)
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id) AND status = 'pending' AND next_attempt_at <= now()
  AND (lease_until IS NULL OR lease_until < now())
RETURNING id, destination_id, kind, destination_name, event_type, severity, payload, status, attempts, next_attempt_at, last_error, created_at, sent_at;

-- name: MarkDeliverySent :exec
UPDATE notifications.deliveries SET status = 'sent', sent_at = now(), lease_until = NULL, last_error = NULL
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: MarkDeliveryRetry :exec
UPDATE notifications.deliveries SET next_attempt_at = sqlc.arg(next_attempt_at), lease_until = NULL, last_error = sqlc.arg(last_error)
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id) AND status = 'pending';

-- name: MarkDeliveryFinal :exec
UPDATE notifications.deliveries SET status = sqlc.arg(status), lease_until = NULL, last_error = sqlc.arg(last_error)
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id) AND status = 'pending';
