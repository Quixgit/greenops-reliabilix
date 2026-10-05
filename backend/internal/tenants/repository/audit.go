// Package repository reads tenant-level data.
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

type AuditEntry struct {
	ID        int64     `json:"id"`
	Actor     string    `json:"actor_user_id"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	RequestID string    `json:"request_id"`
	At        time.Time `json:"created_at"`
}

type Postgres struct{ Pool *pgxpool.Pool }

func (r Postgres) AuditLog(ctx context.Context, tenantID string, limit int) ([]AuditEntry, error) {
	out := []AuditEntry{}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, actor_user_id, action, target, COALESCE(request_id,''), created_at
			FROM audit.audit_logs WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2`, tenantID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a AuditEntry
			if err := rows.Scan(&a.ID, &a.Actor, &a.Action, &a.Target, &a.RequestID, &a.At); err != nil {
				return err
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, err
}
