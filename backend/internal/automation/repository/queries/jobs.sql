-- name: InsertJob :one
INSERT INTO automation.jobs (tenant_id, project_id, recommendation_id, kind, plan, risk_level, risk_factors, rollback_plan, created_by)
VALUES (sqlc.arg(tenant_id), sqlc.arg(project_id), sqlc.arg(recommendation_id), sqlc.arg(kind), sqlc.arg(plan), sqlc.arg(risk_level),
        sqlc.arg(risk_factors), sqlc.arg(rollback_plan), sqlc.arg(created_by))
RETURNING *;

-- name: ListJobs :many
SELECT * FROM automation.jobs WHERE tenant_id = sqlc.arg(tenant_id) ORDER BY created_at DESC LIMIT sqlc.arg(row_limit);

-- name: GetJob :one
SELECT * FROM automation.jobs WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: LockJob :one
-- FOR UPDATE serializes concurrent decisions on the same job.
SELECT * FROM automation.jobs WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id) FOR UPDATE;

-- name: ApproveJob :exec
UPDATE automation.jobs SET status = 'approved', approved_by = sqlc.arg(actor), approved_at = now()
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);

-- name: FinishJob :exec
UPDATE automation.jobs SET status = sqlc.arg(status), finished_by = sqlc.arg(actor), finished_at = now(), result_note = sqlc.narg(note)
WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(id);
