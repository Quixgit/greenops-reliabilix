-- name: Onboard :one
SELECT tenants.onboard(sqlc.arg(sub)::text, sqlc.arg(email)::text, sqlc.arg(org_name)::text)::uuid AS tenant_id;

-- name: AcceptInvitation :one
SELECT tenants.accept_invitation(sqlc.arg(token_hash)::text, sqlc.arg(sub)::text, sqlc.arg(email)::text)::uuid AS tenant_id;

-- name: GetOrganization :one
SELECT id, name, plan, created_at FROM tenants.organizations WHERE id = sqlc.arg(tenant_id);

-- name: ListMembers :many
SELECT m.id, m.user_id, u.auth_sub, u.email, m.role, m.created_at
FROM tenants.memberships m JOIN tenants.users u ON u.id = m.user_id
WHERE m.organization_id = sqlc.arg(tenant_id) ORDER BY m.created_at;

-- name: GetMember :one
SELECT m.id, m.user_id, u.auth_sub, u.email, m.role, m.created_at
FROM tenants.memberships m JOIN tenants.users u ON u.id = m.user_id
WHERE m.organization_id = sqlc.arg(tenant_id) AND m.id = sqlc.arg(id) FOR UPDATE OF m;

-- name: CountOwners :one
SELECT count(*) FROM tenants.memberships WHERE organization_id = sqlc.arg(tenant_id) AND role = 'owner';

-- name: UpdateMemberRole :exec
UPDATE tenants.memberships SET role = sqlc.arg(role) WHERE organization_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: DeleteMember :exec
DELETE FROM tenants.memberships WHERE organization_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: CreateInvitation :one
INSERT INTO tenants.invitations (organization_id, email, role, token_hash, created_by, expires_at)
VALUES (sqlc.arg(tenant_id), sqlc.arg(email), sqlc.arg(role), sqlc.arg(token_hash), sqlc.arg(created_by), sqlc.arg(expires_at))
RETURNING id, email, role, expires_at, created_at;

-- name: ListInvitations :many
SELECT id, email, role, created_by, expires_at, accepted_at, created_at
FROM tenants.invitations WHERE organization_id = sqlc.arg(tenant_id) ORDER BY created_at DESC LIMIT 100;

-- name: CreateAPIKey :one
INSERT INTO tenants.api_keys (organization_id, name, prefix, key_hash, role, created_by)
VALUES (sqlc.arg(tenant_id), sqlc.arg(name), sqlc.arg(prefix), sqlc.arg(key_hash), sqlc.arg(role), sqlc.arg(created_by))
RETURNING id, name, prefix, role, created_at;

-- name: ListAPIKeys :many
SELECT id, name, prefix, role, created_by, created_at, last_used_at, revoked_at
FROM tenants.api_keys WHERE organization_id = sqlc.arg(tenant_id) ORDER BY created_at DESC LIMIT 100;

-- name: RevokeAPIKey :execrows
UPDATE tenants.api_keys SET revoked_at = now() WHERE organization_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id) AND revoked_at IS NULL;

-- name: ListAuditLog :many
SELECT id, actor_user_id, action, target, COALESCE(request_id, '')::text AS request_id, metadata, created_at
FROM audit.audit_logs WHERE tenant_id = sqlc.arg(tenant_id) AND (sqlc.arg(action)::text = '' OR action = sqlc.arg(action)::text)
ORDER BY created_at DESC, id DESC LIMIT sqlc.arg(row_limit);
